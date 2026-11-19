"""Engine DTOs <-> domain for the plan summary: the estimate facts out, the summary in."""

from __future__ import annotations

from ....domain.model.estimate_facts import EstimateFacts
from ....domain.model.plan_summary import PlanSummary, SummaryFixture, SummaryOpening, SummaryRoom
from fga.demo.common.v1 import common_pb2 as c
from fga.demo.engine.v1 import engine_pb2 as e

_SOURCE = {"user": c.FACT_SOURCE_USER, "plan": c.FACT_SOURCE_PLAN, "assumed": c.FACT_SOURCE_ASSUMED}
_STATE = {"empty": c.ROOM_STATE_EMPTY, "existing": c.ROOM_STATE_EXISTING, "to_replace": c.ROOM_STATE_TO_REPLACE,
          "new": c.ROOM_STATE_NEW}


def _fact(name: str, value, source: str) -> c.Fact:
    # bool before number: a flag is an int in Python
    if isinstance(value, bool):
        return c.Fact(name=name, flag=value, source=_SOURCE[source])
    if isinstance(value, (int, float)):
        return c.Fact(name=name, number=float(value), source=_SOURCE[source])
    return c.Fact(name=name, text=str(value), source=_SOURCE[source])


def facts_to_pb(x: EstimateFacts) -> c.EstimateFacts:
    return c.EstimateFacts(facts=[_fact(k, f.value, f.source) for k, f in x.facts.items()],
                           scope=[c.RoomScope(room_id=k, state=_STATE[s.state], source=_SOURCE[s.source]) for k, s in x.scope.items()],
                           fixtures_existing=x.fixtures_existing, excluded=x.excluded)


def request(model: bytes, facts: EstimateFacts) -> e.SummarizePlanRequest:
    return e.SummarizePlanRequest(model_json=model, facts=facts_to_pb(facts))


def to_domain(r: e.PlanSummary) -> PlanSummary:
    return PlanSummary(rooms=[SummaryRoom(x.id, x.name, x.type, x.area_m2) for x in r.rooms],
                       fixtures=[SummaryFixture(x.id, x.kind, x.room_id) for x in r.fixtures],
                       openings=[SummaryOpening(x.id, x.kind, list(x.room_ids), x.width) for x in r.openings],
                       pending_fixtures=list(r.pending_fixtures), structure_in_scope=r.structure_in_scope)
