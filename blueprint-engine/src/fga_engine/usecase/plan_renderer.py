from __future__ import annotations

from typing import Protocol

from ..domain.files import TitleBlock
from ..domain.schemas import BuildingModel


class PlanRenderer(Protocol):
    def render(self, model: BuildingModel, fmt: str, title_block: TitleBlock) -> bytes | str:
        """The model as a file: svg as text, dxf, pdf and ifc as bytes."""
