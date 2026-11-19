from __future__ import annotations

from typing import Protocol

from ..files import TitleBlock
from ..schemas import BuildingModel


class PlanRendering(Protocol):
    def render(self, model: BuildingModel, fmt: str, title_block: TitleBlock) -> bytes | str:
        """The model as an svg (str), dxf, pdf or ifc (bytes) file."""
