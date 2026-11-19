from __future__ import annotations

from ..domain.estimate import EstimateFacts
from ..domain.estimate_rules import pending_fixtures, structure_in_scope
from ..domain.results import PlanSummary, SummaryFixture, SummaryOpening, SummaryRoom
from ..domain.schemas import BuildingModel


class PlanSummarizerService:
    def summarize_plan(self, model: BuildingModel, facts: EstimateFacts) -> PlanSummary:
        return PlanSummary(
            rooms=[SummaryRoom(id=r.id, name=r.name, type=r.type, area_m2=round(r.net_area, 3)) for r in model.rooms],
            fixtures=[SummaryFixture(id=f.id, kind=f.kind, room_id=f.room_id) for f in model.fixtures],
            openings=[SummaryOpening(id=o.id, kind="door" if o.kind == "entrance" else o.kind, room_ids=list(o.rooms), width=o.width)
                      for o in model.openings],
            pending_fixtures=[f.id for f in pending_fixtures(model, facts)],
            structure_in_scope=structure_in_scope(facts),
        )
