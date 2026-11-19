"""Domain `PlanCheck` -> `common.v1.Check` (the Agent's outgoing DTO)."""

from __future__ import annotations

from ....domain.model.plan_check import PlanCheck
from ....gen.fga.demo.common.v1 import common_pb2 as c


def to_pb(x: PlanCheck) -> c.Check:
    return c.Check(rule=x.rule, severity=x.severity, ok=x.ok, message=x.message, room_id=x.room_id or "",
                   category=x.category, issue_id=x.issue_id, elements=x.elements, hint=x.hint)
