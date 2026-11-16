"""Where unit rates come from. Today an LLM researches them on the web (price_research.py); a price
database can replace it later by implementing the same protocol. No prices live in this code base."""

from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from .estimate_models import Quality, RateRequest, RateResult
from .takeoff import RATE_CATALOG

OnResult = Callable[[RateResult], None]

QUALITY_WORDS: dict[str, str] = {
    "typical": "economy basic grade",
    "normal": "standard mid-range grade",
    "good": "good quality premium grade",
    "luxury": "luxury high-end grade",
}


class PriceSource(Protocol):
    def rates(self, requests: list[RateRequest], on_result: OnResult | None = None) -> list[RateResult]: ...


def requests_for(codes: list[str], region: str, quality: Quality | str) -> list[RateRequest]:
    """The research requests for these rate codes. Only material, unit, region and quality go out:
    never tenant, project, user or plan data and never the user's own words."""
    out: list[RateRequest] = []
    for code in codes:
        spec = RATE_CATALOG[code]
        desc = spec.description
        if spec.quality_sensitive:
            desc = f"{desc}, {QUALITY_WORDS.get(str(quality), QUALITY_WORDS['normal'])}"
        out.append(RateRequest(code=code, description=desc, unit=spec.unit, region=region,
                               quality=str(quality) if spec.quality_sensitive else ""))
    return out


class NoPriceSource:
    """Used when no search provider is configured: every code comes back unpriced and the agent asks the user."""

    def rates(self, requests, on_result=None):
        out = []
        for r in requests:
            res = RateResult(code=r.code, unit=r.unit, region=r.region, quality=r.quality, confidence="none",
                             note="no search provider is configured (set ESTIMATE_SEARCH_PROVIDER)")
            if on_result:
                on_result(res)
            out.append(res)
        return out
