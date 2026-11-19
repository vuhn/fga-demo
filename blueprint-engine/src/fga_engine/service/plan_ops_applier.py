from __future__ import annotations

from ..domain import plan_edits
from ..domain.checks import failed
from ..domain.results import PlanResult, PlanStep
from ..domain.schemas import BuildingModel
from ..usecase.checks_runner import ChecksRunner


class PlanOpsApplierService:
    def __init__(self, checks: ChecksRunner):
        self._checks = checks

    def apply_plan_ops(self, model: BuildingModel, ops: plan_edits.PlanOps) -> PlanStep:
        new_model = plan_edits.apply_plan_ops(model, ops)
        checks = self._checks.run_checks(new_model)
        desc = "; ".join(plan_edits.describe(o) for o in ops.ops)
        return PlanStep(result=PlanResult(model=new_model, checks=checks, ok=not failed(checks)),
                        notes=["edited: " + desc], ops=ops, llm_attempts=0)
