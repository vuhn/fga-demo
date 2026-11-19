"""Estimate DTOs <-> domain: facts, takeoff, rates and the estimate (Takeoff, PriceEstimate), both directions."""

from __future__ import annotations

from ....domain.estimate import (Estimate, EstimateFacts, Fact, RateResult, RateSource, RoomScope, Takeoff,
                                 TakeoffLine)
from ....domain.schemas import BuildingModel
from ....gen.fga.demo.common.v1 import common_pb2 as c
from ....gen.fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import dump_json, load_json

_SOURCE = {"user": c.FACT_SOURCE_USER, "plan": c.FACT_SOURCE_PLAN, "assumed": c.FACT_SOURCE_ASSUMED}
_SOURCE_BACK = {v: k for k, v in _SOURCE.items()}
_STATE = {"empty": c.ROOM_STATE_EMPTY, "existing": c.ROOM_STATE_EXISTING, "to_replace": c.ROOM_STATE_TO_REPLACE,
          "new": c.ROOM_STATE_NEW}
_STATE_BACK = {v: k for k, v in _STATE.items()}


def facts_to_domain(f: c.EstimateFacts) -> EstimateFacts:
    facts: dict[str, Fact] = {}
    for x in f.facts:
        kind = x.WhichOneof("value")
        facts[x.name] = Fact(value=getattr(x, kind) if kind else "", source=_SOURCE_BACK.get(x.source, "assumed"))
    scope = {s.room_id: RoomScope(state=_STATE_BACK.get(s.state, "existing"), source=_SOURCE_BACK.get(s.source, "assumed"))
             for s in f.scope}
    return EstimateFacts(facts=facts, scope=scope, fixtures_existing=list(f.fixtures_existing),
                         excluded=list(f.excluded))


def takeoff_to_domain(request: e.TakeoffRequest) -> tuple[BuildingModel, EstimateFacts]:
    return load_json(BuildingModel, request.model_json, "model_json"), facts_to_domain(request.facts)


def takeoff_to_pb(tk: Takeoff) -> e.TakeoffResponse:
    return e.TakeoffResponse(status=tk.status, warnings=tk.warnings, lines=[
        e.TakeoffLine(code=ln.code, package=ln.package, description=ln.description, quantity=ln.quantity, unit=ln.unit,
                      basis=ln.basis, confidence=ln.confidence, deps=ln.deps, room_id=ln.room_id or "")
        for ln in tk.lines])


def _takeoff_from_pb(t: e.TakeoffResponse) -> Takeoff:
    return Takeoff(status=t.status or "ok", warnings=list(t.warnings), lines=[
        TakeoffLine(code=ln.code, package=ln.package, description=ln.description, quantity=ln.quantity, unit=ln.unit,
                    basis=ln.basis, confidence=ln.confidence or "assumed", deps=list(ln.deps),
                    room_id=ln.room_id or None) for ln in t.lines])


def _rate_from_pb(r: c.RateResult) -> RateResult:
    return RateResult(code=r.code, low=r.low if r.HasField("low") else None, base=r.base if r.HasField("base") else None,
                      high=r.high if r.HasField("high") else None, unit=r.unit, currency=r.currency or "VND",
                      confidence=r.confidence or "none", region=r.region, quality=r.quality, note=r.note,
                      sources=[RateSource(url=s.url, quote=s.quote, retrieved_at=s.retrieved_at) for s in r.sources])


def price_to_domain(request: e.PriceEstimateRequest) -> tuple[Takeoff, dict[str, RateResult], EstimateFacts]:
    return (_takeoff_from_pb(request.takeoff), {r.code: _rate_from_pb(r) for r in request.rates},
            facts_to_domain(request.facts))


def estimate_to_pb(est: Estimate) -> e.PriceEstimateResponse:
    return e.PriceEstimateResponse(estimate_json=dump_json(est), total_low=est.total.get("low", 0),
                                   total_base=est.total.get("base", 0), total_high=est.total.get("high", 0),
                                   warnings=est.warnings)
