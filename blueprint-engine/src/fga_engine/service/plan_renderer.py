from __future__ import annotations

from ..domain.files import TitleBlock
from ..domain.gateway import PlanRendering
from ..domain.schemas import BuildingModel


class PlanRendererService:
    def __init__(self, rendering: PlanRendering):
        self._rendering = rendering

    def render(self, model: BuildingModel, fmt: str, title_block: TitleBlock) -> bytes | str:
        return self._rendering.render(model, fmt, title_block)
