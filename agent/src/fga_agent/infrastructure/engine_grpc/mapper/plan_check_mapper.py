"""Engine DTOs <-> domain for plan checks: the RunChecks request out, the check and the report in."""

from __future__ import annotations

from ....domain.model.plan_check import PlanCheck, PlanCheckReport
from fga.demo.common.v1 import common_pb2 as c
from fga.demo.engine.v1 import engine_pb2 as e


def run_checks_request(model: bytes, brief: bytes) -> e.RunChecksRequest:
    return e.RunChecksRequest(model_json=model, brief_json=brief)


def to_domain(x: c.Check) -> PlanCheck:
    return PlanCheck(rule=x.rule, severity=x.severity, ok=x.ok, message=x.message, room_id=x.room_id or None,
                     category=x.category, issue_id=x.issue_id, elements=list(x.elements), hint=x.hint)


def report_to_domain(r: e.RunChecksResponse) -> PlanCheckReport:
    return PlanCheckReport(checks=[to_domain(x) for x in r.checks], ok=r.ok, issue_score=r.issue_score)
