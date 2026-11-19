"""A plan as the estimate interview reads it: rooms, fixtures and openings with their ids, and what is still open."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class SummaryRoom:
    id: str
    name: str
    type: str
    area_m2: float


@dataclass(frozen=True)
class SummaryFixture:
    id: str
    kind: str
    room_id: str


@dataclass(frozen=True)
class SummaryOpening:
    id: str
    kind: str  # door | window | open
    room_ids: list[str]
    width: float


@dataclass(frozen=True)
class PlanSummary:
    rooms: list[SummaryRoom] = field(default_factory=list)
    fixtures: list[SummaryFixture] = field(default_factory=list)
    openings: list[SummaryOpening] = field(default_factory=list)
    pending_fixtures: list[str] = field(default_factory=list)  # ids of priced fixtures whose installed state is unknown
    structure_in_scope: bool = False  # given the facts the summary was asked with
