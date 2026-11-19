from __future__ import annotations

from typing import Protocol

from ..domain.files import ConvertOut


class PlanConverter(Protocol):
    def convert(self, data: bytes, mime: str, scale: int | None, dpi: float | None,
                longest_m: float | None) -> ConvertOut:
        """A PDF, PNG or JPEG floor plan as a DXF; EditError when it is too large or cannot be read."""
