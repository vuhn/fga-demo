"""The reporter used when nobody listens (unary calls, tests)."""

from __future__ import annotations


class NoProgress:
    def step(self, stage: str, message: str) -> None: ...

    def thinking(self, text: str) -> None: ...

    def event(self, name: str, data: dict) -> None: ...


NO_PROGRESS = NoProgress()
