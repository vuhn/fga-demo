from __future__ import annotations

from ..domain.results import PlanBuild, PlanCreated
from ..domain.schemas import Brief
from ..usecase.generator import Generator
from ..usecase.plan_drawer import PlanDrawer


class PlanCreatorService:
    """Create a plan the way an import does. The brief is only the input: the layout is drawn into our own
    DXF, and that file is read back like any CAD file, so the plan has no brief afterwards, keeps the file
    as its original, and is edited, chatted about and exported like an imported one."""

    def __init__(self, generator: Generator, drawer: PlanDrawer):
        self._generator, self._drawer = generator, drawer

    def create_plan(self, brief: Brief) -> PlanCreated:
        step = self._generator.generate(brief, repair=True)
        r = step.result
        if r.model is None:
            return PlanCreated(result=PlanBuild(model=None, issues=r.issues, notes=r.notes), notes=step.notes)
        out = self._drawer.draw_plan(r.model, brief.title)
        out.result.notes = r.notes + out.result.notes
        out.notes = step.notes
        return out
