"""Thin LLM interface. Swap OllamaClient for a hosted model without touching the pipeline."""

from __future__ import annotations

import base64
import json
import os
import time
from typing import Protocol, TypeVar

import httpx
from pydantic import BaseModel, ValidationError

from . import progress

T = TypeVar("T", bound=BaseModel)


class LLMError(RuntimeError):
    pass


class LLMClient(Protocol):
    def structured(
        self, system: str, user: str, schema: type[T], *, images: list[bytes] | None = None, vision: bool = False
    ) -> T: ...


class OllamaClient:
    """Ollama /api/chat with `format` = JSON schema, temperature 0, one retry on invalid output."""

    def __init__(
        self,
        base_url: str | None = None,
        text_model: str | None = None,
        vision_model: str | None = None,
        timeout: float = 180.0,
    ) -> None:
        self.base_url = (base_url or os.getenv("OLLAMA_URL", "http://localhost:11434")).rstrip("/")
        self.text_model = text_model or os.getenv("BLUEPRINT_TEXT_MODEL", "qwen3:latest")
        self.vision_model = vision_model or os.getenv("BLUEPRINT_VISION_MODEL", "qwen2.5vl:7b")
        self.http = httpx.Client(timeout=timeout)
        # show the model's reasoning on the streaming endpoints; BLUEPRINT_STREAM_THINKING=0 turns it off
        self.stream_thinking = os.getenv("BLUEPRINT_STREAM_THINKING", "1") != "0"

    def _chat(self, model: str, messages: list[dict], schema: dict) -> str:
        if self.stream_thinking and progress.active():
            return self._chat_streaming(model, messages, schema)
        try:
            resp = self.http.post(
                f"{self.base_url}/api/chat",
                json={
                    "model": model,
                    "messages": messages,
                    "format": schema,
                    "stream": False,
                    "think": False,
                    "options": {"temperature": 0, "num_ctx": 8192},
                },
            )
        except httpx.HTTPError as e:
            raise LLMError(f"cannot reach Ollama at {self.base_url}: {e}") from e
        if resp.status_code != 200:
            raise LLMError(f"Ollama {resp.status_code}: {resp.text[:300]}")
        return resp.json()["message"]["content"]

    def _chat_streaming(self, model: str, messages: list[dict], schema: dict) -> str:
        """Same call with thinking on and the answer streamed: the model's reasoning goes to the
        progress sink (shown live, never stored); the JSON answer is returned whole. Chunks are
        coalesced to about one event per 150 ms."""
        # qwen3 thinks for minutes unless told otherwise; ask for a short plan (the answer is still validated)
        messages = [{**m, "content": m["content"] + "\n\nThink briefly: a few short sentences, then answer."}
                    if m["role"] == "system" else m for m in messages]
        body = {"model": model, "messages": messages, "format": schema, "stream": True, "think": True,
                "options": {"temperature": 0, "num_ctx": 8192}}
        content: list[str] = []
        pending: list[str] = []
        last = time.monotonic()
        try:
            with self.http.stream("POST", f"{self.base_url}/api/chat", json=body) as resp:
                if resp.status_code != 200:
                    resp.read()
                    raise LLMError(f"Ollama {resp.status_code}: {resp.text[:300]}")
                for line in resp.iter_lines():
                    if not line:
                        continue
                    msg = json.loads(line).get("message", {})
                    if msg.get("thinking"):
                        pending.append(msg["thinking"])
                        if time.monotonic() - last >= 0.15:
                            progress.emit_thinking("".join(pending))
                            pending, last = [], time.monotonic()
                    content.append(msg.get("content", ""))
        except httpx.HTTPError as e:
            raise LLMError(f"cannot reach Ollama at {self.base_url}: {e}") from e
        progress.emit_thinking("".join(pending))
        return "".join(content)

    def structured(self, system, user, schema, *, images=None, vision=False):
        model = self.vision_model if vision else self.text_model
        msg: dict = {"role": "user", "content": user}
        if images:
            msg["images"] = [base64.b64encode(i).decode() for i in images]
        messages = [{"role": "system", "content": system}, msg]
        json_schema = schema.model_json_schema()
        last_err = ""
        for _ in range(2):
            raw = self._chat(model, messages, json_schema)
            try:
                return schema.model_validate(json.loads(raw))
            except (json.JSONDecodeError, ValidationError) as e:
                last_err = str(e)[:1200]
                messages = messages + [
                    {"role": "assistant", "content": raw},
                    {"role": "user", "content": f"That JSON is invalid:\n{last_err}\nReturn corrected JSON only."},
                ]
        raise LLMError(f"model returned invalid JSON twice: {last_err}")
