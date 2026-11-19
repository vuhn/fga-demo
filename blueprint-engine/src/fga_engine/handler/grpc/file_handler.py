"""File endpoints (ConvertToDXF, ImportDXF, PatchDXF, Render). Decode, call one use case, encode through mapper/."""

from __future__ import annotations

from dataclasses import dataclass

from ...usecase.dxf_importer import DxfImporter
from ...usecase.dxf_patcher import DxfPatcher
from ...usecase.plan_converter import PlanConverter
from ...usecase.plan_renderer import PlanRenderer
from .common.error_helper import handled
from .mapper import file_mapper


@dataclass(frozen=True)
class FileUsecases:
    plan_converter: PlanConverter
    dxf_importer: DxfImporter
    dxf_patcher: DxfPatcher
    plan_renderer: PlanRenderer


class FileHandler:
    def __init__(self, usecases: FileUsecases) -> None:
        self._uc = usecases

    @handled
    def ConvertToDXF(self, request, context):
        return file_mapper.converted_to_pb(self._uc.plan_converter.convert(*file_mapper.convert_to_domain(request)))

    @handled
    def ImportDXF(self, request, context):
        return file_mapper.imported_to_pb(self._uc.dxf_importer.import_dxf(*file_mapper.import_to_domain(request)))

    @handled
    def PatchDXF(self, request, context):
        return file_mapper.patched_to_pb(self._uc.dxf_patcher.patch(*file_mapper.patch_to_domain(request)))

    @handled
    def Render(self, request, context):
        model, fmt, title_block = file_mapper.render_to_domain(request)
        return file_mapper.rendered_to_pb(self._uc.plan_renderer.render(model, fmt, title_block), fmt)
