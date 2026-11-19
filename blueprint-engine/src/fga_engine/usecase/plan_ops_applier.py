from __future__ import annotations

from typing import Protocol

from ..domain.plan_edits import PlanOps
from ..domain.results import PlanStep
from ..domain.schemas import BuildingModel


class PlanOpsApplier(Protocol):
    def apply_plan_ops(self, model: BuildingModel, ops: PlanOps) -> PlanStep:
        """Explicit edit ops (from a person, not an LLM) applied to an imported plan, then checked."""
