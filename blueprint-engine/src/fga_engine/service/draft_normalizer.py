from __future__ import annotations

from ..domain.draft import DraftBrief, normalize_draft
from ..domain.schemas import Brief


class DraftNormalizerService:
    def normalize_draft(self, draft: DraftBrief) -> tuple[Brief, list[str]]:
        return normalize_draft(draft)
