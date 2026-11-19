"""Engine DTOs <-> domain for the takeoff: the request out, the measured lines in, and the lines back out for pricing."""

from __future__ import annotations

from ....domain.model.estimate_facts import EstimateFacts
from ....domain.model.takeoff import Takeoff, TakeoffLine
from fga.demo.engine.v1 import engine_pb2 as e
from .estimate_facts_mapper import facts_to_pb


def request(model: bytes, facts: EstimateFacts) -> e.TakeoffRequest:
    return e.TakeoffRequest(model_json=model, facts=facts_to_pb(facts))


def to_pb(t: Takeoff) -> e.TakeoffResponse:
    return e.TakeoffResponse(status=t.status, warnings=t.warnings, lines=[
        e.TakeoffLine(code=x.code, package=x.package, description=x.description, quantity=x.quantity, unit=x.unit, basis=x.basis,
                      confidence=x.confidence, deps=x.deps, room_id=x.room_id) for x in t.lines])


def to_domain(r: e.TakeoffResponse) -> Takeoff:
    return Takeoff(status=r.status or "ok", warnings=list(r.warnings), lines=[
        TakeoffLine(x.code, x.package, x.description, x.quantity, x.unit, x.basis, x.confidence or "assumed", list(x.deps), x.room_id)
        for x in r.lines])
