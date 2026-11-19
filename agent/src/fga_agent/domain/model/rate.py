"""A unit rate: what is asked of the research, and what it found."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class RateRequest:
    """Only material, unit, region and quality go out: never tenant, project, user or plan data, never the person's own words."""

    code: str
    description: str
    unit: str
    region: str
    quality: str  # "" when the rate does not depend on quality


@dataclass(frozen=True)
class RateSource:
    url: str
    quote: str
    retrieved_at: str  # YYYY-MM-DD


@dataclass(frozen=True)
class RateResult:
    code: str
    unit: str
    low: float | None = None
    base: float | None = None
    high: float | None = None
    currency: str = "VND"
    confidence: str = "none"  # ok | low_confidence | user | none
    region: str = ""
    quality: str = ""
    sources: list[RateSource] = field(default_factory=list)
    note: str = ""
