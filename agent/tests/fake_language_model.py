"""A scripted language model for service tests: answer_json() pops the next answer; answer_text() yields the next text."""

from __future__ import annotations

from collections.abc import Iterator

from fga_agent.domain.errors import LlmUnavailable
from fga_agent.domain.gateway import ProgressReporter
from fga_agent.domain.progress import NO_PROGRESS


class FakeLanguageModel:
    def __init__(self, answers: list[dict | Exception] | None = None, texts: list[str] | None = None) -> None:
        self.answers = list(answers or [])
        self.texts = list(texts or [])
        self.calls: list[dict] = []

    def answer_json(self, system: str, user: str, schema: dict, max_tokens: int | None = None,
                    progress: ProgressReporter = NO_PROGRESS, think: bool = True) -> dict:
        self.calls.append({"system": system, "user": user, "schema": schema})
        if not self.answers:
            raise LlmUnavailable("FakeLanguageModel has no answer left")
        a = self.answers.pop(0)
        if isinstance(a, Exception):
            raise a
        return a

    def answer_text(self, system: str, user: str) -> Iterator[str]:
        self.calls.append({"system": system, "user": user})
        text = self.texts.pop(0) if self.texts else ""
        for i in range(0, len(text), 5):
            yield text[i:i + 5]
