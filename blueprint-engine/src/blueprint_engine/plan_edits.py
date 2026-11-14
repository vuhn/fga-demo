"""Chat edits for an imported plan (no brief): operations that change the BuildingModel directly.

An imported plan is taken as drawn, so it is never regenerated. The model may only name rooms, openings
and fixtures that exist (resolved here, deterministically); the geometry is changed by this code, not by
the LLM. Moving walls or resizing rooms is not supported yet and is reported as `unsupported`.
"""

from __future__ import annotations

import math
import typing
from typing import Annotated, Literal

from pydantic import BaseModel, Field
from shapely.geometry import Point, Polygon, box

from . import prompts
from .edits import EditError, UnsupportedEdit, _norm
from .fixtures import SPECS
from .schemas import FIXTURE_KINDS, BuildingModel, FixtureKind, PlacedFixture, PlacedRoom, RoomType, Side, _Strict


class PlanRenameRoom(_Strict):
    """Change a room's name."""

    op: Literal["rename_room"]
    room: str
    name: str = Field(min_length=1, max_length=60)


class PlanSetRoomType(_Strict):
    """Change what a room is (bedroom, bathroom, ...)."""

    op: Literal["set_room_type"]
    room: str
    type: RoomType


class PlanAddFixture(_Strict):
    """Add a fixture to a room; it is placed against a free wall, or refused when there is no space."""

    op: Literal["add_fixture"]
    room: str
    fixture: FixtureKind


class PlanRemoveFixture(_Strict):
    """Remove one fixture of that kind from a room, only when the user asks to remove it."""

    op: Literal["remove_fixture"]
    room: str
    fixture: FixtureKind


class PlanResizeOpening(_Strict):
    """Set the width in metres of a door or window, by its id (d1, w2, ...)."""

    op: Literal["resize_opening"]
    opening: str
    width: float = Field(gt=0.4, le=4.0)


class PlanRemoveOpening(_Strict):
    """Remove a door or window by its id (an entrance cannot be removed)."""

    op: Literal["remove_opening"]
    opening: str


class PlanUnsupported(_Strict):
    """Anything else: moving or resizing rooms or walls, new rooms, other furniture, materials, extra storeys."""

    op: Literal["unsupported"]
    reason: str


PlanOp = Annotated[
    PlanRenameRoom | PlanSetRoomType | PlanAddFixture | PlanRemoveFixture | PlanResizeOpening | PlanRemoveOpening
    | PlanUnsupported,
    Field(discriminator="op"),
]


class PlanOps(_Strict):
    ops: list[PlanOp] = Field(min_length=1, max_length=10)


def ops_help() -> str:
    members = typing.get_args(typing.get_args(PlanOps.model_fields["ops"].annotation)[0])
    members = typing.get_args(members[0])
    lines = ["Operations (refer to rooms by their current name and openings by id):"]
    for cls in members:
        op = typing.get_args(cls.model_fields["op"].annotation)[0]
        fields = ", ".join(prompts._field_sig(n, f) for n, f in cls.model_fields.items() if n != "op")
        lines.append(f"- {op} {{{fields}}}: {(cls.__doc__ or '').strip()}")
    lines.append(f"Fixtures: {', '.join(FIXTURE_KINDS)}.")
    return "\n".join(lines)


def describe(op: BaseModel) -> str:
    d = op.model_dump(exclude_none=True)
    return d.pop("op").replace("_", " ") + " " + ", ".join(f"{k}={v}" for k, v in d.items())


def summary(model: BuildingModel) -> str:
    """The plan as the model sees it: names, types, fixtures and openings with their ids."""
    import json

    rooms = [{"name": r.name, "type": r.type, "area": round(r.net_area, 1),
              "fixtures": [f.kind for f in model.fixtures if f.room_id == r.id]} for r in model.rooms]
    names = {r.id: r.name for r in model.rooms}
    openings = [{"id": o.id, "kind": o.kind, "width": round(o.width, 2),
                 "rooms": [names.get(x, x) for x in o.rooms]} for o in model.openings]
    return json.dumps({"rooms": rooms, "openings": openings})


def resolve_room(model: BuildingModel, ref: str) -> PlacedRoom:
    key = _norm(ref)
    exact = [r for r in model.rooms if key in (_norm(r.id), _norm(r.name))]
    if len(exact) == 1:
        return exact[0]
    part = [r for r in model.rooms if key and (key in _norm(r.name) or _norm(r.name) in key)]
    if len(part) == 1:
        return part[0]
    by_type = [r for r in model.rooms if _norm(r.type) == key]
    if len(by_type) == 1:
        return by_type[0]
    known = ", ".join(r.name for r in model.rooms)
    if exact or part or by_type:
        raise EditError(f"'{ref}' matches more than one room; name it exactly ({known})")
    raise EditError(f"no room called '{ref}' (rooms: {known})")


_FACING: dict[Side, Side] = {"south": "north", "north": "south", "west": "east", "east": "west"}
STEP = 0.1


def _find_spot(model: BuildingModel, room: PlacedRoom, kind: str) -> PlacedFixture | None:
    """A free spot against a wall of this (possibly non-rectangular) room: the body stays inside, off door
    zones and other fixtures, and keeps its clear zone free in front. Deterministic, largest size first."""
    spec = SPECS[kind]
    poly = Polygon(room.polygon) if room.polygon else box(*room.net_rect)
    taken = [box(*f.rect) for f in model.fixtures if f.room_id == room.id]
    walls = {w.id: w for w in model.walls}
    doors = []
    for o in model.openings:
        if room.id in o.rooms and o.kind != "window" and o.wall_id in walls:
            r = o.width + 0.1 if o.swing_into == room.id else 0.7
            doors.append(Point(o.center).buffer(r))
    x0, y0, x1, y1 = poly.bounds
    inner = poly.buffer(0.01)
    for w, d in spec.sizes:
        for aw, ad in ((w, d), (d, w)):
            cands = []
            y = y0
            while y + ad <= y1 + 1e-6:
                x = x0
                while x + aw <= x1 + 1e-6:
                    cands.append(box(x, y, x + aw, y + ad))
                    x += STEP
                y += STEP
            for body in cands:
                if not inner.contains(body) or any(body.intersects(t.buffer(-0.01)) for t in taken):
                    continue
                if any(body.intersects(z) for z in doors):
                    continue
                gap = body.distance(poly.exterior)
                if gap > 0.06 and not spec.centre:
                    continue
                # face away from the nearest wall
                nx = min(((body.bounds[0] - x0), "west"), ((x1 - body.bounds[2]), "east"),
                         ((body.bounds[1] - y0), "south"), ((y1 - body.bounds[3]), "north"))[1]
                facing: Side = _FACING[nx]  # type: ignore[assignment]
                bx0, by0, bx1, by1 = body.bounds
                c = spec.clear
                front = {"north": box(bx0, by1, bx1, by1 + c), "south": box(bx0, by0 - c, bx1, by0),
                         "east": box(bx1, by0, bx1 + c, by1), "west": box(bx0 - c, by0, bx0, by1)}[facing]
                if c > 0 and (not inner.contains(front) or any(front.intersects(t.buffer(-0.01)) for t in taken)):
                    continue
                return PlacedFixture(id="", kind=kind, room_id=room.id,  # type: ignore[arg-type]
                                     rect=(round(bx0, 3), round(by0, 3), round(bx1, 3), round(by1, 3)), facing=facing)
    return None


def apply_plan_ops(model: BuildingModel, ops: PlanOps) -> BuildingModel:
    """Returns a new model; raises EditError on an unresolvable, refused or unsupported op."""
    data = model.model_copy(deep=True)
    for op in ops.ops:
        match op:
            case PlanUnsupported():
                raise UnsupportedEdit(f"not supported for an imported plan: {op.reason}")
            case PlanRenameRoom():
                resolve_room(data, op.room).name = op.name.strip()
            case PlanSetRoomType():
                resolve_room(data, op.room).type = op.type
            case PlanAddFixture():
                room = resolve_room(data, op.room)
                fx = _find_spot(data, room, op.fixture)
                if fx is None:
                    raise EditError(f"no free space for a {op.fixture} in {room.name}")
                n = 1 + sum(1 for f in data.fixtures if f.room_id == room.id and f.kind == op.fixture)
                fx.id = f"{room.id}-{op.fixture}-n{n}"
                data.fixtures.append(fx)
            case PlanRemoveFixture():
                room = resolve_room(data, op.room)
                hit = next((f for f in data.fixtures if f.room_id == room.id and f.kind == op.fixture), None)
                if hit is None:
                    raise EditError(f"{room.name} has no {op.fixture}")
                data.fixtures.remove(hit)
            case PlanResizeOpening() | PlanRemoveOpening():
                o = next((o for o in data.openings if _norm(o.id) == _norm(op.opening)), None)
                if o is None:
                    raise EditError(f"no opening '{op.opening}' (ids: {', '.join(x.id for x in data.openings)})")
                if isinstance(op, PlanRemoveOpening):
                    if o.kind == "entrance":
                        raise EditError("the entrance cannot be removed")
                    data.openings.remove(o)
                else:
                    w = next(w for w in data.walls if w.id == o.wall_id)
                    room_for = math.hypot(w.end[0] - w.start[0], w.end[1] - w.start[1]) - 0.3
                    if op.width > room_for:
                        raise EditError(f"{o.id} cannot be {op.width:g} m wide: its wall allows {room_for:.2f} m")
                    o.width = round(op.width, 3)
    return data
