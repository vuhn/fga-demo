"""A unit rate <-> the Agent's DTO (`common.RateResult`, an event of the estimate stream) and the JSON the session keeps."""

from __future__ import annotations

from ....domain.model.rate import RateResult, RateSource
from fga.demo.common.v1 import common_pb2 as c


def to_pb(r: RateResult) -> c.RateResult:
    return c.RateResult(code=r.code, low=r.low, base=r.base, high=r.high, unit=r.unit, currency=r.currency, confidence=r.confidence,
                        region=r.region, quality=r.quality, note=r.note,
                        sources=[c.RateSource(url=s.url, quote=s.quote, retrieved_at=s.retrieved_at) for s in r.sources])


def to_domain(r: c.RateResult) -> RateResult:
    return RateResult(code=r.code, unit=r.unit, low=r.low if r.HasField("low") else None, base=r.base if r.HasField("base") else None,
                      high=r.high if r.HasField("high") else None, currency=r.currency or "VND", confidence=r.confidence or "none",
                      region=r.region, quality=r.quality, note=r.note,
                      sources=[RateSource(s.url, s.quote, s.retrieved_at) for s in r.sources])


def to_dict(r: RateResult) -> dict:
    return {"code": r.code, "low": r.low, "base": r.base, "high": r.high, "unit": r.unit, "currency": r.currency,
            "confidence": r.confidence, "region": r.region, "quality": r.quality, "note": r.note,
            "sources": [{"url": s.url, "quote": s.quote, "retrieved_at": s.retrieved_at} for s in r.sources]}


def from_dict(x: dict) -> RateResult:
    return RateResult(code=x["code"], unit=x.get("unit", ""), low=x.get("low"), base=x.get("base"), high=x.get("high"),
                      currency=x.get("currency", "VND"), confidence=x.get("confidence", "none"), region=x.get("region", ""),
                      quality=x.get("quality", ""), note=x.get("note", ""),
                      sources=[RateSource(s["url"], s["quote"], s.get("retrieved_at", "")) for s in x.get("sources", [])])
