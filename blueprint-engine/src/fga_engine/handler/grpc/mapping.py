"""Proto <-> domain conversion for the Engine's gRPC handlers. JSON bytes carry plans, briefs and ops."""

from __future__ import annotations

from typing import TypeVar

from pydantic import BaseModel

from ...domain.estimate import (Estimate, EstimateFacts, Fact, RateResult, RateSource, RoomScope, Takeoff,
                                TakeoffLine)
from ...domain.results import BriefResult, PlanSummary
from ...domain.schemas import CheckResult
from ...gen.fga.demo.common.v1 import common_pb2 as c
from ...gen.fga.demo.engine.v1 import engine_pb2 as e

M = TypeVar("M", bound=BaseModel)

_SOURCE = {"user": c.FACT_SOURCE_USER, "plan": c.FACT_SOURCE_PLAN, "assumed": c.FACT_SOURCE_ASSUMED}
_SOURCE_BACK = {v: k for k, v in _SOURCE.items()}
_STATE = {"empty": c.ROOM_STATE_EMPTY, "existing": c.ROOM_STATE_EXISTING, "to_replace": c.ROOM_STATE_TO_REPLACE,
          "new": c.ROOM_STATE_NEW}
_STATE_BACK = {v: k for k, v in _STATE.items()}


def load_json(model: type[M], data: bytes, what: str) -> M:
    if not data:
        raise ValueError(f"{what} is empty")
    return model.model_validate_json(data)


def dump_json(obj: BaseModel) -> bytes:
    return obj.model_dump_json(exclude_none=False).encode()


# --- checks --------------------------------------------------------------------------------------------------

def check_to_pb(ch: CheckResult) -> c.Check:
    return c.Check(rule=ch.rule, severity=ch.severity, ok=ch.ok, message=ch.message, room_id=ch.room_id or "",
                   category=ch.category, issue_id=ch.issue_id, elements=ch.elements, hint=ch.hint)


def checks_to_pb(checks: list[CheckResult]) -> list[c.Check]:
    return [check_to_pb(x) for x in checks]


# --- estimate facts, takeoff, rates ----------------------------------------------------------------------------

def facts_from_pb(f: c.EstimateFacts) -> EstimateFacts:
    facts: dict[str, Fact] = {}
    for x in f.facts:
        kind = x.WhichOneof("value")
        value = getattr(x, kind) if kind else ""
        facts[x.name] = Fact(value=value, source=_SOURCE_BACK.get(x.source, "assumed"))
    scope = {s.room_id: RoomScope(state=_STATE_BACK.get(s.state, "existing"), source=_SOURCE_BACK.get(s.source, "assumed"))
             for s in f.scope}
    return EstimateFacts(facts=facts, scope=scope, fixtures_existing=list(f.fixtures_existing),
                         excluded=list(f.excluded))


def takeoff_to_pb(tk: Takeoff) -> e.TakeoffResponse:
    return e.TakeoffResponse(status=tk.status, warnings=tk.warnings, lines=[
        e.TakeoffLine(code=ln.code, package=ln.package, description=ln.description, quantity=ln.quantity, unit=ln.unit,
                      basis=ln.basis, confidence=ln.confidence, deps=ln.deps, room_id=ln.room_id or "")
        for ln in tk.lines])


def takeoff_from_pb(t: e.TakeoffResponse) -> Takeoff:
    return Takeoff(status=t.status or "ok", warnings=list(t.warnings), lines=[
        TakeoffLine(code=ln.code, package=ln.package, description=ln.description, quantity=ln.quantity, unit=ln.unit,
                    basis=ln.basis, confidence=ln.confidence or "assumed", deps=list(ln.deps),
                    room_id=ln.room_id or None) for ln in t.lines])


def rate_from_pb(r: c.RateResult) -> RateResult:
    return RateResult(code=r.code, low=r.low if r.HasField("low") else None, base=r.base if r.HasField("base") else None,
                      high=r.high if r.HasField("high") else None, unit=r.unit, currency=r.currency or "VND",
                      confidence=r.confidence or "none", region=r.region, quality=r.quality, note=r.note,
                      sources=[RateSource(url=s.url, quote=s.quote, retrieved_at=s.retrieved_at) for s in r.sources])


def estimate_to_pb(est: Estimate) -> e.PriceEstimateResponse:
    return e.PriceEstimateResponse(estimate_json=dump_json(est), total_low=est.total.get("low", 0),
                                   total_base=est.total.get("base", 0), total_high=est.total.get("high", 0),
                                   warnings=est.warnings)


# --- plan summary ---------------------------------------------------------------------------------------------

def summary_to_pb(s: PlanSummary) -> e.PlanSummary:
    return e.PlanSummary(
        rooms=[e.PlanSummary.Room(id=r.id, name=r.name, type=r.type, area_m2=r.area_m2) for r in s.rooms],
        fixtures=[e.PlanSummary.Fixture(id=f.id, kind=f.kind, room_id=f.room_id) for f in s.fixtures],
        openings=[e.PlanSummary.Opening(id=o.id, kind=o.kind, room_ids=o.room_ids, width=o.width) for o in s.openings],
        pending_fixtures=s.pending_fixtures, structure_in_scope=s.structure_in_scope)


# --- brief result ---------------------------------------------------------------------------------------------

def brief_result_to_pb(r: BriefResult) -> e.BriefResult:
    return e.BriefResult(
        brief_json=dump_json(r.brief), model_json=dump_json(r.model) if r.model is not None else b"",
        checks=checks_to_pb(r.checks), issues=r.issues, ok=r.ok, notes=r.notes, attempts=r.attempts,
        issue_score=r.issue_score, error_rules=r.error_rules, checks_line=r.checks_line,
        total_target_area=r.total_target_area, changed=r.changed,
        room_targets=[e.RoomTarget(room_id=k, target_area=v) for k, v in r.room_targets.items()])
