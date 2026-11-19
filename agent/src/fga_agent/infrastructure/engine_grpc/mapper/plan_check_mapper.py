"""Engine `common.v1.Check` / `RunChecksResponse` -> domain."""

from __future__ import annotations

from ....domain.model.plan_check import PlanCheck, PlanCheckReport
from ....gen.fga.demo.common.v1 import common_pb2 as c
from ....gen.fga.demo.engine.v1 import engine_pb2 as e


def plan_check(x: c.Check) -> PlanCheck:
    return PlanCheck(rule=x.rule, severity=x.severity, ok=x.ok, message=x.message, room_id=x.room_id or None,
                     category=x.category, issue_id=x.issue_id, elements=list(x.elements), hint=x.hint)


def plan_check_report(r: e.RunChecksResponse) -> PlanCheckReport:
    return PlanCheckReport(checks=[plan_check(x) for x in r.checks], ok=r.ok, issue_score=r.issue_score)
