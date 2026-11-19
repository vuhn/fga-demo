"""Ollama's `/api/chat` wire shapes. They exist only inside the Ollama adapter; the domain never sees them."""

from __future__ import annotations

import json
from dataclasses import dataclass, field


@dataclass(frozen=True)
class ChatMessage:
    role: str  # system | user | assistant
    content: str

    def as_json(self) -> dict:
        return {"role": self.role, "content": self.content}


@dataclass(frozen=True)
class ChatRequest:
    model: str
    messages: list[ChatMessage]
    stream: bool
    think: bool
    temperature: float = 0
    schema: dict | None = None  # the `format` field: constrains the answer to this JSON Schema
    max_tokens: int | None = None
    options: dict = field(default_factory=dict)

    def body(self) -> dict:
        options = {"temperature": self.temperature, "num_ctx": 8192, **self.options}
        if self.max_tokens:
            options["num_predict"] = self.max_tokens
        body: dict = {"model": self.model, "messages": [m.as_json() for m in self.messages], "stream": self.stream,
                      "think": self.think, "options": options}
        if self.schema is not None:
            body["format"] = self.schema
        return body


@dataclass(frozen=True)
class ChatChunk:
    """One line of a streamed answer (or the whole non-streamed answer): visible text and the model's reasoning."""

    content: str = ""
    thinking: str = ""

    @classmethod
    def from_json(cls, data: dict) -> "ChatChunk":
        msg = data.get("message") or {}
        return cls(content=msg.get("content") or "", thinking=msg.get("thinking") or "")

    @classmethod
    def from_line(cls, line: str | bytes) -> "ChatChunk":
        return cls.from_json(json.loads(line))
