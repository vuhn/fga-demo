"""Data contracts.

The Brief is the only editable source of truth. The BuildingModel is always
derived from it by deterministic code (layout -> building). Units are metres;
the world origin is the lot's bottom-left corner and +y is north.
"""

from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

Side = Literal["north", "east", "south", "west"]
RoomType = Literal[
    "entry",
    "living",
    "kitchen",
    "dining",
    "study",
    "laundry",
    "storage",
    "bedroom",
    "bathroom",
    "corridor",
]

PUBLIC_TYPES: frozenset[str] = frozenset({"entry", "living", "kitchen", "dining", "study", "laundry", "storage"})
PRIVATE_TYPES: frozenset[str] = frozenset({"bedroom", "bathroom"})

# Code-level rule tables (a dozen hard-coded rules, as the proposal says).
MIN_AREA: dict[str, float] = {
    "entry": 2.0, "living": 12.0, "kitchen": 6.0, "dining": 7.0, "study": 6.0,
    "laundry": 2.5, "storage": 1.5, "bedroom": 9.0, "bathroom": 3.0, "corridor": 0.0,
}
MIN_WIDTH: dict[str, float] = {
    "entry": 1.2, "living": 3.0, "kitchen": 2.1, "dining": 2.4, "study": 2.1,
    "laundry": 1.5, "storage": 1.0, "bedroom": 2.7, "bathroom": 1.5, "corridor": 0.9,
}
NEEDS_WINDOW: frozenset[str] = frozenset({"living", "kitchen", "dining", "study", "bedroom"})

FixtureKind = Literal[
    "bed", "wardrobe", "desk", "shelves", "sofa", "tv_unit", "dining_table", "kitchen_counter",
    "sink", "stove", "fridge", "toilet", "basin", "shower", "bathtub", "washer",
]
FIXTURE_KINDS: tuple[str, ...] = (
    "bed", "wardrobe", "desk", "shelves", "sofa", "tv_unit", "dining_table", "kitchen_counter",
    "sink", "stove", "fridge", "toilet", "basin", "shower", "bathtub", "washer",
)
DEFAULT_FIXTURES: dict[str, list[str]] = {
    "entry": [], "living": ["sofa", "tv_unit"], "kitchen": ["kitchen_counter", "sink", "stove", "fridge"],
    "dining": ["dining_table"], "study": ["desk", "shelves"], "laundry": ["washer", "sink"],
    "storage": ["shelves"], "bedroom": ["bed", "wardrobe"], "bathroom": ["toilet", "basin", "shower"],
    "corridor": [],
}
# Rooms that may sit inside another room (ensuite, walk-in closet, pantry).
NESTABLE_TYPES: frozenset[str] = frozenset({"bathroom", "storage", "laundry"})
# with Brief.open_plan, rooms of these types (and the hall) share space without walls
OPEN_PLAN_TYPES: frozenset[str] = frozenset({"entry", "living", "kitchen", "dining"})
MAX_NESTED = 2

EXTERIOR_WALL = 0.30
INTERIOR_WALL = 0.12
CORRIDOR_WIDTH = 1.2
GRID = 0.1


class _Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Room(_Strict):
    id: str = Field(min_length=1, max_length=40, pattern=r"^[a-z0-9_]+$")
    name: str = Field(min_length=1, max_length=60)
    type: RoomType
    target_area: float = Field(gt=0, le=200, description="net floor area in m²")
    min_width: float | None = Field(default=None, gt=0, le=20)
    side: Side | None = Field(default=None, description="exterior wall this room must touch")
    adjacent_to: list[str] = Field(default_factory=list, description="room ids that should share a wall")
    needs_window: bool | None = None
    parent: str | None = Field(default=None, description="id of the room this one is inside (ensuite, walk-in closet)")
    fixtures: list[FixtureKind] = Field(default_factory=list, max_length=8,
                                        description="basic housing components; omitted = the type's defaults")

    @model_validator(mode="before")
    @classmethod
    def _default_fixtures(cls, data):
        if isinstance(data, dict) and "fixtures" not in data and data.get("type") in DEFAULT_FIXTURES:
            data = {**data, "fixtures": list(DEFAULT_FIXTURES[data["type"]])}
        return data

    def effective_min_width(self) -> float:
        return self.min_width if self.min_width is not None else MIN_WIDTH[self.type]

    def effective_needs_window(self) -> bool:
        return self.needs_window if self.needs_window is not None else self.type in NEEDS_WINDOW


class Setbacks(_Strict):
    front: float = Field(default=3.0, ge=0, le=50)
    back: float = Field(default=3.0, ge=0, le=50)
    sides: float = Field(default=1.5, ge=0, le=50)


class Lot(_Strict):
    width: float = Field(gt=4, le=200, description="east-west size in m")
    depth: float = Field(gt=4, le=200, description="north-south size in m")
    setbacks: Setbacks = Field(default_factory=Setbacks)


class Brief(_Strict):
    title: str = Field(default="Single-storey house", max_length=80)
    lot: Lot
    entrance_side: Side = "south"
    rooms: list[Room] = Field(min_length=1, max_length=16)
    # open plan: no walls between the hall, entry, living, kitchen and dining (OPEN_PLAN_TYPES)
    open_plan: bool = False

    @model_validator(mode="after")
    def _ids_and_refs(self) -> Brief:
        ids = [r.id for r in self.rooms]
        if len(ids) != len(set(ids)):
            raise ValueError("room ids must be unique")
        if any(r.type == "corridor" for r in self.rooms):
            raise ValueError("do not list a corridor; the layout adds the hall itself")
        known = set(ids)
        for r in self.rooms:
            unknown = [a for a in r.adjacent_to if a not in known or a == r.id]
            if unknown:
                raise ValueError(f"room {r.id!r} has unknown adjacent_to ids {unknown}")
        by_id = {r.id: r for r in self.rooms}
        for r in self.rooms:
            if r.parent is None:
                continue
            host = by_id.get(r.parent)
            if host is None or host.id == r.id:
                raise ValueError(f"room {r.id!r} has unknown parent {r.parent!r}")
            if host.parent is not None:
                raise ValueError(f"room {r.id!r} is inside {host.id!r}, which is itself inside a room; one level only")
            if r.type not in NESTABLE_TYPES:
                raise ValueError(f"a {r.type} cannot be inside another room; only bathroom, storage or laundry")
            if r.side is not None:
                raise ValueError(f"room {r.id!r} is inside {host.id!r} and cannot also ask for a wall side")
        for r in self.rooms:
            if len(self.children(r.id)) > MAX_NESTED:
                raise ValueError(f"room {r.id!r} holds more than {MAX_NESTED} rooms")
        return self

    def children(self, room_id: str) -> list[Room]:
        return [r for r in self.rooms if r.parent == room_id]

    def top_level(self) -> list[Room]:
        return [r for r in self.rooms if r.parent is None]

    def room(self, room_id: str) -> Room:
        for r in self.rooms:
            if r.id == room_id:
                return r
        raise KeyError(room_id)


# ---------- derived building model ----------

Point = tuple[float, float]


class PlacedRoom(_Strict):
    id: str
    name: str
    type: RoomType
    rect: tuple[float, float, float, float]  # x0, y0, x1, y1 on wall centerlines (world)
    net_rect: tuple[float, float, float, float]
    net_area: float
    target_area: float
    exterior_sides: list[Side]


class Wall(_Strict):
    id: str
    start: Point
    end: Point
    thickness: float
    kind: Literal["exterior", "interior"]


class Opening(_Strict):
    id: str
    kind: Literal["door", "window", "entrance", "open"]  # open: a wall-less passage (open plan)
    wall_id: str
    center: Point
    width: float
    rooms: list[str]  # rooms it connects; "outside" for exterior side
    swing_into: str | None = None  # room id the door leaf swings into


class PlacedFixture(_Strict):
    id: str
    kind: FixtureKind
    room_id: str
    rect: tuple[float, float, float, float]  # world, axis-aligned footprint
    facing: Side  # direction the fixture's front faces (away from the wall it stands on)


class BuildingModel(_Strict):
    footprint: tuple[float, float, float, float]
    lot: tuple[float, float]
    entrance_side: Side
    rooms: list[PlacedRoom]
    walls: list[Wall]
    openings: list[Opening]
    fixtures: list[PlacedFixture] = Field(default_factory=list)
    unplaced_fixtures: list[tuple[str, str]] = Field(default_factory=list)  # (room_id, kind) that did not fit

    def room(self, room_id: str) -> PlacedRoom:
        for r in self.rooms:
            if r.id == room_id:
                return r
        raise KeyError(room_id)


class CheckResult(_Strict):
    """One rule result. Failed results are the 'issues' (our BCF): a stable id, the elements
    involved and a hint naming an edit op, so a person or the reviewer agent can act on it."""

    rule: str
    severity: Literal["error", "warning"]
    ok: bool
    message: str
    room_id: str | None = None
    category: Literal["geometry", "collision", "information", "program", "design"] = "program"
    issue_id: str = ""  # rule:room[:n], stable across regenerations of the same brief
    elements: list[str] = Field(default_factory=list)  # room, opening or fixture ids
    hint: str = ""  # suggested edit op(s)


# ---------- edit operations (chat revisions; LLM output schema) ----------


class AddRoom(_Strict):
    """Add a room. With inside, it goes inside that room (only bathroom, storage, laundry), e.g. an ensuite in the master bedroom."""

    op: Literal["add_room"]
    name: str
    type: RoomType
    target_area: float = Field(gt=0, le=200)
    side: Side | None = None
    inside: str | None = Field(default=None, description="name of the room it goes inside (ensuite, walk-in closet)")


class SetInside(_Strict):
    """Put an existing bathroom, storage or laundry inside a room; inside null puts it back on the hall."""

    op: Literal["set_inside"]
    room: str
    inside: str | None  # null takes it out to the hall


class AddFixture(_Strict):
    """Add a fixture to a room (adding one the room already has does nothing)."""

    op: Literal["add_fixture"]
    room: str
    fixture: FixtureKind


class RemoveFixture(_Strict):
    """Remove a fixture, only when the user asks to remove it."""

    op: Literal["remove_fixture"]
    room: str
    fixture: FixtureKind


class RemoveRoom(_Strict):
    """Remove a room."""

    op: Literal["remove_room"]
    room: str


class ResizeRoom(_Strict):
    """Set a room's target area in m² (bigger/smaller without a number = +/-20%)."""

    op: Literal["resize_room"]
    room: str
    target_area: float = Field(gt=0, le=200)


class SetSide(_Strict):
    """Make a room touch an outside wall ("move the kitchen to the south wall"); null frees it."""

    op: Literal["set_side"]
    room: str
    side: Side | None


class SetAdjacent(_Strict):
    """Ask for two rooms to share a wall."""

    op: Literal["set_adjacent"]
    room: str
    other: str


class RenameRoom(_Strict):
    """Rename a room."""

    op: Literal["rename_room"]
    room: str
    name: str


class SetLot(_Strict):
    """Change the lot size in metres (width east-west, depth north-south)."""

    op: Literal["set_lot"]
    width: float = Field(gt=4, le=200)
    depth: float = Field(gt=4, le=200)


class SetEntrance(_Strict):
    """Move the street entrance to another side."""

    op: Literal["set_entrance"]
    side: Side


class SetOpenPlan(_Strict):
    """Open plan: true removes the walls between the hall, entry, living, kitchen and dining ("open-plan living", "remove the hall partitions"); false puts them back. The hall itself always stays."""

    op: Literal["set_open_plan"]
    open: bool


class Unsupported(_Strict):
    """Anything the other ops cannot express: other furniture, extra storeys, materials, curved walls."""

    op: Literal["unsupported"]
    reason: str


EditOp = Annotated[
    AddRoom | RemoveRoom | ResizeRoom | SetSide | SetAdjacent | RenameRoom | SetLot | SetEntrance
    | SetInside | AddFixture | RemoveFixture | SetOpenPlan | Unsupported,
    Field(discriminator="op"),
]


class EditOps(_Strict):
    ops: list[EditOp] = Field(min_length=1, max_length=10)
