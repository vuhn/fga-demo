"""Legacy estimate contracts. The engine types moved to domain/estimate.py and are re-exported here; the session
types (SessionState, Message, Question) stay for the legacy estimate agent until T12 copies them to the Agent.
`facts_of` is the adapter from a session to the facts takeoff and pricing take."""

from __future__ import annotations

from typing import Literal

from pydantic import Field

from .domain.estimate import (  # noqa: F401  (re-exported)
    FACT_NAMES, Confidence, Estimate, EstimateFacts, Fact, Markup, PricedLine, Quality, RateRequest, RateResult, RateSource,
    RoomScope, RoomState, Source, Takeoff, TakeoffLine, WorkType, _Strict,
)

Status = Literal["interviewing", "researching", "ready", "failed"]


class Message(_Strict):
    role: Literal["user", "assistant"]
    content: str


class Question(_Strict):
    id: str  # the slot it asks for
    text: str
    options: list[str] = Field(default_factory=list)
    allow_unknown: bool = True


class SessionState(_Strict):
    facts: dict[str, Fact] = Field(default_factory=dict)
    scope: dict[str, RoomScope] = Field(default_factory=dict)
    # plan fixture ids the person says are already installed; every other priced fixture is assumed NOT there
    fixtures_existing: list[str] = Field(default_factory=list)
    # rate codes the person took out of the estimate ("the doors already exist"); the takeoff leaves them out
    excluded: list[str] = Field(default_factory=list)
    rate_overrides: dict[str, float] = Field(default_factory=dict)
    rates: list[RateResult] = Field(default_factory=list)  # researched rates (kept to re-price without searching)
    status: Status = "interviewing"
    pending: list[str] = Field(default_factory=list)  # open slot names, for display
    fingerprint: str = ""  # of the inputs the stored estimate was computed from
    estimate: Estimate | None = None
    turns: int = 0


def facts_of(state: SessionState) -> EstimateFacts:
    """The part of a session takeoff and pricing read."""
    return EstimateFacts(facts=state.facts, scope=state.scope, fixtures_existing=state.fixtures_existing, excluded=state.excluded)
