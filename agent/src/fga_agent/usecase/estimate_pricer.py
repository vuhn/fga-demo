from __future__ import annotations

from collections.abc import Callable
from typing import Any, Protocol

from ..domain.model.call_context import CallContext
from ..domain.model.estimate import Estimate
from ..domain.model.estimate_facts import EstimateFacts
from ..domain.model.estimate_session import SessionState
from ..domain.model.takeoff import Takeoff


class EstimatePricer(Protocol):
    """Prices a takeoff: the person's own rates first, then rates researched earlier for this region and quality, then the
    web for the rest. Researched rates are kept in the session so an unchanged answer is never searched twice."""

    def price(self, ctx: CallContext, state: SessionState, facts: EstimateFacts, takeoff: Takeoff,
              on_event: Callable[[Any], None]) -> tuple[SessionState, Estimate]:
        """The session with the rates it learned, and the priced estimate. `on_event` hears progress and each rate as it
        is found (and may raise Cancelled). EngineUnavailable when the Engine cannot price."""
