"""What takeoff and pricing read from an interview: the facts with their sources, the rooms' condition, what exists."""

from __future__ import annotations

from dataclasses import dataclass, field

from .estimate_session import EstimateFact, RoomScope


@dataclass(frozen=True)
class EstimateFacts:
    facts: dict[str, EstimateFact] = field(default_factory=dict)
    scope: dict[str, RoomScope] = field(default_factory=dict)
    fixtures_existing: list[str] = field(default_factory=list)  # plan fixture ids said to be installed already
    excluded: list[str] = field(default_factory=list)  # rate codes the person took out of the estimate
