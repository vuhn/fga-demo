from __future__ import annotations

from typing import Protocol


class ProgressReporter(Protocol):
    """Where a long run says what it is doing. Progress is shown, never stored. Any method may raise Cancelled
    when the client has left, which stops the run at that step."""

    def step(self, stage: str, message: str) -> None:
        """A stage of the run: ask_model, retry, ops, layout, checks, review."""

    def thinking(self, text: str) -> None:
        """A piece of the model's reasoning."""

    def event(self, name: str, data: dict) -> None:
        """A chat event that is not progress (token, fact, question, rate, estimate)."""
