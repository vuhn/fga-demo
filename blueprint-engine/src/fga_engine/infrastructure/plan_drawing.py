from __future__ import annotations

from ..domain.schemas import BuildingModel
from ..draw import TitleBlock, build_dxf, to_dxf_bytes
from ..dxf_plan import import_dxf


class EzdxfPlanDrawing:
    """PlanDrawing over the ezdxf drawer and reader (they move under infrastructure in T4b)."""

    def draw(self, model: BuildingModel, title: str) -> bytes:
        return to_dxf_bytes(build_dxf(model, TitleBlock(project=title))[0])

    def read(self, data: bytes) -> list[BuildingModel]:
        return [p.model for p in import_dxf(data).plans]
