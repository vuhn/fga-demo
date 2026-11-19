"""Legacy entry point of the pricing. The math lives in service/price_estimate.py and works on EstimateFacts."""

from __future__ import annotations

from .domain.estimate import Estimate, RateResult, Takeoff
from .domain.rate_catalog import DEFAULT_MARKUPS  # noqa: F401  (re-exported)
from .estimate_models import SessionState, facts_of
from .service.price_estimate import PriceEstimateService

_SERVICE = PriceEstimateService()


def price(tk: Takeoff, rates: dict[str, RateResult], state: SessionState) -> Estimate:
    return _SERVICE.price_estimate(tk, rates, facts_of(state))
