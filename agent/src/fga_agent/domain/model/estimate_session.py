"""The estimate conversation as the Agent keeps it between turns (opaque to the API and the Engine)."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class EstimateFact:
    value: str | float | bool
    source: str  # user | plan | assumed


@dataclass(frozen=True)
class RoomScope:
    state: str  # empty | existing | to_replace | new
    source: str


@dataclass(frozen=True)
class Message:
    role: str  # user | assistant
    content: str


@dataclass(frozen=True)
class Question:
    id: str  # the slot it asks for
    text: str
    options: list[str] = field(default_factory=list)
    allow_unknown: bool = True


@dataclass(frozen=True)
class FactChange:
    """One change a message made to the session: a fact, `scope.<room id>`, or `excluded`."""

    name: str
    value: str | float | bool
    source: str


@dataclass
class SessionState:
    facts: dict[str, EstimateFact] = field(default_factory=dict)
    scope: dict[str, RoomScope] = field(default_factory=dict)
    fixtures_existing: list[str] = field(default_factory=list)  # every other priced fixture is assumed NOT installed
    excluded: list[str] = field(default_factory=list)  # rate codes left out of the estimate
    rate_overrides: dict[str, float] = field(default_factory=dict)
    status: str = "interviewing"  # interviewing | researching | ready | failed
    pending: list[str] = field(default_factory=list)  # open slot names, the first is the question just asked
    fingerprint: str = ""  # of the inputs the stored estimate was computed from
    estimate: dict | None = None  # the Engine's priced estimate as returned; typed with the turn (T13b)
    turns: int = 0
