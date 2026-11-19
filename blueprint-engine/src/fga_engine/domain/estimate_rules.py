"""Pure rules over the estimate facts that takeoff, pricing and the plan summary share."""

from __future__ import annotations

from .estimate import EstimateFacts
from .rate_catalog import PRICED_FIXTURES
from .schemas import BuildingModel


def fact_num(facts: EstimateFacts, name: str, default: float) -> float:
    f = facts.facts.get(name)
    try:
        return float(f.value) if f is not None else default
    except (TypeError, ValueError):
        return default


def fact_bool(facts: EstimateFacts, name: str, default: bool) -> bool:
    f = facts.facts.get(name)
    return bool(f.value) if f is not None else default


def structure_in_scope(facts: EstimateFacts) -> bool:
    wt = facts.facts.get("work_type")
    if wt is not None and wt.value == "new_build":
        return True
    return fact_bool(facts, "structure_in_scope", False)


def pending_fixtures(model: BuildingModel, facts: EstimateFacts):
    """Priced fixtures the plan shows that are not in a room already finished and not said to exist. A floor plan
    shows where things go, not what is installed, so each is assumed absent until the person says it exists."""
    active = {r.id for r in model.rooms if (facts.scope[r.id].state if r.id in facts.scope else "empty") != "existing"}
    return [f for f in model.fixtures if f.kind in PRICED_FIXTURES and f.room_id in active and f.id not in facts.fixtures_existing]
