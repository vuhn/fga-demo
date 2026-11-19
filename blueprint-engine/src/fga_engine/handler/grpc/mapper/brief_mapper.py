"""Brief DTOs <-> domain (NormalizeDraft, Generate, ApplyBriefOps), both directions."""

from __future__ import annotations

from ....domain.draft import DraftBrief
from ....domain.results import BriefResult
from ....domain.schemas import Brief, BuildingModel, EditOps
from fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import dump_json, load_json, load_optional_json
from . import check_mapper


def normalize_to_domain(request: e.NormalizeDraftRequest) -> DraftBrief:
    return load_json(DraftBrief, request.draft_json, "draft_json")


def normalized_to_pb(brief: Brief, notes: list[str]) -> e.NormalizeDraftResponse:
    return e.NormalizeDraftResponse(brief_json=dump_json(brief), notes=notes)


def generate_to_domain(request: e.GenerateRequest) -> tuple[Brief, BuildingModel | None, bool]:
    return (load_json(Brief, request.brief_json, "brief_json"),
            load_optional_json(BuildingModel, request.previous_model_json, "previous_model_json"), request.repair)


def apply_ops_to_domain(request: e.ApplyBriefOpsRequest) -> tuple[Brief, EditOps, BuildingModel | None]:
    return (load_json(Brief, request.brief_json, "brief_json"), load_json(EditOps, request.ops_json, "ops_json"),
            load_optional_json(BuildingModel, request.previous_model_json, "previous_model_json"))


def result_to_pb(r: BriefResult) -> e.BriefResult:
    return e.BriefResult(
        brief_json=dump_json(r.brief), model_json=dump_json(r.model) if r.model is not None else b"",
        checks=check_mapper.list_to_pb(r.checks), issues=r.issues, ok=r.ok, notes=r.notes, attempts=r.attempts,
        issue_score=r.issue_score, error_rules=r.error_rules, checks_line=r.checks_line,
        total_target_area=r.total_target_area, changed=r.changed,
        room_targets=[e.RoomTarget(room_id=k, target_area=v) for k, v in r.room_targets.items()])
