from __future__ import annotations

from collections.abc import Callable
from typing import Any, Protocol

from ..domain.model.call_context import CallContext
from ..domain.model.estimate_turn import EstimateTurnRequest, EstimateTurnResult


class EstimateTurner(Protocol):
    """One turn of the estimate conversation: read the message, ask the next open question or price the plan, and reply.
    The Agent keeps nothing between turns: the session comes in with the request and goes out with the result."""

    def turn(self, ctx: CallContext, request: EstimateTurnRequest, on_event: Callable[[Any], None]) -> EstimateTurnResult:
        """`on_event` hears, in order: `Progress`, `FactChange` for each change the message made, `ReplyToken` as the reply is
        written, `Question` (the turn asks one), `RateResult` for each researched rate and the `Estimate` (the turn priced
        the plan). It may raise Cancelled when the caller left. LlmUnavailable and EngineUnavailable are never hidden: the
        turn must not go on without the model or the Engine; TurnTimedOut when the turn runs past its ceiling."""
