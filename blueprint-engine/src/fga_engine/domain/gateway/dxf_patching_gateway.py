from __future__ import annotations

from typing import Protocol

from ..files import PatchReport
from ..schemas import BuildingModel


class DxfPatching(Protocol):
    def patch(self, original: bytes, base: BuildingModel, current: BuildingModel) -> tuple[bytes, PatchReport]:
        """The original CAD file with the changes between base and current written into it."""
