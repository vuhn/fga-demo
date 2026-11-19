"""Engine DTOs <-> domain for the plan summary: the request out (with the session's facts), the summary in."""

from __future__ import annotations

from ....domain.model.estimate_facts import EstimateFacts
from ....domain.model.plan_summary import PlanSummary, SummaryFixture, SummaryOpening, SummaryRoom
from fga.demo.engine.v1 import engine_pb2 as e
from .estimate_facts_mapper import facts_to_pb

def request(model: bytes, facts: EstimateFacts) -> e.SummarizePlanRequest:
    return e.SummarizePlanRequest(model_json=model, facts=facts_to_pb(facts))


def to_domain(r: e.PlanSummary) -> PlanSummary:
    return PlanSummary(rooms=[SummaryRoom(x.id, x.name, x.type, x.area_m2) for x in r.rooms],
                       fixtures=[SummaryFixture(x.id, x.kind, x.room_id) for x in r.fixtures],
                       openings=[SummaryOpening(x.id, x.kind, list(x.room_ids), x.width) for x in r.openings],
                       pending_fixtures=list(r.pending_fixtures), structure_in_scope=r.structure_in_scope)
