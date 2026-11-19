"""The brief draft an LLM produces and its deterministic clean-up (ids, references, minimums, halls)."""

from __future__ import annotations

import re

from pydantic import BaseModel, Field

from .edits import EditError, resolve_room
from .schemas import (DEFAULT_FIXTURES, MAX_NESTED, MIN_AREA, NESTABLE_TYPES, Brief, FixtureKind, Lot, Room,
                      RoomType, Side)

_HALL = re.compile(r"\b(hall|hallway|corridor|passage)\b", re.I)


class DraftRoom(BaseModel):
    name: str
    type: RoomType
    target_area: float = Field(gt=0, le=200)
    side: Side | None = None
    adjacent_to: list[str] = Field(default_factory=list, description="names of other rooms")
    inside: str | None = Field(default=None, description="name of the room this one is inside (ensuite, walk-in closet)")
    fixtures: list[FixtureKind] = Field(default_factory=list, description="extra fixtures the user asked for")


class DraftBrief(BaseModel):
    title: str
    lot_width: float = Field(gt=4, le=200)
    lot_depth: float = Field(gt=4, le=200)
    entrance_side: Side
    rooms: list[DraftRoom] = Field(min_length=1, max_length=16)
    open_plan: bool = Field(default=False, description="true only when the user asks for open-plan living "
                                                       "(no walls between hall, living, kitchen and dining)")


def normalize_draft(draft: DraftBrief) -> tuple[Brief, list[str]]:
    """Deterministic clean-up of model output: ids, references, minimum areas, corridors."""
    notes: list[str] = []
    rooms: list[Room] = []
    pairs: list[tuple[DraftRoom, Room]] = []
    used: set[str] = set()
    for d in draft.rooms:
        if _HALL.search(d.name):
            notes.append(f"dropped '{d.name}': the layout adds the hall itself")
            continue
        base = re.sub(r"[^a-z0-9]+", "_", d.name.lower()).strip("_")[:30] or d.type
        rid, n = base, 2
        while rid in used:
            rid, n = f"{base}_{n}", n + 1
        used.add(rid)
        area = d.target_area
        if area < MIN_AREA[d.type]:
            notes.append(f"raised {d.name} from {area:g} to the {d.type} minimum of {MIN_AREA[d.type]:g} m²")
            area = MIN_AREA[d.type]
        name = d.name.strip()[:56] or d.type.title()
        if "_" in name or name.islower():
            name = name.replace("_", " ").capitalize()
        taken = {r.name.lower() for r in rooms}
        label, k = name, 2
        while label.lower() in taken:
            label, k = f"{name} {k}", k + 1
        fixtures = list(DEFAULT_FIXTURES[d.type])
        fixtures += [f for f in d.fixtures if f not in fixtures]
        room = Room(id=rid, name=label, type=d.type, target_area=area, side=d.side, fixtures=fixtures[:8])
        rooms.append(room)
        pairs.append((d, room))
    if not rooms:
        raise EditError("the description has no rooms besides halls")
    brief = Brief(
        title=draft.title[:80] or "Single-storey house",
        lot=Lot(width=draft.lot_width, depth=draft.lot_depth),
        entrance_side=draft.entrance_side,
        rooms=rooms,
        open_plan=draft.open_plan,
    )
    for d, room in pairs:
        for ref in d.adjacent_to:
            try:
                other = resolve_room(brief, ref)
            except EditError:
                notes.append(f"ignored adjacency {room.name} - {ref!r} (no such room)")
                continue
            if other.id != room.id and other.id not in room.adjacent_to:
                room.adjacent_to.append(other.id)
    for d, room in pairs:
        if not d.inside:
            continue
        try:
            host = resolve_room(brief, d.inside)
        except EditError:
            notes.append(f"{room.name} is not inside {d.inside!r} (no such room)")
            continue
        if room.type not in NESTABLE_TYPES or host.id == room.id or host.parent is not None \
                or len(brief.children(host.id)) >= MAX_NESTED:
            notes.append(f"{room.name} opens onto the hall; it cannot be inside {host.name}")
            continue
        room.parent, room.side = host.id, None
    for room in brief.rooms:  # a parent that ended up nested itself: un-nest its children
        if room.parent and brief.room(room.parent).parent:
            room.parent = None
    return Brief.model_validate(brief.model_dump()), notes
