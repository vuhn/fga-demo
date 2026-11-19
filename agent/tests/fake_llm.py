"""A scripted LLM for service tests: structured() pops the next answer; stream_text() yields the next text in pieces."""

from __future__ import annotations

from collections.abc import Iterator

from fga_agent.domain.errors import LLMError


class FakeLLM:
    def __init__(self, answers: list[dict | Exception] | None = None, texts: list[str] | None = None) -> None:
        self.answers = list(answers or [])
        self.texts = list(texts or [])
        self.calls: list[dict] = []

    def structured(self, system: str, user: str, schema: dict, max_tokens: int | None = None) -> dict:
        self.calls.append({"system": system, "user": user, "schema": schema})
        if not self.answers:
            raise LLMError("FakeLLM has no answer left")
        a = self.answers.pop(0)
        if isinstance(a, Exception):
            raise a
        return a

    def stream_text(self, system: str, user: str) -> Iterator[str]:
        self.calls.append({"system": system, "user": user})
        text = self.texts.pop(0) if self.texts else ""
        for i in range(0, len(text), 5):
            yield text[i:i + 5]
