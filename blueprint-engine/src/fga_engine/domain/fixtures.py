"""Place basic housing components (fixtures) inside rooms. Deterministic, code only.

Each fixture is an axis-aligned body that stands against one wall of its room's net
rectangle (or in the middle, for a dining table) and needs a clear zone in front of it.
A candidate is accepted when its body stays inside the room, out of door swings and out of
other fixtures' bodies and clear zones, and its clear zone stays inside the room and off other
bodies. Tall fixtures also stay off windows. Fixtures that fit nowhere are reported, not forced.
"""

from __future__ import annotations

from dataclasses import dataclass

from .schemas import Brief, Opening, PlacedFixture, PlacedRoom, Side, Wall

Rect = tuple[float, float, float, float]
STEP = 0.05
EPS = 1e-6
PASSAGE = 0.6  # clear depth kept on the non-swing side of a door


@dataclass(frozen=True)
class Spec:
    sizes: tuple[tuple[float, float], ...]  # (along the wall, out from the wall), largest first
    clear: float  # clear depth needed in front
    tall: bool = False  # keeps off windows
    centre: bool = False  # stands in the middle of the room (dining table)
    centred_on_wall: bool = False  # prefer the middle of its wall (bed)
    run_with: str | None = None  # prefer the wall this other fixture is on (kitchen run)
    opposite: str | None = None  # prefer the wall facing this other fixture (tv vs sofa)


SPECS: dict[str, Spec] = {
    "bed": Spec(((1.6, 2.0), (1.4, 2.0), (0.9, 1.9)), 0.5, centred_on_wall=True),
    "wardrobe": Spec(((1.8, 0.6), (1.2, 0.6), (0.9, 0.6)), 0.6, tall=True),
    "desk": Spec(((1.2, 0.6), (1.0, 0.5)), 0.7),
    "shelves": Spec(((1.0, 0.35), (0.6, 0.35)), 0.5, tall=True),
    "sofa": Spec(((2.0, 0.9), (1.6, 0.85)), 0.5),
    "tv_unit": Spec(((1.6, 0.45), (1.2, 0.4)), 0.3, opposite="sofa"),
    "dining_table": Spec(((1.6, 0.9), (1.2, 0.8), (0.8, 0.8)), 0.6, centre=True),
    "kitchen_counter": Spec(((1.8, 0.6), (1.2, 0.6), (0.8, 0.6)), 0.9),
    "sink": Spec(((0.8, 0.6), (0.6, 0.5)), 0.6, run_with="kitchen_counter"),
    "stove": Spec(((0.6, 0.6),), 0.9, run_with="kitchen_counter"),
    "fridge": Spec(((0.7, 0.7), (0.6, 0.65)), 0.9, tall=True, run_with="kitchen_counter"),
    "toilet": Spec(((0.4, 0.7),), 0.5),
    "basin": Spec(((0.6, 0.45), (0.5, 0.4)), 0.5),
    "shower": Spec(((0.9, 0.9), (0.8, 0.8)), 0.0),
    "bathtub": Spec(((1.7, 0.75), (1.5, 0.7)), 0.5),
    "washer": Spec(((0.6, 0.6),), 0.6),
}
# big items first, so small ones fill the gaps
_PRIORITY = ["bed", "dining_table", "bathtub", "shower", "sofa", "kitchen_counter", "wardrobe", "desk",
             "fridge", "stove", "sink", "toilet", "basin", "washer", "tv_unit", "shelves"]
_FACING: dict[Side, Side] = {"south": "north", "north": "south", "west": "east", "east": "west"}


def overlap(a: Rect, b: Rect) -> bool:
    return a[0] < b[2] - EPS and b[0] < a[2] - EPS and a[1] < b[3] - EPS and b[1] < a[3] - EPS


def _inside(a: Rect, room: Rect) -> bool:
    return a[0] >= room[0] - EPS and a[1] >= room[1] - EPS and a[2] <= room[2] + EPS and a[3] <= room[3] + EPS


def _r(x0: float, y0: float, x1: float, y1: float) -> Rect:
    return (round(x0, 3), round(y0, 3), round(x1, 3), round(y1, 3))


def _on_wall(net: Rect, wall: Side, t: float, w: float, d: float, depth: float) -> Rect:
    """Rect of size w (along the wall) x depth (out from the wall), t along the wall."""
    x0, y0, x1, y1 = net
    if wall == "south":
        return _r(x0 + t, y0 + d, x0 + t + w, y0 + d + depth)
    if wall == "north":
        return _r(x0 + t, y1 - d - depth, x0 + t + w, y1 - d)
    if wall == "west":
        return _r(x0 + d, y0 + t, x0 + d + depth, y0 + t + w)
    return _r(x1 - d - depth, y0 + t, x1 - d, y0 + t + w)


def _wall_len(net: Rect, wall: Side) -> float:
    return (net[2] - net[0]) if wall in ("south", "north") else (net[3] - net[1])


def _side_of(net: Rect, wall: Wall) -> Side | None:
    """Which side of the room's net rect a wall runs along (within its half thickness)."""
    (ax, ay), (bx, by) = wall.start, wall.end
    tol = wall.thickness / 2 + 0.02
    x0, y0, x1, y1 = net
    if abs(ay - by) < EPS:  # horizontal
        if min(ax, bx) > x1 - EPS or max(ax, bx) < x0 + EPS:
            return None
        if abs(ay - y0) <= tol:
            return "south"
        if abs(ay - y1) <= tol:
            return "north"
        return None
    if min(ay, by) > y1 - EPS or max(ay, by) < y0 + EPS:
        return None
    if abs(ax - x0) <= tol:
        return "west"
    if abs(ax - x1) <= tol:
        return "east"
    return None


def _zone(net: Rect, side: Side, center: tuple[float, float], width: float, depth: float) -> Rect:
    x0, y0, x1, y1 = net
    cx, cy = center
    h = width / 2 + 0.05
    if side == "south":
        return _r(cx - h, y0, cx + h, y0 + depth)
    if side == "north":
        return _r(cx - h, y1 - depth, cx + h, y1)
    if side == "west":
        return _r(x0, cy - h, x0 + depth, cy + h)
    return _r(x1 - depth, cy - h, x1, cy + h)


def opening_zones(room: PlacedRoom, walls: dict[str, Wall], openings: list[Opening]
                  ) -> list[tuple[Opening, Rect, Side]]:
    """Each door's swing (or passage) zone and each window's zone inside this room."""
    out: list[tuple[Opening, Rect, Side]] = []
    for o in openings:
        if room.id not in o.rooms:
            continue
        side = _side_of(room.net_rect, walls[o.wall_id])
        if side is None:
            continue
        if o.kind == "window":
            out.append((o, _zone(room.net_rect, side, o.center, o.width, 0.7), side))
            continue
        depth = o.width if o.swing_into == room.id else PASSAGE
        out.append((o, _zone(room.net_rect, side, o.center, o.width, depth + 0.1), side))
    return out


def _keepouts(room: PlacedRoom, walls: dict[str, Wall], openings: list[Opening]) -> tuple[list[Rect], list[Rect], set[Side]]:
    """Door zones (no fixture bodies), window zones (no tall fixtures), and the walls with doors."""
    doors: list[Rect] = []
    windows: list[Rect] = []
    door_sides: set[Side] = set()
    for o, zone, side in opening_zones(room, walls, openings):
        if o.kind == "window":
            windows.append(zone)
        else:
            doors.append(zone)
            door_sides.add(side)
    return doors, windows, door_sides


def _place_room(room: PlacedRoom, kinds: list[str], walls: dict[str, Wall], openings: list[Opening]
                ) -> tuple[list[PlacedFixture], list[str]]:
    net = room.net_rect
    doors, windows, door_sides = _keepouts(room, walls, openings)
    bodies: list[Rect] = []
    clears: list[Rect] = []
    wall_of: dict[str, Side] = {}
    placed: list[PlacedFixture] = []
    missed: list[str] = []
    order = sorted(range(len(kinds)), key=lambda i: (_PRIORITY.index(kinds[i]), i))
    for i in order:
        kind = kinds[i]
        spec = SPECS[kind]

        def ok(body: Rect, clear: Rect | None) -> bool:
            if not _inside(body, net) or any(overlap(body, z) for z in doors):
                return False
            if any(overlap(body, b) for b in bodies) or any(overlap(body, c) for c in clears):
                return False
            if spec.tall and any(overlap(body, z) for z in windows):
                return False
            if clear is not None and (not _inside(clear, net) or any(overlap(clear, b) for b in bodies)):
                return False
            return True

        hit: tuple[Rect, Rect | None, Side] | None = None
        for w, d in spec.sizes:
            if spec.centre:
                cx, cy = (net[0] + net[2]) / 2, (net[1] + net[3]) / 2
                along_x = (net[2] - net[0]) >= (net[3] - net[1])
                hw, hd = (w / 2, d / 2) if along_x else (d / 2, w / 2)
                body = _r(cx - hw, cy - hd, cx + hw, cy + hd)
                c = spec.clear
                clear = _r(body[0] - c, body[1] - c, body[2] + c, body[3] + c)
                if ok(body, clear):
                    hit = (body, None, "north")
                    clears.append(clear)
                    break
                continue
            sides: list[Side] = ["south", "north", "west", "east"]
            pref = wall_of.get(spec.run_with or "") or (
                _FACING[wall_of[spec.opposite]] if spec.opposite in wall_of else None)
            # preferred wall first, then walls without doors, then the longest
            sides.sort(key=lambda s: (s != pref, s in door_sides, -_wall_len(net, s)))
            for side in sides:
                length = _wall_len(net, side)
                if length + EPS < w:
                    continue
                n = int((length - w) / STEP + EPS)
                ts = [k * STEP for k in range(n + 1)]
                if spec.centred_on_wall:
                    mid = (length - w) / 2
                    ts = sorted(ts + [mid], key=lambda t: abs(t - mid))
                for t in ts:
                    body = _on_wall(net, side, t, w, 0.0, d)
                    clear = _on_wall(net, side, t, w, d, spec.clear) if spec.clear > 0 else None
                    if ok(body, clear):
                        hit = (body, clear, side)
                        break
                if hit:
                    break
            if hit:
                break
        if hit is None:
            missed.append(kind)
            continue
        body, clear, side = hit
        bodies.append(body)
        if clear is not None:
            clears.append(clear)
        wall_of.setdefault(kind, side)
        placed.append(PlacedFixture(id=f"{room.id}__{kind}_{i}", kind=kind, room_id=room.id, rect=body,
                                    facing=_FACING[side]))
    placed.sort(key=lambda f: f.id)
    return placed, missed


def place_fixtures(brief: Brief, rooms: list[PlacedRoom], walls: list[Wall], openings: list[Opening]
                   ) -> tuple[list[PlacedFixture], list[tuple[str, str]]]:
    meta = {r.id: r for r in brief.rooms}
    by_id = {w.id: w for w in walls}
    out: list[PlacedFixture] = []
    unplaced: list[tuple[str, str]] = []
    for pr in rooms:
        room = meta.get(pr.id)
        if room is None or not room.fixtures:
            continue
        placed, missed = _place_room(pr, list(room.fixtures), by_id, openings)
        out += placed
        unplaced += [(pr.id, k) for k in missed]
    return out, unplaced
