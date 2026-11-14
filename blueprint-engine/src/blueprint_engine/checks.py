"""Brief feasibility (before layout) and rule checks on the built model (after).

Brief issues are phrased as facts about the brief so an LLM can fix them;
model checks are what the user sees in the Checks tab.
"""

from __future__ import annotations

from collections import deque

from shapely.geometry import LineString, box

from .fixtures import SPECS, opening_zones, overlap
from .layout import GROSS_FACTOR
from .schemas import CORRIDOR_WIDTH, MIN_AREA, PUBLIC_TYPES, Brief, BuildingModel, CheckResult

AREA_TOLERANCE = 0.20
DOOR_MARGIN = 0.10
MAX_ASPECT = 2.5
BATH_WALK = 9.0  # metres along the hall from a bedroom door to the nearest bathroom door


def buildable(brief: Brief) -> tuple[float, float]:
    """Buildable width (along the street) and depth after setbacks."""
    lot, sb = brief.lot, brief.lot.setbacks
    across = lot.width if brief.entrance_side in ("south", "north") else lot.depth
    along = lot.depth if brief.entrance_side in ("south", "north") else lot.width
    return across - 2 * sb.sides, along - sb.front - sb.back


def area_budget(brief: Brief) -> float:
    """Net room area that fits on the lot (after walls and the hall)."""
    bw, bd = buildable(brief)
    has_private = any(r.type not in PUBLIC_TYPES for r in brief.rooms)
    return max(0.0, (bw * bd - (CORRIDOR_WIDTH * bw if has_private else 0.0)) / GROSS_FACTOR)


def brief_issues(brief: Brief) -> list[str]:
    """Problems no layout can fix; empty means 'worth laying out'."""
    issues: list[str] = []
    bw, bd = buildable(brief)
    if bw < 4 or bd < 4:
        issues.append(f"setbacks leave {max(bw, 0):.1f} x {max(bd, 0):.1f} m to build on; at least 4 x 4 m is needed")
        return issues
    for r in brief.rooms:
        if r.target_area < MIN_AREA[r.type]:
            issues.append(f"{r.name} ({r.id}) is {r.target_area:g} m² but a {r.type} needs at least {MIN_AREA[r.type]:g} m²")
    if not any(r.type in PUBLIC_TYPES and r.parent is None for r in brief.rooms):
        issues.append("the house needs at least one entry, living or kitchen room to enter through")
    need = sum(r.target_area for r in brief.rooms) * GROSS_FACTOR
    has_private = any(r.type not in PUBLIC_TYPES for r in brief.rooms)
    hall = CORRIDOR_WIDTH * bw if has_private else 0.0
    if need + hall > bw * bd:
        issues.append(
            f"rooms total {sum(r.target_area for r in brief.rooms):.0f} m² (about {need + hall:.0f} m² with walls and hall) "
            f"but the buildable area is {bw:.1f} x {bd:.1f} m = {bw * bd:.0f} m²; shrink rooms, drop a room or enlarge the lot"
        )
    return issues


def _reachable(model: BuildingModel) -> set[str]:
    graph: dict[str, set[str]] = {}
    for o in model.openings:
        if o.kind in ("door", "entrance", "open"):
            a, b = o.rooms
            graph.setdefault(a, set()).add(b)
            graph.setdefault(b, set()).add(a)
    seen = {"outside"}
    queue = deque(["outside"])
    while queue:
        for n in graph.get(queue.popleft(), ()):
            if n not in seen:
                seen.add(n)
                queue.append(n)
    return seen


def run_checks(brief: Brief, model: BuildingModel) -> list[CheckResult]:
    """Rule checks in Text2BIM's categories (geometry, collision, information) plus the
    program (the brief's own rules) and design quality. Every failed result names the
    elements involved and a hint phrased as edit ops."""
    out: list[CheckResult] = []

    def add(rule: str, severity: str, ok: bool, message: str, room_id: str | None = None, *,
            category: str = "program", elements: list[str] | None = None, hint: str = "") -> None:
        out.append(CheckResult(rule=rule, severity=severity, ok=ok, message=message, room_id=room_id,
                               category=category, elements=elements if elements is not None else
                               ([room_id] if room_id else []), hint="" if ok else hint))

    meta = {r.id: r for r in brief.rooms}
    placed = {pr.id: pr for pr in model.rooms}
    reach = _reachable(model)
    walls = {w.id: w for w in model.walls}
    windows = {o.rooms[1] for o in model.openings if o.kind == "window"}

    # ---- program: the brief's own rules ----
    for pr in model.rooms:
        if pr.type == "corridor":
            continue
        room = meta[pr.id]
        nx0, ny0, nx1, ny1 = pr.net_rect
        short = min(nx1 - nx0, ny1 - ny0)
        min_area = MIN_AREA[room.type]
        add("min_area", "error", pr.net_area >= min_area, f"{pr.name}: {pr.net_area:.1f} m² (minimum {min_area:g} m²)",
            pr.id, hint=f"resize_room {pr.name} to at least {min_area + 1:g}")
        mw = room.effective_min_width()
        add("min_width", "error", short + 1e-6 >= mw, f"{pr.name}: narrowest side {short:.2f} m (minimum {mw:g} m)",
            pr.id, hint=f"resize_room {pr.name} larger, or set_side {pr.name} null to free its position")
        dev = abs(pr.net_area - room.target_area) / room.target_area
        add("target_area", "warning", dev <= AREA_TOLERANCE,
            f"{pr.name}: {pr.net_area:.1f} m² vs target {room.target_area:g} m² ({dev:.0%} off)", pr.id,
            hint=f"resize_room {pr.name} {round(pr.net_area * 2) / 2:g} to accept the built size")
        add("reachable", "error", pr.id in reach,
            f"{pr.name}: {'reachable' if pr.id in reach else 'not reachable'} from the entrance", pr.id,
            hint=f"set_inside {pr.name} null, or set_side {pr.name} null")
        if room.effective_needs_window():
            add("window", "error", pr.id in windows,
                f"{pr.name}: {'has' if pr.id in windows else 'has no'} window on an exterior wall", pr.id,
                hint=f"set_side {pr.name} to an outside wall")
        if room.side is not None:
            ok = room.side in pr.exterior_sides
            add("side", "error", ok, f"{pr.name}: {'touches' if ok else 'does not touch'} the {room.side} wall",
                pr.id, hint=f'set_side {{"room": "{pr.name}", "side": null}} (frees it), or shrink the rooms competing for the {room.side} wall')

    rects = {pr.id: box(*pr.rect) for pr in model.rooms}
    for room in brief.rooms:
        for other in room.adjacent_to:
            if room.id < other or room.id not in meta[other].adjacent_to:
                shared = rects[room.id].intersection(rects[other]).length
                add("adjacency", "warning", shared >= 0.9,
                    f"{room.name} {'shares' if shared >= 0.9 else 'does not share'} a wall with {meta[other].name}",
                    room.id, elements=[room.id, other],
                    hint=f"set_side {room.name} and {meta[other].name} to the same wall")

    names = {pr.id: pr.name for pr in model.rooms}
    for rid, kind in model.unplaced_fixtures:
        add("fixture_fit", "warning", False, f"{names.get(rid, rid)}: no room for the {kind.replace('_', ' ')}", rid,
            category="collision", hint=f"resize_room {names.get(rid, rid)} +2, or remove_fixture {kind}")
    for rid in sorted({f.room_id for f in model.fixtures}):
        if not any(r == rid for r, _ in model.unplaced_fixtures):
            n = sum(1 for f in model.fixtures if f.room_id == rid)
            add("fixture_fit", "warning", True, f"{names.get(rid, rid)}: all {n} fixtures fit", rid, category="collision")

    for room in brief.rooms:
        if room.parent is None:
            continue
        via = {next(x for x in o.rooms if x != room.id) for o in model.openings
               if o.kind == "door" and room.id in o.rooms}
        ok = via == {room.parent}
        add("nested", "error", ok,
            f"{room.name}: {'opens only into' if ok else 'does not open only into'} {meta[room.parent].name}",
            room.id, elements=[room.id, room.parent], hint=f"set_inside {room.name} null")

    # door clearances: fits its wall, and the leaf can swing inside the room
    net = {pr.id: pr.net_rect for pr in model.rooms}
    for o in model.openings:
        if o.kind in ("window", "open"):
            continue
        w = walls[o.wall_id]
        seg = LineString([w.start, w.end])
        fits = o.width <= seg.length - 2 * DOOR_MARGIN
        swing_ok = True
        if o.swing_into and o.swing_into in net:
            x0, y0, x1, y1 = net[o.swing_into]
            swing_ok = o.width <= min(x1 - x0, y1 - y0) + 1e-6
        target = names.get(o.swing_into or "", o.swing_into)
        add("door_clearance", "error", fits and swing_ok,
            f"{o.id}: {o.width:g} m leaf {'fits' if fits and swing_ok else 'does not fit'} (wall {seg.length:.2f} m)",
            o.swing_into, category="collision", elements=[o.id, *[r for r in o.rooms if r != "outside"]],
            hint=f"resize_room {target} larger")

    # ---- geometry: rooms tile the footprint without overlaps ----
    ids = list(rects)
    overlaps = [(a, b) for i, a in enumerate(ids) for b in ids[i + 1:] if rects[a].intersection(rects[b]).area > 1e-6]
    add("overlap", "error", not overlaps,
        "rooms do not overlap" if not overlaps else
        "overlapping rooms: " + ", ".join(f"{names[a]} / {names[b]}" for a, b in overlaps),
        category="geometry", elements=sorted({x for p in overlaps for x in p}), hint="regenerate; report this as a bug")
    fx0, fy0, fx1, fy1 = model.footprint
    covered = sum(r.area for r in rects.values())
    gap = (fx1 - fx0) * (fy1 - fy0) - covered
    add("coverage", "error", abs(gap) < 0.01, f"rooms cover the footprint (gap {gap:.2f} m²)",
        category="geometry", elements=[], hint="regenerate; report this as a bug")

    # ---- collision: fixtures against each other, door swings and windows ----
    by_room: dict[str, list] = {}
    for f in model.fixtures:
        by_room.setdefault(f.room_id, []).append(f)
    for rid, fs in by_room.items():
        pr = placed.get(rid)
        if pr is None:
            continue
        clash = [(a.id, b.id) for i, a in enumerate(fs) for b in fs[i + 1:] if overlap(a.rect, b.rect)]
        zones = opening_zones(pr, walls, model.openings)
        in_door = [(f.id, o.id) for f in fs for o, z, _ in zones if o.kind != "window" and overlap(f.rect, z)]
        tall_win = [(f.id, o.id) for f in fs for o, z, _ in zones
                    if o.kind == "window" and SPECS[f.kind].tall and overlap(f.rect, z)]
        if clash:
            add("fixture_overlap", "error", False, f"{pr.name}: fixtures overlap", rid, category="collision",
                elements=sorted({x for p in clash for x in p}), hint="regenerate; report this as a bug")
        if in_door:
            add("fixture_door", "error", False, f"{pr.name}: a fixture blocks a door swing", rid, category="collision",
                elements=sorted({x for p in in_door for x in p}), hint=f"resize_room {pr.name} larger")
        if tall_win:
            add("fixture_window", "warning", False, f"{pr.name}: a tall fixture stands in front of a window", rid,
                category="collision", elements=sorted({x for p in tall_win for x in p}),
                hint=f"resize_room {pr.name} larger")
        doors = [(o, z) for o, z, _ in zones if o.kind != "window" and o.swing_into == rid]
        swings = [(a.id, b.id) for i, (a, za) in enumerate(doors) for b, zb in doors[i + 1:] if overlap(za, zb)]
        if swings:
            add("door_swing", "warning", False, f"{pr.name}: two door leaves swing into each other", rid,
                category="collision", elements=sorted({x for p in swings for x in p}), hint=f"resize_room {pr.name} larger")

    # ---- information: every element carries what a consumer needs ----
    seen: dict[str, str] = {}
    dupes = []
    for pr in model.rooms:
        key = pr.name.strip().lower()
        if key in seen:
            dupes.append((seen[key], pr.id))
        seen[key] = pr.id
    add("names", "warning", not dupes,
        "room names are unique" if not dupes else "duplicate room names: " + ", ".join(names[b] for _, b in dupes),
        category="information", elements=[x for p in dupes for x in p], hint="rename_room one of them")
    bad_rooms = [pr.id for pr in model.rooms if not pr.name.strip() or pr.net_area <= 0]
    bad_doors = [o.id for o in model.openings if o.width <= 0 or len(o.rooms) != 2 or o.rooms[0] == o.rooms[1]]
    add("element_info", "error", not bad_rooms and not bad_doors,
        "every room has a name and an area; every opening has a width and two sides",
        category="information", elements=bad_rooms + bad_doors, hint="regenerate; report this as a bug")

    # ---- design quality (warnings, never blocking) ----
    for pr in model.rooms:
        room = meta.get(pr.id)
        if room is None:
            continue
        x0, y0, x1, y1 = pr.net_rect
        w, d = x1 - x0, y1 - y0
        ratio = max(w, d) / max(min(w, d), 0.1)
        add("aspect", "warning", ratio <= MAX_ASPECT + 0.05, f"{pr.name}: proportions {ratio:.1f}:1 (at most {MAX_ASPECT:g}:1)",
            pr.id, category="design", hint=f"resize_room {pr.name}, or set_side {pr.name} null")
        if room.type == "bedroom" and room.side != model.entrance_side:
            street = model.entrance_side in pr.exterior_sides
            add("bedroom_street", "warning", not street,
                f"{pr.name}: {'faces' if street else 'is away from'} the street", pr.id, category="design",
                hint="shrink the public rooms so the bedrooms fit at the back, or set_side the bedroom to the back wall")
    types = {r.type for r in brief.rooms}
    kitchens = [r for r in brief.rooms if r.type == "kitchen"]
    dinings = [r for r in brief.rooms if r.type == "dining"]
    explicit = {frozenset((r.id, a)) for r in brief.rooms for a in r.adjacent_to}
    for k in kitchens:
        for dn in dinings:
            if frozenset((k.id, dn.id)) in explicit:
                continue  # already reported by the adjacency rule
            shared = rects[k.id].intersection(rects[dn.id]).length
            add("kitchen_dining", "warning", shared >= 0.9,
                f"{k.name} {'is' if shared >= 0.9 else 'is not'} next to {dn.name}", k.id, category="design",
                elements=[k.id, dn.id], hint=f"set_adjacent {k.name} {dn.name}")
    if "bathroom" in types:
        door_at = {o.swing_into: o.center for o in model.openings if o.kind == "door" and "hall" in o.rooms}
        baths = [r.id for r in brief.rooms if r.type == "bathroom" and r.parent is None and r.id in door_at]
        for r in brief.rooms:
            if r.type != "bedroom" or r.id not in door_at:
                continue
            if any(c.type == "bathroom" for c in brief.children(r.id)):
                continue  # has an ensuite
            if not baths:
                continue
            bx, by = door_at[r.id]
            walk = min(abs(bx - door_at[b][0]) + abs(by - door_at[b][1]) for b in baths)
            add("bath_distance", "warning", walk <= BATH_WALK,
                f"{r.name}: nearest bathroom is {walk:.1f} m along the hall (at most {BATH_WALK:g} m)", r.id,
                category="design", hint=f"set_adjacent {r.name} with a bathroom")

    # stable issue ids: rule:room, numbered when a rule repeats for the same room
    count: dict[str, int] = {}
    for c in out:
        base = f"{c.rule}:{c.room_id or '-'}"
        count[base] = count.get(base, 0) + 1
        c.issue_id = base if count[base] == 1 else f"{base}:{count[base]}"
    return out


def failed(checks: list[CheckResult], severity: str = "error") -> list[CheckResult]:
    return [c for c in checks if not c.ok and c.severity == severity]
