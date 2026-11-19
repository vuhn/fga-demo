from __future__ import annotations

from typing import Protocol

from ..files import ConvertReport


class PlanConversion(Protocol):
    def to_dxf(self, data: bytes, mime: str, scale: int | None, dpi: float | None,
               longest_m: float | None) -> tuple[bytes, ConvertReport]:
        """A PDF, PNG or JPEG floor plan converted into a DXF (metres)."""
