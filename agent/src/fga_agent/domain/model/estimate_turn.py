"""One turn of the estimate conversation: what comes in and what the person is left with."""

from __future__ import annotations

from dataclasses import dataclass, field

from .estimate import Estimate
from .estimate_session import Message, Question, SessionState


@dataclass(frozen=True)
class EstimateTurnRequest:
    model: bytes  # the blueprint version's plan, as the Engine writes it
    state: SessionState
    message: str  # may be empty: the turn then only continues the interview
    messages: list[Message] = field(default_factory=list)  # earlier chat messages, oldest first
    rate_overrides: dict[str, float] = field(default_factory=dict)  # rate code -> VND per unit the person entered


@dataclass(frozen=True)
class EstimateTurnResult:
    state: SessionState
    reply: str
    question: Question | None = None
    estimate: Estimate | None = None  # only when this turn priced the plan
