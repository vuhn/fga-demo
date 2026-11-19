from __future__ import annotations

from ..domain.checks import failed, run_imported_checks
from ..domain.files import ImportOut, ImportPlanOut, TitleBlock
from ..domain.gateway import DxfReading, PlanRendering


class DxfImporterService:
    def __init__(self, reading: DxfReading, rendering: PlanRendering):
        self._reading, self._rendering = reading, rendering

    def import_dxf(self, data: bytes, unit: str | None) -> ImportOut:
        """One plan per cluster of walls found in the file, each with its checks and its drawing for review."""
        result = self._reading.read_plans(data, unit)
        plans = []
        for p in result.plans:
            checks = run_imported_checks(p.model)
            svg = str(self._rendering.render(p.model, "svg", TitleBlock(project=p.name)))
            plans.append(ImportPlanOut(**p.model_dump(), checks=checks, ok=not failed(checks), svg=svg))
        return ImportOut(plans=plans, report=result.report)
