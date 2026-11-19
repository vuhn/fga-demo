from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress


class BriefReviewer(Protocol):
    """Fixes the failed checks of a plan made from a brief (Text2BIM's Reviewer): each round the model proposes edit
    operations for the issues, and a round is kept only when the issue score drops and no new kind of error appears. The
    person's intent is fixed: no removing or renaming rooms, no lot or entrance change, total area within 15%."""

    def review(self, ctx: CallContext, brief: bytes, previous_model: bytes, history: list[str], max_rounds: int,
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        """At most `max_rounds` (default and ceiling 3) rounds. Issues the model cannot fix are left in the outcome for the
        person; the plan returned is never worse than the one given. LlmUnavailable; EngineUnavailable."""
