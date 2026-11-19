from __future__ import annotations

from typing import Protocol

from ..domain.files import PatchOut


class DxfPatcher(Protocol):
    def patch(self, original: bytes, base_json: str, current_json: str) -> PatchOut:
        """The original file with the plan changes written into it; EditError for bad input."""
