from __future__ import annotations

from ..domain.checks import run_checks, run_imported_checks
from ..domain.schemas import Brief, BuildingModel, CheckResult


class ChecksRunnerService:
    def run_checks(self, model: BuildingModel, brief: Brief | None = None) -> list[CheckResult]:
        return run_imported_checks(model) if brief is None else run_checks(brief, model)
