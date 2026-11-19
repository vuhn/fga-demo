from __future__ import annotations

from ..domain.checks import failed, run_checks, run_imported_checks
from ..domain.results import CheckReport
from ..domain.schemas import Brief, BuildingModel, CheckResult


class ChecksRunnerService:
    def run_checks(self, model: BuildingModel, brief: Brief | None = None) -> list[CheckResult]:
        return run_imported_checks(model) if brief is None else run_checks(brief, model)

    def check_plan(self, model: BuildingModel, brief: Brief | None = None) -> CheckReport:
        checks = self.run_checks(model, brief)
        return CheckReport(checks=checks, ok=not failed(checks),
                           issue_score=10 * len(failed(checks)) + len(failed(checks, "warning")))
