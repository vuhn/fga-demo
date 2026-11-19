from __future__ import annotations

from ..domain.edits import apply_ops
from ..domain.results import BriefResult, brief_result
from ..domain.schemas import Brief, BuildingModel, EditOps
from ..usecase.generator import Generator


class BriefOpsApplierService:
    def __init__(self, generator: Generator):
        self._generator = generator

    def apply_brief_ops(self, brief: Brief, ops: EditOps, previous: BuildingModel | None = None) -> BriefResult:
        new_brief = apply_ops(brief, ops)
        step = self._generator.generate(new_brief, previous, repair=True)
        return brief_result(step.result, notes=step.notes, attempts=step.attempts, changed=new_brief != brief)
