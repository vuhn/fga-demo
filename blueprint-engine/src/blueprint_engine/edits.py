"""Apply edit operations to a Brief (the only editable source of truth).

Room references from the LLM are names as the user said them ("the kitchen",
"bedroom 2"); they are resolved here, deterministically.
"""

from __future__ import annotations

import difflib
import re

from pydantic import ValidationError

from .schemas import (
    MAX_NESTED,
    NESTABLE_TYPES,
    AddFixture,
    AddRoom,
    RemoveFixture,
    SetInside,
    Brief,
    EditOps,
    RemoveRoom,
    RenameRoom,
    ResizeRoom,
    Room,
    SetAdjacent,
    SetEntrance,
    SetLot,
    SetOpenPlan,
    SetSide,
    Unsupported,
)


class EditError(ValueError):
    pass


class UnsupportedEdit(EditError):
    """The request is outside what the ops can express; retrying the LLM will not help."""


def _norm(s: str) -> str:
    s = s.lower().strip()
    s = re.sub(r"^(the|a|an)\s+", "", s)
    return re.sub(r"[^a-z0-9]+", " ", s).strip()


def resolve_room(brief: Brief, ref: str) -> Room:
    key = _norm(ref)
    for r in brief.rooms:
        if key in (_norm(r.id), _norm(r.name)):
            return r
    by_type = [r for r in brief.rooms if _norm(r.type) == key or _norm(r.type) + "s" == key]
    if len(by_type) == 1:
        return by_type[0]
    names = {_norm(r.name): r for r in brief.rooms} | {_norm(r.id): r for r in brief.rooms}
    close = difflib.get_close_matches(key, list(names), n=2, cutoff=0.75)
    if len(close) == 1 or (close and names[close[0]] is names[close[-1]]):
        return names[close[0]]
    known = ", ".join(r.name for r in brief.rooms)
    if key in ("hall", "hallway", "corridor", "passage"):
        raise EditError(f"the hall is not a room of the brief (the layout adds it), so it cannot be removed or "
                        f"edited; to remove the walls around it use set_open_plan {{\"open\": true}}; rooms are: {known}")
    raise EditError(f"no single room matches {ref!r}; rooms are: {known}")


def _new_id(brief: Brief, name: str) -> str:
    base = re.sub(r"[^a-z0-9]+", "_", name.lower()).strip("_")[:30] or "room"
    ids = {r.id for r in brief.rooms}
    rid, n = base, 2
    while rid in ids:
        rid, n = f"{base}_{n}", n + 1
    return rid


def _nest(data: Brief, room: Room, host_ref: str | None) -> None:
    """Put a room inside another (ensuite, walk-in closet), or back onto the hall with None."""
    if host_ref is None:
        room.parent = None
        return
    host = resolve_room(data, host_ref)
    if host.id == room.id:
        raise EditError("a room cannot be inside itself")
    if room.type not in NESTABLE_TYPES:
        raise EditError(f"only a bathroom, storage or laundry can go inside another room, not a {room.type}")
    if host.parent is not None:
        raise EditError(f"{host.name} is itself inside {data.room(host.parent).name}; nest one level only")
    if data.children(room.id):
        raise EditError(f"{room.name} holds other rooms and cannot go inside {host.name}")
    if len([c for c in data.children(host.id) if c.id != room.id]) >= MAX_NESTED:
        raise EditError(f"{host.name} already holds {MAX_NESTED} rooms")
    room.parent = host.id
    room.side = None


def apply_ops(brief: Brief, ops: EditOps) -> Brief:
    """Returns a new Brief; raises EditError on unresolvable or unsupported ops."""
    data = brief.model_copy(deep=True)
    for op in ops.ops:
        match op:
            case Unsupported():
                raise UnsupportedEdit(f"not supported in this demo: {op.reason}")
            case AddRoom():
                room = Room(id=_new_id(data, op.name), name=op.name, type=op.type,
                            target_area=op.target_area, side=op.side)
                data.rooms.append(room)
                if op.inside:
                    _nest(data, room, op.inside)
            case RemoveRoom():
                room = resolve_room(data, op.room)
                data.rooms = [r for r in data.rooms if r.id != room.id]
                for r in data.rooms:
                    r.adjacent_to = [a for a in r.adjacent_to if a != room.id]
                    if r.parent == room.id:  # its ensuite / closet now opens onto the hall
                        r.parent = None
                if not data.rooms:
                    raise EditError("cannot remove the last room")
            case ResizeRoom():
                resolve_room(data, op.room).target_area = op.target_area
            case SetSide():
                room = resolve_room(data, op.room)
                if room.parent is not None and op.side is not None:
                    raise EditError(f"{room.name} is inside {data.room(room.parent).name}; it follows that room's wall")
                room.side = op.side
            case SetInside():
                _nest(data, resolve_room(data, op.room), op.inside)
            case AddFixture():
                room = resolve_room(data, op.room)
                if op.fixture in room.fixtures:  # models re-list type defaults; adding is idempotent
                    continue
                if len(room.fixtures) >= 8:
                    raise EditError(f"{room.name} already has 8 fixtures")
                room.fixtures.append(op.fixture)
            case RemoveFixture():
                room = resolve_room(data, op.room)
                if op.fixture not in room.fixtures:
                    raise EditError(f"{room.name} has no {op.fixture.replace('_', ' ')}")
                room.fixtures.remove(op.fixture)
            case SetAdjacent():
                a, b = resolve_room(data, op.room), resolve_room(data, op.other)
                if a.id == b.id:
                    raise EditError("a room cannot be adjacent to itself")
                if b.id not in a.adjacent_to:
                    a.adjacent_to.append(b.id)
            case RenameRoom():
                resolve_room(data, op.room).name = op.name
            case SetLot():
                data.lot.width, data.lot.depth = op.width, op.depth
            case SetEntrance():
                data.entrance_side = op.side
            case SetOpenPlan():
                data.open_plan = op.open
    # re-validate the whole brief (ids, references, limits)
    try:
        return Brief.model_validate(data.model_dump())
    except ValidationError as e:
        raise EditError("; ".join(err["msg"] for err in e.errors())[:600]) from e
