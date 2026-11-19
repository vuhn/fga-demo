"""Engine `NormalizeDraftResponse` / `BriefResult` -> domain."""

from __future__ import annotations

from ....domain.model.brief import BriefBuild, BriefDraft
from ....gen.fga.demo.engine.v1 import engine_pb2 as e
from .plan_check_mapper import plan_check


def brief_draft(r: e.NormalizeDraftResponse) -> BriefDraft:
    return BriefDraft(brief=r.brief_json, notes=list(r.notes))


def brief_build(r: e.BriefResult) -> BriefBuild:
    return BriefBuild(brief=r.brief_json, model=r.model_json, checks=[plan_check(x) for x in r.checks],
                      issues=list(r.issues), ok=r.ok, notes=list(r.notes), attempts=r.attempts,
                      issue_score=r.issue_score, error_rules=list(r.error_rules), checks_line=r.checks_line,
                      total_target_area=r.total_target_area,
                      room_targets={t.room_id: t.target_area for t in r.room_targets}, changed=r.changed)
