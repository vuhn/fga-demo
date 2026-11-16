"""Deterministic quantity takeoff from the plan model plus the interview's facts and room scope.
No LLM, no prices: every line names the rate code it needs. A DXF is 2D, so heights, structure and
MEP come from facts the user gave (or agreed to assume) and those lines carry their dependencies."""

from __future__ import annotations

import math
from dataclasses import dataclass

from shapely.geometry import Point, Polygon

from .estimate_models import Confidence, SessionState, Takeoff, TakeoffLine
from .schemas import BuildingModel, PlacedRoom

DOOR_HEIGHT = 2.1
WINDOW_HEIGHT = 1.5
UPTURN = 0.3  # waterproofing turned up the wall
WET_TYPES = {"bathroom", "laundry", "balcony"}
TILED_WALL_HEIGHT = 2.1  # wall tile in a bathroom
SLAB_THICKNESS = 0.12
COLUMN_GRID = 3.5
ROOF_SLOPE_FACTOR = 1.15
REBAR_KG_PER_M3 = {"house": 120.0, "apartment": 110.0}
FORMWORK_M2_PER_M3 = 7.0
DEFAULT_FLOOR_HEIGHT = 3.0
# Fixtures the plan draws that are part of the building work; loose furniture and appliances are not priced.
PRICED_FIXTURES: dict[str, tuple[str, str]] = {  # kind -> (rate code, how it is counted)
    "toilet": ("fixture_toilet", "each"), "basin": ("fixture_basin", "each"), "shower": ("fixture_shower", "each"),
    "bathtub": ("fixture_bathtub", "each"), "sink": ("fixture_sink", "each"),
    "kitchen_counter": ("fixture_kitchen_counter", "run"), "wardrobe": ("fixture_wardrobe", "run"),
}
PLUMBED = {"toilet", "basin", "shower", "bathtub", "sink", "washer"}


def pending_fixtures(model: BuildingModel, state: SessionState):
    """Priced fixtures the plan shows that are not in a room already finished and not said to exist. A floor plan
    shows where things go, not what is installed, so each is assumed absent until the person says it exists."""
    active = {r.id for r in model.rooms if (state.scope[r.id].state if r.id in state.scope else "empty") != "existing"}
    return [f for f in model.fixtures if f.kind in PRICED_FIXTURES and f.room_id in active and f.id not in state.fixtures_existing]


@dataclass(frozen=True)
class RateSpec:
    package: str
    description: str  # what to search for; quality words are added for quality-sensitive codes
    unit: str
    quality_sensitive: bool = False


# Rate codes: descriptions and units only. There are no prices in the codebase.
RATE_CATALOG: dict[str, RateSpec] = {
    "demolition_finish": RateSpec("Demolition", "demolition of floor tiles and wall plaster, labour and disposal", "m2"),
    "brick_wall_100": RateSpec("Masonry", "100 mm brick wall built and plastered-ready, material and labour", "m2"),
    "plaster": RateSpec("Plaster", "cement plaster 15 mm on walls, material and labour", "m2"),
    "paint": RateSpec("Finishes", "interior wall paint, putty and 2 coats, material and labour", "m2", True),
    "floor_tile": RateSpec("Finishes", "ceramic or porcelain floor tile, supply and install", "m2", True),
    "wall_tile": RateSpec("Finishes", "bathroom wall tile, supply and install", "m2", True),
    "ceiling": RateSpec("Finishes", "gypsum board false ceiling, supply and install", "m2", True),
    "skirting": RateSpec("Finishes", "floor skirting, supply and install", "m", True),
    "waterproofing": RateSpec("Finishes", "waterproofing membrane for bathroom floor, material and labour", "m2"),
    "door_interior": RateSpec("Openings", "interior door with frame, supply and install", "each", True),
    "door_entrance": RateSpec("Openings", "entrance door with frame, supply and install", "each", True),
    "window": RateSpec("Openings", "aluminium glass window, supply and install", "m2", True),
    "electrical_point": RateSpec("MEP", "electrical point (outlet or light) including wiring, supply and install", "each", True),
    "plumbing_point": RateSpec("MEP", "water supply and drain point for a sanitary fixture, supply and install", "each", True),
    "fixture_toilet": RateSpec("Fixtures", "toilet (water closet), supply and install", "each", True),
    "fixture_basin": RateSpec("Fixtures", "washbasin with tap, supply and install", "each", True),
    "fixture_shower": RateSpec("Fixtures", "shower set with tray or screen, supply and install", "each", True),
    "fixture_bathtub": RateSpec("Fixtures", "bathtub with tap, supply and install", "each", True),
    "fixture_sink": RateSpec("Fixtures", "kitchen sink with tap, supply and install", "each", True),
    "fixture_kitchen_counter": RateSpec("Fixtures", "built-in kitchen cabinet and countertop, per metre run, supply and install", "m", True),
    "fixture_wardrobe": RateSpec("Fixtures", "built-in wardrobe, per metre run, supply and install", "m", True),
    "excavation": RateSpec("Earthwork and foundation", "foundation excavation by machine, per cubic metre", "m3"),
    "concrete": RateSpec("Structure", "ready-mix concrete C25 supplied, poured and cured", "m3"),
    "rebar": RateSpec("Structure", "reinforcement steel bar cut, bent and tied", "kg"),
    "formwork": RateSpec("Structure", "formwork for columns beams and slabs", "m2"),
    "roof_tile": RateSpec("Roof", "pitched roof with tiles on steel or concrete frame", "m2", True),
}


def _poly_area(pts) -> float:
    return abs(Polygon(pts).area) if len(pts) >= 3 else 0.0


def _perimeter(pts) -> float:
    return Polygon(pts).length if len(pts) >= 3 else 0.0


def _open_area(op, width: float) -> float:
    return width * (WINDOW_HEIGHT if op.kind == "window" else DOOR_HEIGHT)


def _conf(base: Confidence, state: SessionState, deps: list[str]) -> Confidence:
    if base == "assumed" or any(d in state.facts and state.facts[d].source == "assumed" for d in deps):
        return "assumed"
    return base


def _fact_num(state: SessionState, name: str, default: float) -> float:
    f = state.facts.get(name)
    try:
        return float(f.value) if f is not None else default
    except (TypeError, ValueError):
        return default


def _fact_bool(state: SessionState, name: str, default: bool) -> bool:
    f = state.facts.get(name)
    return bool(f.value) if f is not None else default


def structure_in_scope(state: SessionState) -> bool:
    wt = state.facts.get("work_type")
    if wt is not None and wt.value == "new_build":
        return True
    return _fact_bool(state, "structure_in_scope", False)


def takeoff(model: BuildingModel, state: SessionState) -> Takeoff:
    out = Takeoff()
    if not model.rooms:
        out.status = "unsupported"
        out.warnings.append("This file is a drawing with no rooms, so there is nothing to measure. Import a floor plan with closed rooms.")
        return out

    height = _fact_num(state, "floor_height", DEFAULT_FLOOR_HEIGHT)
    work_type = str(state.facts["work_type"].value) if "work_type" in state.facts else "fit_out"
    mep = _fact_bool(state, "mep_in_scope", False)
    lines: list[TakeoffLine] = []

    def add(code, qty, basis, conf: Confidence, deps=(), room: str | None = None):
        if qty <= 0.0005:
            return
        spec = RATE_CATALOG[code]
        deps = list(deps)
        lines.append(TakeoffLine(
            code=code, package=spec.package, description=spec.description, quantity=round(qty, 3),
            unit=spec.unit, basis=basis, confidence=_conf(conf, state, deps), deps=deps, room_id=room))

    rooms: dict[str, PlacedRoom] = {r.id: r for r in model.rooms}
    state_of = {rid: (state.scope[rid].state if rid in state.scope else "empty") for rid in rooms}
    active = {rid for rid, s in state_of.items() if s != "existing"}
    openings_of: dict[str, list] = {rid: [] for rid in rooms}
    for op in model.openings:
        for rid in op.rooms:
            if rid in openings_of:
                openings_of[rid].append(op)

    for rid, room in rooms.items():
        st = state_of[rid]
        if st == "existing":
            continue
        poly = room.net_polygon()
        area = room.net_area or _poly_area(poly)
        perim = _perimeter(poly)
        ops = openings_of[rid]
        wall_area = max(perim * height - sum(_open_area(o, o.width) for o in ops), 0.0)
        label = room.name
        wet = room.type in WET_TYPES
        h_dep = ["floor_height"]

        if st == "to_replace":
            add("demolition_finish", area + wall_area, f"{label}: floor {area:.1f} m² + wall {wall_area:.1f} m² removed", "derived", h_dep, rid)
        if st == "new":
            # a new room: its partition walls are built (a wall shared with another room is split between the two)
            part = 0.0
            for w in model.walls:
                if w.kind == "interior":
                    mid = ((w.start[0] + w.end[0]) / 2, (w.start[1] + w.end[1]) / 2)
                    if Polygon(poly).buffer(w.thickness).contains(Point(mid)):
                        part += math.dist(w.start, w.end)
            masonry = max(part * height - sum(_open_area(o, o.width) for o in ops if o.kind != "window") / 2, 0.0)
            add("brick_wall_100", masonry, f"{label}: {part:.1f} m of partition × {height:.1f} m high", "derived", h_dep, rid)

        add("plaster", wall_area, f"{label}: room perimeter {perim:.1f} m × {height:.1f} m − openings", "derived", h_dep, rid)
        add("floor_tile", area, f"{label}: net floor area", "measured", [], rid)
        add("ceiling", area, f"{label}: net floor area", "measured", [], rid)
        tiled = 0.0
        if room.type == "bathroom":
            tiled = max(perim * TILED_WALL_HEIGHT - sum(_open_area(o, o.width) for o in ops), 0.0)
            add("wall_tile", tiled, f"{label}: perimeter {perim:.1f} m × {TILED_WALL_HEIGHT} m − openings", "derived", [], rid)
        add("paint", wall_area - tiled + area, f"{label}: plastered wall not tiled + ceiling", "derived", h_dep, rid)
        if wet:
            add("waterproofing", area + perim * UPTURN, f"{label}: floor {area:.1f} m² + {UPTURN} m upturn on {perim:.1f} m", "derived", [], rid)
        else:
            doors = sum(o.width for o in ops if o.kind in ("door", "entrance", "open"))
            add("skirting", perim - doors, f"{label}: perimeter {perim:.1f} m − door widths {doors:.1f} m", "derived", [], rid)

        if mep:
            outlets = max(2, round(area / 4))
            lights = max(1, round(area / 8))
            add("electrical_point", outlets + lights, f"{label}: {outlets} outlets (1 per 4 m²) + {lights} lights (1 per 8 m²)", "assumed", [], rid)

    for op in model.openings:
        touched = [r for r in op.rooms if r in active]
        if not touched or op.kind == "open":
            continue
        # a door between an active and an existing room belongs to the active one
        if op.kind == "window":
            add("window", _open_area(op, op.width), f"{op.id}: {op.width:.2f} m × {WINDOW_HEIGHT} m", "measured")
        elif op.kind == "entrance":
            add("door_entrance", 1, f"{op.id}: entrance, {op.width:.2f} m wide", "measured")
        else:
            add("door_interior", 1, f"{op.id}: {op.width:.2f} m wide", "measured")

    for fx in pending_fixtures(model, state):
        code, how = PRICED_FIXTURES[fx.kind]
        rm = rooms[fx.room_id].name if fx.room_id in rooms else fx.room_id
        qty = 1.0 if how == "each" else max(fx.rect[2] - fx.rect[0], fx.rect[3] - fx.rect[1])
        add(code, qty, f"{fx.kind} in {rm}" + ("" if how == "each" else f" ({qty:.1f} m run)") + ", assumed not installed yet", "measured", [], fx.room_id)
    skipped = sorted({f.kind for f in model.fixtures if f.kind not in PRICED_FIXTURES and f.room_id in active})
    if skipped:
        out.warnings.append("Loose furniture and appliances are not priced: " + ", ".join(skipped) + ".")

    if mep:
        for fx in model.fixtures:
            if fx.room_id in active and fx.id not in state.fixtures_existing and fx.kind in PLUMBED:
                add("plumbing_point", 1, f"{fx.kind} in {rooms[fx.room_id].name if fx.room_id in rooms else fx.room_id}", "measured", [], fx.room_id)
        if not any(ln.code == "plumbing_point" for ln in lines):
            out.warnings.append("MEP is in scope but the plan has no sanitary fixtures, so no plumbing points were counted.")

    if structure_in_scope(state):
        _structure(model, state, lines, add, work_type)
    elif work_type != "fit_out":
        out.warnings.append("Structure is out of scope, so no concrete, steel or foundation is priced.")

    # merge per-room lines of the same code into one line per code, keeping the basis list readable
    out.lines = _merge(lines)
    if not out.lines:
        out.warnings.append("Nothing to build: every room is marked as existing.")
    return out


def _structure(model: BuildingModel, state: SessionState, lines, add, work_type: str) -> None:
    storeys = max(1, round(_fact_num(state, "storeys", 1)))
    height = _fact_num(state, "floor_height", DEFAULT_FLOOR_HEIGHT)
    kind = str(state.facts["kind"].value) if "kind" in state.facts else "house"
    footprint = _poly_area(model.outline)
    ext = [w for w in model.walls if w.kind == "exterior"]
    ext_len = sum(math.dist(w.start, w.end) for w in ext)
    all_len = sum(math.dist(w.start, w.end) for w in model.walls)
    deps = ["storeys", "floor_height"]

    cols = max(4, math.ceil(footprint / (COLUMN_GRID * COLUMN_GRID)))
    col_v = cols * 0.2 * 0.2 * height * storeys
    beam_v = all_len * 0.2 * 0.3 * storeys
    slab_v = footprint * SLAB_THICKNESS * storeys
    foot_v = ext_len * 0.5 * 0.4
    concrete = col_v + beam_v + slab_v + foot_v
    add("excavation", ext_len * 0.6 * 1.0, f"strip foundation {ext_len:.1f} m × 0.6 m × 1.0 m deep", "assumed", [])
    add("concrete", concrete,
        f"{cols} columns 0.2×0.2 m on a {COLUMN_GRID} m grid + beams {all_len:.1f} m × 0.2×0.3 + slab {footprint:.1f} m² × {SLAB_THICKNESS} m "
        f"× {storeys} storey(s) + strip footing", "assumed", deps)
    ratio = REBAR_KG_PER_M3.get(kind, 120.0)
    add("rebar", concrete * ratio, f"{concrete:.1f} m³ × {ratio:.0f} kg/m³", "assumed", deps)
    add("formwork", concrete * FORMWORK_M2_PER_M3, f"{concrete:.1f} m³ × {FORMWORK_M2_PER_M3:.0f} m²/m³", "assumed", deps)
    if kind == "house":
        add("roof_tile", footprint * ROOF_SLOPE_FACTOR, f"footprint {footprint:.1f} m² × slope factor {ROOF_SLOPE_FACTOR}", "assumed", [])


def _merge(lines: list[TakeoffLine]) -> list[TakeoffLine]:
    merged: dict[str, TakeoffLine] = {}
    order = {"assumed": 2, "derived": 1, "measured": 0}
    for ln in lines:
        cur = merged.get(ln.code)
        if cur is None:
            merged[ln.code] = ln.model_copy(update={"room_id": None})
            continue
        cur.quantity = round(cur.quantity + ln.quantity, 3)
        cur.basis = f"{cur.basis}; {ln.basis}" if len(cur.basis) < 700 else cur.basis
        cur.deps = sorted(set(cur.deps) | set(ln.deps))
        if order[ln.confidence] > order[cur.confidence]:
            cur.confidence = ln.confidence
    return list(merged.values())
