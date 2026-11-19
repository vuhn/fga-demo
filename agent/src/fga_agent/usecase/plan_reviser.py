from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress


class PlanReviser(Protocol):
    """Changes a plan kept as drawn (imported from CAD) by a chat message: the model names plan operations, the Engine
    applies them and runs the checks. Walls and rooms cannot be moved or resized."""

    def revise(self, ctx: CallContext, model: bytes, message: str, history: list[str],
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        """InvalidEdit for an empty message or operations the Engine refuses three times; UnsupportedEdit when the request
        asks for geometry changes (not retried); LlmUnavailable; EngineUnavailable (both before the first step when the
        request itself is refused or the Engine's catalog cannot be loaded)."""
