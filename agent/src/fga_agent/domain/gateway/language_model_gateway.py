from __future__ import annotations

from collections.abc import Iterator
from typing import Protocol

from ..progress import NO_PROGRESS
from .progress_reporter_gateway import ProgressReporter


class LanguageModel(Protocol):
    """The language model. Its output is checked by the Engine (NormalizeDraft, ApplyBriefOps), so it returns plain
    JSON for the schema it is given; unparseable JSON is retried once with the parse error."""

    def answer_json(self, system: str, user: str, schema: dict, max_tokens: int | None = None,
                    progress: ProgressReporter = NO_PROGRESS, think: bool = True) -> dict:
        """A JSON object for the JSON Schema. The model's reasoning goes to `progress` when `think` is true and the
        caller listens. LlmUnavailable when the model cannot be reached or answers invalid JSON twice."""

    def answer_text(self, system: str, user: str) -> Iterator[str]:
        """A plain-text reply, yielded as it is generated."""
