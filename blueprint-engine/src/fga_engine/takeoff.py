"""Legacy entry points of the takeoff. The math lives in service/takeoff.py and works on EstimateFacts; these
adapters take the legacy session, convert it with `facts_of`, and keep the old signatures for the estimate agent."""

from __future__ import annotations

from .domain.estimate import Takeoff
from .domain.estimate_rules import pending_fixtures as _pending_fixtures
from .domain.estimate_rules import structure_in_scope as _structure_in_scope
from .domain.rate_catalog import PLUMBED, PRICED_FIXTURES, RATE_CATALOG, RateSpec  # noqa: F401  (re-exported)
from .domain.schemas import BuildingModel
from .estimate_models import SessionState, facts_of
from .service.takeoff import TakeoffService

_SERVICE = TakeoffService()


def pending_fixtures(model: BuildingModel, state: SessionState):
    return _pending_fixtures(model, facts_of(state))


def structure_in_scope(state: SessionState) -> bool:
    return _structure_in_scope(facts_of(state))


def takeoff(model: BuildingModel, state: SessionState) -> Takeoff:
    return _SERVICE.takeoff(model, facts_of(state))
