"""Engine proto -> Agent domain. The only place that knows both shapes (empty string = absent, JSON text = dict)."""

from __future__ import annotations

import json

from ...domain.brief import BriefBuild, BriefDraft
from ...domain.design_limits import DesignLimits, RateSpec
from ...domain.operation_catalog import EditOperation, OperationCatalog
from ...domain.plan import PlanRevision
from ...domain.plan_check import PlanCheck, PlanCheckReport
from ...gen.fga.demo.common.v1 import common_pb2 as c
from ...gen.fga.demo.engine.v1 import engine_pb2 as e


def plan_check(x: c.Check) -> PlanCheck:
    return PlanCheck(rule=x.rule, severity=x.severity, ok=x.ok, message=x.message, room_id=x.room_id or None,
                     category=x.category, issue_id=x.issue_id, elements=list(x.elements), hint=x.hint)


def operation_catalog(r: e.ListToolsResponse) -> OperationCatalog:
    return OperationCatalog(
        operations=[EditOperation(t.name, t.kind, t.description, json.loads(t.json_schema or "{}"), t.signature)
                    for t in r.tools],
        brief_ops_schema=json.loads(r.ops_json_schema or "{}"), plan_ops_schema=json.loads(r.plan_ops_json_schema or "{}"),
        draft_schema=json.loads(r.draft_json_schema or "{}"))


def design_limits(r: e.GetCatalogResponse) -> DesignLimits:
    return DesignLimits(min_area=dict(r.min_area), min_width=dict(r.min_width), nestable_types=list(r.nestable_types),
                        max_nested=r.max_nested, default_fixtures=dict(r.default_fixtures),
                        fixture_kinds=list(r.fixture_kinds), default_markups=dict(r.default_markups),
                        priced_fixtures=list(r.priced_fixtures),
                        rate_catalog=[RateSpec(x.code, x.package, x.description, x.unit) for x in r.rate_catalog])


def brief_draft(r: e.NormalizeDraftResponse) -> BriefDraft:
    return BriefDraft(brief=r.brief_json, notes=list(r.notes))


def brief_build(r: e.BriefResult) -> BriefBuild:
    return BriefBuild(brief=r.brief_json, model=r.model_json, checks=[plan_check(x) for x in r.checks],
                      issues=list(r.issues), ok=r.ok, notes=list(r.notes), attempts=r.attempts,
                      issue_score=r.issue_score, error_rules=list(r.error_rules), checks_line=r.checks_line,
                      total_target_area=r.total_target_area,
                      room_targets={t.room_id: t.target_area for t in r.room_targets}, changed=r.changed)


def plan_revision(r: e.PlanStep) -> PlanRevision:
    return PlanRevision(model=r.model_json, checks=[plan_check(x) for x in r.checks], ok=r.ok, notes=list(r.notes),
                        summary=r.summary, descriptions=list(r.descriptions), changed=r.changed)


def plan_check_report(r: e.RunChecksResponse) -> PlanCheckReport:
    return PlanCheckReport(checks=[plan_check(x) for x in r.checks], ok=r.ok, issue_score=r.issue_score)
