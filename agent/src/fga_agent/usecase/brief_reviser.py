from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress


class BriefReviser(Protocol):
    """Changes a brief by a chat message: the model names edit operations, the Engine applies and builds them."""

    def revise(self, ctx: CallContext, brief: bytes, previous_model: bytes, message: str, history: list[str],
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        """InvalidEdit for an empty message or operations the Engine refuses three times; UnsupportedEdit when the request
        is not supported (not retried); LlmUnavailable; EngineUnavailable. Both are raised before the first progress
        step when the request itself is refused or the Engine's catalog cannot be loaded."""
