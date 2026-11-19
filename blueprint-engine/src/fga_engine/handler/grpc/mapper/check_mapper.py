"""Check DTOs <-> domain (`common.v1.Check`, RunChecks), both directions."""

from __future__ import annotations

from ....domain.results import CheckReport
from ....domain.schemas import Brief, BuildingModel, CheckResult
from ....gen.fga.demo.common.v1 import common_pb2 as c
from ....gen.fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import load_json, load_optional_json


def to_pb(ch: CheckResult) -> c.Check:
    return c.Check(rule=ch.rule, severity=ch.severity, ok=ch.ok, message=ch.message, room_id=ch.room_id or "",
                   category=ch.category, issue_id=ch.issue_id, elements=ch.elements, hint=ch.hint)


def list_to_pb(checks: list[CheckResult]) -> list[c.Check]:
    return [to_pb(x) for x in checks]


def run_checks_to_domain(request: e.RunChecksRequest) -> tuple[BuildingModel, Brief | None]:
    return (load_json(BuildingModel, request.model_json, "model_json"),
            load_optional_json(Brief, request.brief_json, "brief_json"))


def report_to_pb(report: CheckReport) -> e.RunChecksResponse:
    return e.RunChecksResponse(checks=list_to_pb(report.checks), ok=report.ok, issue_score=report.issue_score)
