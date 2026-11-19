"""Engine DTOs <-> domain for briefs: the NormalizeDraft, Generate and ApplyBriefOps requests out, their results in."""

from __future__ import annotations

from ....domain.model.brief import BriefBuild, BriefDraft
from ....gen.fga.demo.engine.v1 import engine_pb2 as e
from . import plan_check_mapper


def normalize_request(draft: bytes) -> e.NormalizeDraftRequest:
    return e.NormalizeDraftRequest(draft_json=draft)


def generate_request(brief: bytes, previous_model: bytes, repair: bool) -> e.GenerateRequest:
    return e.GenerateRequest(brief_json=brief, previous_model_json=previous_model, repair=repair)


def apply_ops_request(brief: bytes, ops: bytes, previous_model: bytes) -> e.ApplyBriefOpsRequest:
    return e.ApplyBriefOpsRequest(brief_json=brief, ops_json=ops, previous_model_json=previous_model)


def draft_to_domain(r: e.NormalizeDraftResponse) -> BriefDraft:
    return BriefDraft(brief=r.brief_json, notes=list(r.notes))


def build_to_domain(r: e.BriefResult) -> BriefBuild:
    return BriefBuild(brief=r.brief_json, model=r.model_json, checks=[plan_check_mapper.to_domain(x) for x in r.checks],
                      issues=list(r.issues), ok=r.ok, notes=list(r.notes), attempts=r.attempts,
                      issue_score=r.issue_score, error_rules=list(r.error_rules), checks_line=r.checks_line,
                      total_target_area=r.total_target_area,
                      room_targets={t.room_id: t.target_area for t in r.room_targets}, changed=r.changed)
