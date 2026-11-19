from __future__ import annotations

from ..domain.files import ConvertReport, ImportResult, PatchReport, TitleBlock
from ..domain.schemas import BuildingModel
from . import convert, dxf_patch, dxf_plan
from .draw import build_dxf, to_dxf_bytes, to_pdf, to_svg
from .ifc import to_ifc


class EzdxfPlanDrawing:
    """PlanDrawing over the ezdxf drawer and reader."""

    def draw(self, model: BuildingModel, title: str) -> bytes:
        return to_dxf_bytes(build_dxf(model, TitleBlock(project=title))[0])

    def read(self, data: bytes) -> list[BuildingModel]:
        return [p.model for p in dxf_plan.import_dxf(data).plans]


class EzdxfPlanRendering:
    def render(self, model: BuildingModel, fmt: str, title_block: TitleBlock) -> bytes | str:
        if fmt == "ifc":
            return to_ifc(model, title_block.project)
        doc, sheet = build_dxf(model, title_block)
        if fmt == "dxf":
            return to_dxf_bytes(doc)
        if fmt == "pdf":
            return to_pdf(doc, sheet)
        return to_svg(doc, sheet)


class EzdxfDxfReading:
    def read_plans(self, data: bytes, unit: str | None) -> ImportResult:
        return dxf_plan.import_dxf(data, unit)


class RasterPlanConversion:
    def to_dxf(self, data: bytes, mime: str, scale: int | None, dpi: float | None,
               longest_m: float | None) -> tuple[bytes, ConvertReport]:
        return convert.to_dxf(data, mime, scale=scale, dpi=dpi, longest_m=longest_m)


class EzdxfDxfPatching:
    def patch(self, original: bytes, base: BuildingModel, current: BuildingModel) -> tuple[bytes, PatchReport]:
        return dxf_patch.patch_dxf(original, base, current)
