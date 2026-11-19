from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from ..domain.model.brief import BriefDraft
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress


class BriefDrafter(Protocol):
    """Turns a house description into a brief (the model drafts, the Engine normalizes)."""

    def draft(self, ctx: CallContext, prompt: str, enhance: bool, on_progress: Callable[[Progress], None]) -> BriefDraft:
        """InvalidEdit for an empty description or a draft the Engine refuses three times; LlmUnavailable; EngineUnavailable."""
