from __future__ import annotations

import base64

from ..domain.checks import failed
from ..domain.edits import EditError
from ..domain.gateway import PlanDrawing
from ..domain.results import PlanBuild, PlanCreated
from ..domain.schemas import BuildingModel
from ..usecase.checks_runner import ChecksRunner


class PlanDrawerService:
    def __init__(self, drawing: PlanDrawing, checks: ChecksRunner):
        self._drawing, self._checks = drawing, checks

    def draw_plan(self, model: BuildingModel, title: str) -> PlanCreated:
        """Draw a model into our own DXF and read it back as a plan with provenance: the file becomes the plan's
        original. Used for a plan made from a brief and for converting a plan made before DXF plans existed."""
        if model.source != "generated":
            raise EditError("this plan is already a CAD plan")
        data = self._drawing.draw(model, title[:80] or "Floor plan")
        plans = self._drawing.read(data)
        if len(plans) != 1:  # our own drawing must read back as one plan; anything else is a bug, not a bad input
            raise EditError("the drawn plan could not be read back as one plan")
        plan = plans[0]
        before, after = sum(r.net_area for r in model.rooms), sum(r.net_area for r in plan.rooms)
        if abs(after - before) > 0.03 * before or len(plan.fixtures) != len(model.fixtures):
            raise EditError(f"the drawing reads back as {after:.0f} m² and {len(plan.fixtures)} fixtures, but the plan has "
                            f"{before:.0f} m² and {len(model.fixtures)}; it cannot be converted without losing parts of it")
        notes = []
        if len(plan.rooms) != len(model.rooms):  # an open plan has no walls between its rooms: a CAD file shows one space
            notes.append(f"{len(model.rooms)} rooms became {len(plan.rooms)} spaces: rooms with no wall between them "
                         "(an open plan) are one space in the drawing")
        checks = self._checks.run_checks(plan)
        return PlanCreated(result=PlanBuild(model=plan, checks=checks, ok=not failed(checks), notes=notes), notes=[],
                           dxf=base64.b64encode(data).decode("ascii"))
