"""Ollama client behind gateway.LLM. Copy of the Engine-era llm.py: the model answers plain JSON for a schema and the
Engine's tools check it, so the client only retries unparseable JSON."""

from __future__ import annotations

import json
import os
import time
from collections.abc import Iterator

import httpx

from ...domain import progress
from ...domain.errors import LLMError


class _ThinkingBudget(Exception):
    """The reasoning stream ran past its budget; the caller redoes the call without thinking."""


class OllamaClient:
    """Ollama /api/chat with `format` = JSON schema, temperature 0, one retry on invalid output."""

    def __init__(
        self,
        base_url: str | None = None,
        text_model: str | None = None,
        timeout: float = 180.0,
    ) -> None:
        self.base_url = (base_url or os.getenv("OLLAMA_URL", "http://localhost:11434")).rstrip("/")
        self.text_model = text_model or os.getenv("BLUEPRINT_TEXT_MODEL", "qwen3:latest")
        self.http = httpx.Client(timeout=timeout)
        # show the model's reasoning on the streaming endpoints; BLUEPRINT_STREAM_THINKING=0 turns it off
        self.stream_thinking = os.getenv("BLUEPRINT_STREAM_THINKING", "1") != "0"
        # A thinking stream that exceeds either budget is cut and the call is redone without thinking. Greedy
        # decoding (temperature 0) lets qwen3 fall into a repetition loop in its reasoning, and the read timeout
        # never fires while chunks keep arriving, so without a budget a revise can think forever.
        self.thinking_max_chars = int(os.getenv("BLUEPRINT_THINKING_MAX_CHARS", "6000"))
        self.thinking_max_seconds = float(os.getenv("BLUEPRINT_THINKING_MAX_SECONDS", "90"))

    def _chat(self, model: str, messages: list[dict], schema: dict, max_tokens: int | None = None) -> str:
        if self.stream_thinking and progress.thinking_wanted():
            try:
                return self._chat_streaming(model, messages, schema, max_tokens)
            except _ThinkingBudget as e:
                progress.emit("retry", f"the model thought too long ({e}); answering without thinking")
        return self._chat_plain(model, messages, schema, max_tokens)

    def _chat_plain(self, model: str, messages: list[dict], schema: dict, max_tokens: int | None = None) -> str:
        try:
            resp = self.http.post(
                f"{self.base_url}/api/chat",
                json={
                    "model": model,
                    "messages": messages,
                    "format": schema,
                    "stream": False,
                    "think": False,
                    "options": {"temperature": 0, "num_ctx": 8192, **({"num_predict": max_tokens} if max_tokens else {})},
                },
            )
        except httpx.HTTPError as e:
            raise LLMError(f"cannot reach Ollama at {self.base_url}: {e}") from e
        if resp.status_code != 200:
            raise LLMError(f"Ollama {resp.status_code}: {resp.text[:300]}")
        return resp.json()["message"]["content"]

    def _chat_streaming(self, model: str, messages: list[dict], schema: dict, max_tokens: int | None = None) -> str:
        """Same call with thinking on and the answer streamed: the model's reasoning goes to the
        progress sink (shown live, never stored); the JSON answer is returned whole. Chunks are
        coalesced to about one event per 150 ms."""
        # qwen3 thinks for minutes unless told otherwise; ask for a short plan (the answer is still validated)
        messages = [{**m, "content": m["content"] + "\n\nThink briefly: a few short sentences, then answer."}
                    if m["role"] == "system" else m for m in messages]
        body = {"model": model, "messages": messages, "format": schema, "stream": True, "think": True,
                "options": {"temperature": 0, "num_ctx": 8192, **({"num_predict": max_tokens} if max_tokens else {})}}
        content: list[str] = []
        pending: list[str] = []
        started = last = time.monotonic()
        thought = 0
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
                        thought += len(msg["thinking"])
                        if thought > self.thinking_max_chars or time.monotonic() - started > self.thinking_max_seconds:
                            progress.emit_thinking("".join(pending))
                            raise _ThinkingBudget(f"{thought} characters, {time.monotonic() - started:.0f}s")
                        if time.monotonic() - last >= 0.15:
                            progress.emit_thinking("".join(pending))
                            pending, last = [], time.monotonic()
                    content.append(msg.get("content", ""))
        except httpx.HTTPError as e:
            raise LLMError(f"cannot reach Ollama at {self.base_url}: {e}") from e
        progress.emit_thinking("".join(pending))
        return "".join(content)

    def structured(self, system, user, schema, max_tokens=None):
        model = self.text_model
        messages = [{"role": "system", "content": system}, {"role": "user", "content": user}]
        last_err = ""
        for _ in range(2):
            raw = self._chat(model, messages, schema, max_tokens)
            try:
                out = json.loads(raw)
                if not isinstance(out, dict):
                    raise ValueError("expected a JSON object")
                return out
            except ValueError as e:  # includes json.JSONDecodeError
                last_err = str(e)[:1200]
                messages = messages + [
                    {"role": "assistant", "content": raw},
                    {"role": "user", "content": f"That JSON is invalid:\n{last_err}\nReturn corrected JSON only."},
                ]
        raise LLMError(f"model returned invalid JSON twice: {last_err}")

    def stream_text(self, system, user):
        """Plain-text reply, yielded as it is generated (no `format`, thinking off). Raises
        progress.Cancelled through the caller's sink, not here: the caller checks between chunks."""
        body = {"model": self.text_model, "stream": True, "think": False,
                "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}],
                "options": {"temperature": 0.3, "num_ctx": 8192}}
        try:
            with self.http.stream("POST", f"{self.base_url}/api/chat", json=body) as resp:
                if resp.status_code != 200:
                    resp.read()
                    raise LLMError(f"Ollama {resp.status_code}: {resp.text[:300]}")
                for line in resp.iter_lines():
                    if not line:
                        continue
                    piece = json.loads(line).get("message", {}).get("content", "")
                    if piece:
                        yield piece
        except httpx.HTTPError as e:
            raise LLMError(f"cannot reach Ollama at {self.base_url}: {e}") from e
