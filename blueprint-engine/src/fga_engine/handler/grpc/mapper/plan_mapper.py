"""Plan DTOs <-> domain (CreatePlan, DrawPlan, ApplyPlanOps), both directions."""

from __future__ import annotations

import base64

from ....domain import plan_edits
from ....domain.results import PlanCreated, PlanEdit
from ....domain.schemas import Brief, BuildingModel
from fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import dump_json, load_json
from . import check_mapper


def create_to_domain(request: e.CreatePlanRequest) -> Brief:
    return load_json(Brief, request.brief_json, "brief_json")


def draw_to_domain(request: e.DrawPlanRequest) -> tuple[BuildingModel, str]:
    return load_json(BuildingModel, request.model_json, "model_json"), request.title


def edit_to_domain(request: e.ApplyPlanOpsRequest) -> tuple[BuildingModel, plan_edits.PlanOps]:
    return (load_json(BuildingModel, request.model_json, "model_json"),
            load_json(plan_edits.PlanOps, request.ops_json, "ops_json"))


def outcome_to_pb(created: PlanCreated) -> e.PlanOutcome:
    r = created.result
    return e.PlanOutcome(model_json=dump_json(r.model) if r.model is not None else b"",
                         checks=check_mapper.list_to_pb(r.checks), issues=r.issues, ok=r.ok,
                         notes=created.notes or r.notes, dxf=base64.b64decode(created.dxf) if created.dxf else b"")


def edit_to_pb(edit: PlanEdit) -> e.PlanStep:
    res = edit.step.result
    return e.PlanStep(model_json=dump_json(res.model), checks=check_mapper.list_to_pb(res.checks), ok=res.ok,
                      notes=edit.step.notes, summary=edit.summary, descriptions=edit.descriptions, changed=edit.changed)
