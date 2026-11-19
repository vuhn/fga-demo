from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class RateRange:
    low: float
    base: float
    high: float
    confidence: str  # ok | low_confidence | none
