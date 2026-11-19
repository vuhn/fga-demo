"""Layout -> BuildingModel: walls (with thickness), doors, windows, net areas."""

from __future__ import annotations

from .fixtures import place_fixtures
from .layout import Layout, local_side, world_side
from .schemas import (
    EXTERIOR_WALL,
    INTERIOR_WALL,
    OPEN_PLAN_TYPES,
    Brief,
    BuildingModel,
    Opening,
    PlacedRoom,
    Side,
    Wall,
)

Rect = tuple[float, float, float, float]
EPS = 1e-6

HALL = "hall"


def _edges(r: Rect) -> dict[Side, tuple[tuple[float, float], tuple[float, float]]]:
    x0, y0, x1, y1 = r
    return {
        "south": ((x0, y0), (x1, y0)),
        "north": ((x0, y1), (x1, y1)),
        "west": ((x0, y0), (x0, y1)),
        "east": ((x1, y0), (x1, y1)),
    }


def _exterior_sides(r: Rect, fp: Rect) -> list[Side]:
    out: list[Side] = []
    if abs(r[1] - fp[1]) < EPS:
        out.append("south")
    if abs(r[3] - fp[3]) < EPS:
        out.append("north")
    if abs(r[0] - fp[0]) < EPS:
        out.append("west")
    if abs(r[2] - fp[2]) < EPS:
        out.append("east")
    return out


def shared_edge(a: Rect, b: Rect) -> tuple[tuple[float, float], tuple[float, float]] | None:
    """Common boundary segment of two touching rectangles (length > 0), else None."""
    if abs(a[2] - b[0]) < EPS or abs(b[2] - a[0]) < EPS:  # vertical shared edge
        x = a[2] if abs(a[2] - b[0]) < EPS else a[0]
        lo, hi = max(a[1], b[1]), min(a[3], b[3])
        if hi - lo > EPS:
            return ((x, lo), (x, hi))
    if abs(a[3] - b[1]) < EPS or abs(b[3] - a[1]) < EPS:  # horizontal shared edge
        y = a[3] if abs(a[3] - b[1]) < EPS else a[1]
        lo, hi = max(a[0], b[0]), min(a[2], b[2])
        if hi - lo > EPS:
            return ((lo, y), (hi, y))
    return None


def _lerp(p: tuple[float, float], q: tuple[float, float], t: float) -> tuple[float, float]:
    return (round(p[0] + (q[0] - p[0]) * t, 3), round(p[1] + (q[1] - p[1]) * t, 3))


def _seg_len(p: tuple[float, float], q: tuple[float, float]) -> float:
    return abs(q[0] - p[0]) + abs(q[1] - p[1])


def _door_center(p, q, width: float) -> tuple[float, float]:
    length = _seg_len(p, q)
    if length < 2.2:
        return _lerp(p, q, 0.5)
    # near one end, leaving a wall return for furniture
    return _lerp(p, q, (0.25 + width / 2) / length)


def _on_segment(pt: tuple[float, float], p: tuple[float, float], q: tuple[float, float]) -> bool:
    """Axis-aligned segments only (all our walls are)."""
    (x, y), (x0, y0), (x1, y1) = pt, p, q
    return (min(x0, x1) - EPS <= x <= max(x0, x1) + EPS) and (min(y0, y1) - EPS <= y <= max(y0, y1) + EPS)


def build_model(brief: Brief, layout: Layout) -> BuildingModel:
    fp = layout.footprint
    rects = layout.rooms
    meta = {r.id: r for r in brief.rooms}

    walls: list[Wall] = []
    ext_ids: dict[Side, str] = {}
    for side, (p, q) in _edges(fp).items():
        wid = f"ext_{side}"
        ext_ids[side] = wid
        walls.append(Wall(id=wid, start=p, end=q, thickness=EXTERIOR_WALL, kind="exterior"))

    ids = list(rects)
    wall_between: dict[frozenset[str], Wall] = {}
    for i, a in enumerate(ids):
        for b in ids[i + 1 :]:
            seg = shared_edge(rects[a], rects[b])
            if seg is None:
                continue
            w = Wall(id=f"int_{a}__{b}", start=seg[0], end=seg[1], thickness=INTERIOR_WALL, kind="interior")
            walls.append(w)
            wall_between[frozenset((a, b))] = w

    openings: list[Opening] = []

    # open plan: the walls between the hall, entry, living, kitchen and dining become wall-less
    # openings; short stubs stay where they meet the walls at their ends
    def in_open_plan(rid: str) -> bool:
        return rid == HALL or (rid in meta and meta[rid].parent is None and meta[rid].type in OPEN_PLAN_TYPES)

    open_pairs: set[frozenset[str]] = set()
    if brief.open_plan:
        open_pairs = {pair for pair in wall_between if all(in_open_plan(r) for r in pair)}
        kept = [w for w in walls if w.kind == "exterior" or not any(wall_between.get(p) is w for p in open_pairs)]

        def on_kept_wall(pt: tuple[float, float]) -> bool:
            return any(_on_segment(pt, k.start, k.end) for k in kept)

        for pair in sorted(open_pairs, key=sorted):
            a, b = sorted(pair)
            w = wall_between[pair]
            length = _seg_len(w.start, w.end)
            # keep a short return where a remaining wall meets this end; else cut through the junction
            m0, m1 = ((EXTERIOR_WALL / 2 + 0.1) if on_kept_wall(p) else -(INTERIOR_WALL / 2 + 0.01)
                      for p in (w.start, w.end))
            width = round(length - m0 - m1, 3)
            if width < 0.9:
                open_pairs.discard(pair)
                continue
            openings.append(Opening(id=f"open_{a}__{b}", kind="open", wall_id=w.id, width=width, rooms=[a, b],
                                    center=_lerp(w.start, w.end, (m0 + width / 2) / length)))

    # entrance: entry room, else living, else the first front room
    front = layout.front
    entrance_room = next(
        (rid for t in ("entry", "living") for rid in front if meta[rid].type == t),
        front[0],
    )
    entrance_side = brief.entrance_side
    ep, eq = _edges(rects[entrance_room])[entrance_side]
    needs_window_same_edge = meta[entrance_room].effective_needs_window() and len(
        _exterior_sides(rects[entrance_room], fp)
    ) == 1
    t_door = 0.3 if needs_window_same_edge else 0.5
    openings.append(
        Opening(
            id="entrance",
            kind="entrance",
            wall_id=ext_ids[entrance_side],
            center=_lerp(ep, eq, t_door),
            width=1.0,
            rooms=["outside", entrance_room],
            swing_into=entrance_room,
        )
    )

    # interior doors: everyone opens onto the hall; without a hall, chain the front band
    def add_door(a: str, b: str, swing: str) -> None:
        w = wall_between.get(frozenset((a, b)))
        if w is None or frozenset((a, b)) in open_pairs:  # an open-plan pair needs no door
            return
        width = 0.8 if meta.get(swing) and meta[swing].type in ("bathroom", "storage", "laundry") else 0.9
        openings.append(
            Opening(
                id=f"door_{swing}",
                kind="door",
                wall_id=w.id,
                center=_door_center(w.start, w.end, width),
                width=width,
                rooms=[a, b],
                swing_into=swing,
            )
        )

    if HALL in rects:
        for rid, room in meta.items():
            if room.parent is None:
                add_door(HALL, rid, rid)
    else:
        order = layout.front
        hub = order.index(entrance_room)
        for i in range(hub - 1, -1, -1):
            add_door(order[i + 1], order[i], order[i])
        for i in range(hub + 1, len(order)):
            add_door(order[i - 1], order[i], order[i])
    # a nested room (ensuite, walk-in closet) opens only into its parent
    for rid, room in meta.items():
        if room.parent is not None:
            add_door(room.parent, rid, rid)

    # windows on exterior walls
    for rid, r in rects.items():
        if rid == HALL:
            continue
        room = meta[rid]
        sides = _exterior_sides(r, fp)
        if not sides:
            continue
        wants = room.effective_needs_window()
        if not wants and room.type != "bathroom":
            continue
        edges = _edges(r)
        # prefer the longest exterior edge, but not the entrance edge of the entrance room
        sides.sort(key=lambda s: (rid == entrance_room and s == entrance_side, -_seg_len(*edges[s])))
        side = sides[0]
        p, q = edges[side]
        length = _seg_len(p, q)
        width = 0.6 if room.type == "bathroom" else round(min(1.8, max(0.9, 0.4 * length)), 1)
        t = 0.72 if (rid == entrance_room and side == entrance_side) else 0.5
        openings.append(
            Opening(
                id=f"win_{rid}",
                kind="window",
                wall_id=ext_ids[side],
                center=_lerp(p, q, t),
                width=width,
                rooms=["outside", rid],
            )
        )

    placed: list[PlacedRoom] = []
    for rid, r in rects.items():
        ext = _exterior_sides(r, fp)
        inset = {s: (EXTERIOR_WALL if s in ext else INTERIOR_WALL) / 2 for s in ("south", "north", "west", "east")}
        net = (
            round(r[0] + inset["west"], 3),
            round(r[1] + inset["south"], 3),
            round(r[2] - inset["east"], 3),
            round(r[3] - inset["north"], 3),
        )
        area = round(max(0.0, net[2] - net[0]) * max(0.0, net[3] - net[1]), 2)
        if rid == HALL:
            name, rtype, target = "Hall", "corridor", area
        else:
            name, rtype, target = meta[rid].name, meta[rid].type, meta[rid].target_area
        placed.append(
            PlacedRoom(
                id=rid,
                name=name,
                type=rtype,
                rect=r,
                net_rect=net,
                net_area=area,
                target_area=target,
                exterior_sides=ext,
            )
        )

    fixtures, unplaced = place_fixtures(brief, placed, walls, openings)
    return BuildingModel(
        footprint=fp,
        lot=(brief.lot.width, brief.lot.depth),
        entrance_side=brief.entrance_side,
        rooms=placed,
        walls=walls,
        openings=openings,
        fixtures=fixtures,
        unplaced_fixtures=unplaced,
    )


__all__ = ["build_model", "shared_edge", "HALL", "local_side", "world_side"]
