"""Plan summary DTOs <-> domain (SummarizePlan), both directions."""

from __future__ import annotations

from ....domain.estimate import EstimateFacts
from ....domain.results import PlanSummary
from ....domain.schemas import BuildingModel
from fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import load_json
from .estimate_mapper import facts_to_domain


def to_domain(request: e.SummarizePlanRequest) -> tuple[BuildingModel, EstimateFacts]:
    return load_json(BuildingModel, request.model_json, "model_json"), facts_to_domain(request.facts)


def to_pb(s: PlanSummary) -> e.PlanSummary:
    return e.PlanSummary(
        rooms=[e.PlanSummary.Room(id=r.id, name=r.name, type=r.type, area_m2=r.area_m2) for r in s.rooms],
        fixtures=[e.PlanSummary.Fixture(id=f.id, kind=f.kind, room_id=f.room_id) for f in s.fixtures],
        openings=[e.PlanSummary.Opening(id=o.id, kind=o.kind, room_ids=o.room_ids, width=o.width) for o in s.openings],
        pending_fixtures=s.pending_fixtures, structure_in_scope=s.structure_in_scope)
