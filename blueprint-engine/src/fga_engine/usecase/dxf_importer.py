from __future__ import annotations

from typing import Protocol

from ..domain.files import ImportOut


class DxfImporter(Protocol):
    def import_dxf(self, data: bytes, unit: str | None) -> ImportOut:
        """Every plan in a CAD file with its checks and drawing; EditError when the file cannot be read."""
