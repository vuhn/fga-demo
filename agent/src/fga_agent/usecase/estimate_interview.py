from __future__ import annotations

from typing import Protocol

from ..domain.model.call_context import CallContext
from ..domain.model.estimate_facts import EstimateFacts
from ..domain.model.estimate_session import FactChange, Question, SessionState
from ..domain.model.extracted import Extracted
from ..domain.model.plan_summary import PlanSummary


class EstimateInterview(Protocol):
    """The interview of a cost estimate. Which questions are still open is decided by code, never by the model, so the
    interview is stable and testable. `summary` is the Engine's summary of the plan asked with the current facts
    (`facts(state)`): what is pending and whether structure is in scope come from the Engine."""

    def facts(self, state: SessionState) -> EstimateFacts:
        """The part of the session takeoff and pricing read."""

    def missing(self, ctx: CallContext, state: SessionState, summary: PlanSummary) -> list[Question]:
        """The open questions in the order they are asked (the first is the next one). Empty when the estimate can be made."""

    def record(self, ctx: CallContext, state: SessionState, summary: PlanSummary, extracted: Extracted,
               message: str) -> tuple[SessionState, list[FactChange]]:
        """The session after what the message states, and every change it made. The given state is not modified. What the model
        set without the words to back it (typical percentages, giving up) is ignored."""

    def fingerprint(self, state: SessionState) -> str:
        """Identifies the inputs an estimate was computed from, so an unchanged session is not priced again."""
