"""What one message states about the estimate. A field is empty unless the person said it."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class RoomStateClaim:
    room_id: str
    state: str  # empty | existing | to_replace | new


@dataclass(frozen=True)
class Extracted:
    kind: str | None = None  # apartment | house
    work_type: str | None = None  # new_build | fit_out | renovation
    city: str | None = None
    district: str | None = None
    quality: str | None = None  # typical | normal | good | luxury
    floor_height: float | None = None  # metres
    storeys: int | None = None
    structure_in_scope: bool | None = None
    mep_in_scope: bool | None = None
    scope_default: str | None = None  # the condition of every room
    scope_rooms: list[RoomStateClaim] = field(default_factory=list)
    vat_percent: float | None = None
    overhead_percent: float | None = None
    profit_percent: float | None = None
    design_percent: float | None = None
    contingency_percent: float | None = None
    fixtures_existing: list[str] = field(default_factory=list)  # fixture ids said to be installed already
    fixtures_all_exist: bool = False
    fixtures_none_exist: bool = False
    exclude_items: list[str] = field(default_factory=list)  # rate codes to take out of the estimate
    include_items: list[str] = field(default_factory=list)  # rate codes to put back
    use_typical_markups: bool = False
    decide_for_me: list[str] = field(default_factory=list)  # slots the person gave up on
    explain: str | None = None  # a cost item the person asks about
