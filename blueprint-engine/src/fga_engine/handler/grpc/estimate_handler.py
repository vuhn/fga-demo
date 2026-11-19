"""Estimate endpoints (Takeoff, PriceEstimate). Decode, call one use case, encode through mapper/."""

from __future__ import annotations

from dataclasses import dataclass

from ...usecase.price_estimate import EstimatePricer
from ...usecase.takeoff import QuantityTakeoff
from .common.error_helper import handled
from .mapper import estimate_mapper


@dataclass(frozen=True)
class EstimateUsecases:
    takeoff: QuantityTakeoff
    estimate_pricer: EstimatePricer


class EstimateHandler:
    def __init__(self, usecases: EstimateUsecases) -> None:
        self._uc = usecases

    @handled
    def Takeoff(self, request, context):
        model, facts = estimate_mapper.takeoff_to_domain(request)
        return estimate_mapper.takeoff_to_pb(self._uc.takeoff.takeoff(model, facts))

    @handled
    def PriceEstimate(self, request, context):
        takeoff, rates, facts = estimate_mapper.price_to_domain(request)
        return estimate_mapper.estimate_to_pb(self._uc.estimate_pricer.price_estimate(takeoff, rates, facts))
