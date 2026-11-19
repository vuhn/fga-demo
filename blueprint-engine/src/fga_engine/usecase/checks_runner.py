from __future__ import annotations

from typing import Protocol

from ..domain.results import CheckReport
from ..domain.schemas import Brief, BuildingModel, CheckResult


class ChecksRunner(Protocol):
    def run_checks(self, model: BuildingModel, brief: Brief | None = None) -> list[CheckResult]:
        """Rule checks. With a brief the plan is judged against it; without one (an imported plan) it is
        taken as drawn and only its own consistency is judged."""

    def check_plan(self, model: BuildingModel, brief: Brief | None = None) -> CheckReport:
        """run_checks, judged: ok when nothing failed, and the issue score (10 per failed error, 1 per failed warning)."""
