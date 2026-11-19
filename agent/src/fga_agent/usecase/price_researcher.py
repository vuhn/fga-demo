from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress
from ..domain.model.rate import RateRequest, RateResult


class PriceResearcher(Protocol):
    """Finds the current supply-and-install unit rates of construction items on the web: the model writes the searches and
    reads the pages, code checks every price. No prices live in this code base."""

    def requests_for(self, ctx: CallContext, codes: list[str], region: str, quality: str) -> list[RateRequest]:
        """What to research for these rate codes (from the Engine's catalog). Only material, unit, region and quality.
        InvalidEdit for a code the catalog does not have; EngineUnavailable while the catalog cannot be loaded."""

    def research(self, ctx: CallContext, requests: list[RateRequest], on_progress: Callable[[Progress], None],
                 on_rate: Callable[[RateResult], None] | None = None) -> list[RateResult]:
        """One result per request, in order; `on_rate` hears each as it is known. A rate that could not be found is
        returned with confidence none and the reason in `note`, never raised. `on_progress` may raise Cancelled."""
