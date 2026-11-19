"""A live step of a long run (asking the model, a retry, rebuilding, checking) or a piece of the model's reasoning.
Shown while the run lasts, never stored. The caller passes a `Callable[[Progress], None]` that may raise Cancelled to stop the run."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Progress:
    kind: str  # step | thinking
    stage: str  # ask_model, retry, ops, layout, checks, review, thinking
    message: str
