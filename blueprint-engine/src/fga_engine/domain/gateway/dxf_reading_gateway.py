from __future__ import annotations

from typing import Protocol

from ..files import ImportResult


class DxfReading(Protocol):
    def read_plans(self, data: bytes, unit: str | None) -> ImportResult:
        """The plans found in a CAD file; EditError when it is not a usable file."""
