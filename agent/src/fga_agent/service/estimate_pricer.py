from __future__ import annotations

import copy
from collections.abc import Callable
from typing import Any

from ..domain.model.call_context import CallContext
from ..domain.model.estimate import Estimate
from ..domain.model.estimate_facts import EstimateFacts
from ..domain.model.estimate_session import SessionState
from ..domain.model.progress import Progress
from ..domain.model.rate import RateResult, RateSource
from ..domain.model.takeoff import Takeoff
from ..domain.gateway import BlueprintEngine
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.price_researcher import PriceResearcher

MAX_RATES = 40  # a plan never needs more; the rest would be left unpriced rather than searched for minutes


class EstimatePricerService:
    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, researcher: PriceResearcher) -> None:
        self._engine, self._catalog, self._researcher = engine, catalog, researcher

    def price(self, ctx: CallContext, state: SessionState, facts: EstimateFacts, takeoff: Takeoff,
              on_event: Callable[[Any], None]) -> tuple[SessionState, Estimate]:
        state = copy.deepcopy(state)
        region, quality = str(state.facts["city"].value), str(state.facts["quality"].value)
        specs = {s.code: s for s in self._catalog.limits(ctx).rate_catalog}
        rates: dict[str, RateResult] = {}
        todo: list[str] = []
        for code in dict.fromkeys(line.code for line in takeoff.lines):  # each code once, in takeoff order
            if code in state.rate_overrides:
                rates[code] = self._entered(code, state.rate_overrides[code], specs[code].unit)
            elif (have := self._known(state, code, region, quality if specs[code].quality_sensitive else "")) is not None:
                rates[code] = have
            else:
                todo.append(code)
        if todo:
            self._look_up(ctx, state, todo[:MAX_RATES], region, quality, rates, on_event)
        on_event(Progress("step", "compute", "computing quantities and totals"))
        return state, self._engine.price_estimate(ctx, takeoff, list(rates.values()), facts)

    def _look_up(self, ctx: CallContext, state: SessionState, codes: list[str], region: str, quality: str,
                 rates: dict[str, RateResult], on_event: Callable[[Any], None]) -> None:
        state.status = "researching"
        on_event(Progress("step", "research", f"looking up {len(codes)} unit price(s) in {region}"))
        requests = self._researcher.requests_for(ctx, codes, region, quality)
        for res in self._researcher.research(ctx, requests, on_event, on_event):
            rates[res.code] = res
            state.rates = [x for x in state.rates if x.code != res.code] + [res]

    @staticmethod
    def _known(state: SessionState, code: str, region: str, quality: str) -> RateResult | None:
        return next((r for r in state.rates if r.code == code and r.region == region and r.quality == quality
                     and r.confidence != "none"), None)

    @staticmethod
    def _entered(code: str, value: float, unit: str) -> RateResult:
        return RateResult(code=code, unit=unit, low=value, base=value, high=value, confidence="user",
                          sources=[RateSource(url="", quote="entered by the user", retrieved_at="")], note="entered by the user")
