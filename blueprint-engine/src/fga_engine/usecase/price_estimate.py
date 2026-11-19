from __future__ import annotations

from typing import Protocol

from ..domain.estimate import Estimate, EstimateFacts, RateResult, Takeoff


class EstimatePricer(Protocol):
    def price_estimate(self, tk: Takeoff, rates: dict[str, RateResult], facts: EstimateFacts) -> Estimate:
        """Takeoff lines x rates, then the markup cascade; a line with no rate is unpriced, never zero."""
