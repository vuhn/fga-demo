"""A priced estimate as the Engine returns it. Money is VND."""

from __future__ import annotations

from dataclasses import dataclass, field

from .rate import RateResult


@dataclass(frozen=True)
class EstimateLine:
    code: str
    package: str
    description: str
    quantity: float
    unit: str
    basis: str
    confidence: str  # measured | derived | assumed
    priced: bool
    rate_base: float | None
    rate_confidence: str
    low: float
    base: float
    high: float


@dataclass(frozen=True)
class Estimate:
    total: dict[str, float]  # low / base / high, VAT included
    packages: dict[str, dict[str, float]]  # package -> low / base / high
    assumptions: list[str]
    warnings: list[str]
    lines: list[EstimateLine]
    rates: list[RateResult] = field(default_factory=list)  # every rate used, with its sources
    document: dict = field(default_factory=dict)  # the whole estimate as the Engine wrote it: shown and stored unchanged
