"""Engine proto <-> Agent domain."""

from __future__ import annotations

from ...domain.tools import (BriefResult, Catalog, Check, ChecksResult, DraftNormalized, PlanStep, RateSpec, Tool,
                             ToolList)
from ...gen.fga.demo.common.v1 import common_pb2 as c
from ...gen.fga.demo.engine.v1 import engine_pb2 as e


def check_from_pb(x: c.Check) -> Check:
    return Check(rule=x.rule, severity=x.severity, ok=x.ok, message=x.message, room_id=x.room_id, category=x.category,
                 issue_id=x.issue_id, elements=list(x.elements), hint=x.hint)


def tool_list_from_pb(r: e.ListToolsResponse) -> ToolList:
    return ToolList(tools=[Tool(t.name, t.kind, t.description, t.json_schema, t.signature) for t in r.tools],
                    ops_json_schema=r.ops_json_schema, plan_ops_json_schema=r.plan_ops_json_schema,
                    draft_json_schema=r.draft_json_schema)


def catalog_from_pb(r: e.GetCatalogResponse) -> Catalog:
    return Catalog(min_area=dict(r.min_area), min_width=dict(r.min_width), nestable_types=list(r.nestable_types),
                   max_nested=r.max_nested, default_fixtures=dict(r.default_fixtures), fixture_kinds=list(r.fixture_kinds),
                   default_markups=dict(r.default_markups), priced_fixtures=list(r.priced_fixtures),
                   rate_catalog=[RateSpec(x.code, x.package, x.description, x.unit) for x in r.rate_catalog])


def draft_normalized_from_pb(r: e.NormalizeDraftResponse) -> DraftNormalized:
    return DraftNormalized(brief=r.brief_json, notes=list(r.notes))


def brief_result_from_pb(r: e.BriefResult) -> BriefResult:
    return BriefResult(brief=r.brief_json, model=r.model_json, checks=[check_from_pb(x) for x in r.checks],
                       issues=list(r.issues), ok=r.ok, notes=list(r.notes), attempts=r.attempts,
                       issue_score=r.issue_score, error_rules=list(r.error_rules), checks_line=r.checks_line,
                       total_target_area=r.total_target_area,
                       room_targets={t.room_id: t.target_area for t in r.room_targets}, changed=r.changed)


def plan_step_from_pb(r: e.PlanStep) -> PlanStep:
    return PlanStep(model=r.model_json, checks=[check_from_pb(x) for x in r.checks], ok=r.ok, notes=list(r.notes),
                    summary=r.summary, descriptions=list(r.descriptions), changed=r.changed)


def checks_result_from_pb(r: e.RunChecksResponse) -> ChecksResult:
    return ChecksResult(checks=[check_from_pb(x) for x in r.checks], ok=r.ok, issue_score=r.issue_score)
