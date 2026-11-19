"""Engine DTOs <-> domain for the priced estimate: the pricing request out, the estimate document in."""

from __future__ import annotations

import json

from ....domain.model.estimate import Estimate, EstimateLine
from ....domain.model.estimate_facts import EstimateFacts
from ....domain.model.rate import RateResult, RateSource
from ....domain.model.takeoff import Takeoff
from fga.demo.engine.v1 import engine_pb2 as e
from . import rate_mapper, takeoff_mapper
from .estimate_facts_mapper import facts_to_pb


def request(takeoff: Takeoff, rates: list[RateResult], facts: EstimateFacts) -> e.PriceEstimateRequest:
    return e.PriceEstimateRequest(takeoff=takeoff_mapper.to_pb(takeoff), rates=[rate_mapper.to_pb(r) for r in rates],
                                  facts=facts_to_pb(facts))


def _line(x: dict) -> EstimateLine:
    return EstimateLine(code=x["code"], package=x.get("package", ""), description=x.get("description", ""), quantity=x.get("quantity", 0),
                        unit=x.get("unit", ""), basis=x.get("basis", ""), confidence=x.get("confidence", "assumed"),
                        priced=x.get("priced", True), rate_base=x.get("rate_base"), rate_confidence=x.get("rate_confidence", "none"),
                        low=x.get("low", 0), base=x.get("base", 0), high=x.get("high", 0))


def _rate(x: dict) -> RateResult:
    return RateResult(code=x["code"], unit=x.get("unit", ""), low=x.get("low"), base=x.get("base"), high=x.get("high"),
                      currency=x.get("currency", "VND"), confidence=x.get("confidence", "none"), region=x.get("region", ""),
                      quality=x.get("quality", ""), note=x.get("note", ""),
                      sources=[RateSource(s["url"], s["quote"], s.get("retrieved_at", "")) for s in x.get("sources", [])])


def to_domain(r: e.PriceEstimateResponse) -> Estimate:
    doc = json.loads(r.estimate_json)
    return Estimate(total=doc.get("total", {}), packages=doc.get("packages", {}), assumptions=doc.get("assumptions", []),
                    warnings=doc.get("warnings", []), lines=[_line(x) for x in doc.get("lines", [])],
                    rates=[_rate(x) for x in doc.get("rates", [])], document=doc)
