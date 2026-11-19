"""Ollama client behind gateway.LLM. Copy of the Engine-era llm.py: the model answers plain JSON for a schema and the
Engine's tools check it, so the client only retries unparseable JSON."""

from __future__ import annotations

import json
import os
import time
from collections.abc import Callable, Iterator

import httpx

from ...domain.errors import LlmUnavailable
from ...domain.model.progress import Progress
from ..entity.ollama_chat import ChatChunk, ChatMessage, ChatRequest


class _ThinkingBudget(Exception):
    """The reasoning stream ran past its budget; the caller redoes the call without thinking."""


class OllamaLanguageModel:
    """Ollama /api/chat (implements gateway.LanguageModel) with `format` = JSON schema, temperature 0, one retry on invalid output."""

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

    def _chat(self, model: str, messages: list[ChatMessage], schema: dict, max_tokens: int | None,
              on_progress: Callable[[Progress], None] | None, think: bool) -> str:
        if self.stream_thinking and think and on_progress is not None:
            try:
                return self._chat_streaming(model, messages, schema, max_tokens, on_progress)
            except _ThinkingBudget as e:
                on_progress(Progress("step", "retry", f"the model thought too long ({e}); answering without thinking"))
        return self._chat_plain(model, messages, schema, max_tokens)

    def _chat_plain(self, model: str, messages: list[ChatMessage], schema: dict, max_tokens: int | None = None) -> str:
        try:
            req = ChatRequest(model, messages, stream=False, think=False, schema=schema, max_tokens=max_tokens)
            resp = self.http.post(f"{self.base_url}/api/chat", json=req.body())
        except httpx.HTTPError as e:
            raise LlmUnavailable(f"cannot reach Ollama at {self.base_url}: {e}") from e
        if resp.status_code != 200:
            raise LlmUnavailable(f"Ollama {resp.status_code}: {resp.text[:300]}")
        return ChatChunk.from_json(resp.json()).content

    def _chat_streaming(self, model: str, messages: list[ChatMessage], schema: dict, max_tokens: int | None,
                        on_progress: Callable[[Progress], None]) -> str:
        """Same call with thinking on and the answer streamed: the model's reasoning goes to the
        progress callback (shown live, never stored); the JSON answer is returned whole. Chunks are
        coalesced to about one event per 150 ms."""
        # qwen3 thinks for minutes unless told otherwise; ask for a short plan (the answer is still validated)
        messages = [ChatMessage(m.role, m.content + "\n\nThink briefly: a few short sentences, then answer.")
                    if m.role == "system" else m for m in messages]
        body = ChatRequest(model, messages, stream=True, think=True, schema=schema, max_tokens=max_tokens).body()
        content: list[str] = []
        pending: list[str] = []
        started = last = time.monotonic()
        thought = 0
        try:
            with self.http.stream("POST", f"{self.base_url}/api/chat", json=body) as resp:
                if resp.status_code != 200:
                    resp.read()
                    raise LlmUnavailable(f"Ollama {resp.status_code}: {resp.text[:300]}")
                for line in resp.iter_lines():
                    if not line:
                        continue
                    chunk = ChatChunk.from_line(line)
                    if chunk.thinking:
                        pending.append(chunk.thinking)
                        thought += len(chunk.thinking)
                        if thought > self.thinking_max_chars or time.monotonic() - started > self.thinking_max_seconds:
                            on_progress(Progress("thinking", "thinking", "".join(pending)))
                            raise _ThinkingBudget(f"{thought} characters, {time.monotonic() - started:.0f}s")
                        if time.monotonic() - last >= 0.15:
                            on_progress(Progress("thinking", "thinking", "".join(pending)))
                            pending, last = [], time.monotonic()
                    content.append(chunk.content)
        except httpx.HTTPError as e:
            raise LlmUnavailable(f"cannot reach Ollama at {self.base_url}: {e}") from e
        if pending:
            on_progress(Progress("thinking", "thinking", "".join(pending)))
        return "".join(content)

    def answer_json(self, system, user, schema, max_tokens=None, on_progress=None, think=True):
        model = self.text_model
        messages = [ChatMessage("system", system), ChatMessage("user", user)]
        last_err = ""
        for _ in range(2):
            raw = self._chat(model, messages, schema, max_tokens, on_progress, think)
            try:
                out = json.loads(raw)
                if not isinstance(out, dict):
                    raise ValueError("expected a JSON object")
                return out
            except ValueError as e:  # includes json.JSONDecodeError
                last_err = str(e)[:1200]
                messages = messages + [
                    ChatMessage("assistant", raw),
                    ChatMessage("user", f"That JSON is invalid:\n{last_err}\nReturn corrected JSON only."),
                ]
        raise LlmUnavailable(f"model returned invalid JSON twice: {last_err}")

    def answer_text(self, system, user):
        """Plain-text reply, yielded as it is generated (no `format`, thinking off). The caller
        stops early by closing the generator."""
        body = ChatRequest(self.text_model, [ChatMessage("system", system), ChatMessage("user", user)], stream=True,
                           think=False, temperature=0.3).body()
        try:
            with self.http.stream("POST", f"{self.base_url}/api/chat", json=body) as resp:
                if resp.status_code != 200:
                    resp.read()
                    raise LlmUnavailable(f"Ollama {resp.status_code}: {resp.text[:300]}")
                for line in resp.iter_lines():
                    if not line:
                        continue
                    piece = ChatChunk.from_line(line).content
                    if piece:
                        yield piece
        except httpx.HTTPError as e:
            raise LlmUnavailable(f"cannot reach Ollama at {self.base_url}: {e}") from e
