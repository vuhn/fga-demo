from __future__ import annotations

from typing import Protocol

from ..domain.model.call_context import CallContext
from ..domain.model.estimate_session import Message, SessionState
from ..domain.model.extracted import Extracted
from ..domain.model.plan_summary import PlanSummary


class EstimateReader(Protocol):
    """Reads the person's message, in any language, into what it states about the estimate. The model reads four narrow
    things at once (the facts, the settings, the condition of rooms, what already exists) as claims with the words they rest
    on; a claim whose words are not in the message is dropped. Code's certain readings fill the gaps and veto what the model
    could not have known from the message."""

    def read(self, ctx: CallContext, summary: PlanSummary, state: SessionState, message: str, history: list[Message]) -> Extracted:
        """LlmUnavailable when the model cannot be reached (never hidden: the turn must not continue on nothing);
        EngineUnavailable when the Engine's catalog cannot be loaded."""
