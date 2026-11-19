"""Engine DTO for a unit rate (`common.RateResult`), both directions."""

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
