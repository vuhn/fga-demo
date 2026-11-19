from __future__ import annotations

from typing import Protocol

from ..domain.draft import DraftBrief
from ..domain.schemas import Brief


class DraftNormalizer(Protocol):
    def normalize_draft(self, draft: DraftBrief) -> tuple[Brief, list[str]]:
        """A drafted brief -> a valid Brief (ids, references, minimum areas, halls dropped) and notes.
        Raises EditError when no rooms remain."""
