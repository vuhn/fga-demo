"""DXF -> Plan: read the geometry of a CAD floor plan and build a BuildingModel from it. No LLM.

What is read (basic level, one storey per plan, 2D):
  * wall linework (layers named wall/mur/tuong/...) -> closed faces; thin faces are wall solids, thick
    ones are rooms (a drawing with single centerlines instead of double lines is handled too);
  * door and window symbols (blocks or loose entities on door/glazing layers) -> openings, and door
    doorways are filled into the wall solids so a doorway splits two rooms;
  * TEXT/MTEXT/attributes inside a room -> its name and type (English and Vietnamese keywords);
  * units from $INSUNITS, else guessed from the extent and checked against DIMENSION values.
A sheet holding several plans is split into one plan per connected cluster.

Everything it cannot use is listed in the ImportReport, never dropped silently.
"""

from __future__ import annotations

import io
import math
import re
import unicodedata
from dataclasses import dataclass, field

from pydantic import BaseModel, Field
from shapely.geometry import LineString, MultiPolygon, Point, Polygon, box
from shapely.geometry.polygon import orient
from shapely.ops import polygonize, unary_union

from . import dxf_drawing
from .domain.edits import EditError
from .domain.schemas import FIXTURE_KINDS, BuildingModel, DrawItem, Opening, Origin, PlacedFixture, PlacedRoom, Side, Wall

# $INSUNITS code -> metres per unit
INSUNITS = {1: 0.0254, 2: 0.3048, 4: 0.001, 5: 0.01, 6: 1.0, 9: 2.54e-8, 10: 0.9144}
GUESS_ORDER = (("mm", 0.001), ("m", 1.0), ("cm", 0.01), ("in", 0.0254), ("ft", 0.3048))
UNIT_FACTORS = {"mm": 0.001, "cm": 0.01, "m": 1.0, "in": 0.0254, "ft": 0.3048}

MAX_BYTES = 20 << 20
MAX_ENTITIES = 400_000
MIN_ROOM_AREA = 1.2  # m²
ROOM_INSCRIBED = 0.6  # a face narrower than this (m) is a wall, not a room
DEFAULT_WALL = 0.2
GAP = 0.05
MAX_WALL = 0.6  # m; a measured wall thicker than this is a ray that ran along a crossing wall

WALL_WORDS = ("wall", "mur", "muro", "wand", "tuong", "tường")
WALL_NOT = ("patt", "hatch", "fill", "finish", "dim", "text", "anno", "furn", "tag", "iden", "head")
DOOR_WORDS = ("door", "porte", "cua_di", "cua di", "cửa đi", "dor")
WINDOW_WORDS = ("glaz", "window", "wdw", "wnd", "fenetre", "cua_so", "cua so", "cửa sổ", "a-wind")
IGNORE_WORDS = ("dim", "anno", "text", "furn", "hatch", "patt", "grid", "axis", "defp", "title", "bord", "plmb",
                "fixt", "equp", "elev", "sect", "tree", "land", "prop", "site", "symb", "area", "iden", "fixture")

# (type, regex over the accent-stripped lowercase label), first match wins
LABELS: list[tuple[str, re.Pattern[str]]] = [
    (t, re.compile(rx)) for t, rx in [
        ("balcony", r"\b(balcony|terrace|veranda|deck|patio|loggia|ban cong|lo gia)\b"),
        ("bathroom", r"\b(bath|bathroo|bathroom|wc|toilet|restroom|shower|ensuite|en-suite|vs|ve sinh|phong tam|powder|lavatory)\b"),
        ("kitchen", r"\b(kitchen|kitchenette|pantry|bep|nha bep|cuisine)\b"),
        ("dining", r"\b(dining|dinner|phong an|salle a manger)\b"),
        ("living", r"\b(living|lounge|sitting|family room|great room|khach|phong khach|sinh hoat|salon|sejour)\b"),
        ("bedroom", r"\b(bed|bedroom|bdrm|br|master|phong ngu|pn|ngu|chambre|guest room)\b"),
        ("study", r"\b(study|office|home office|phong lam viec|lam viec)\b"),
        ("laundry", r"\b(laundry|utility|washing|giat|phoi)\b"),
        ("storage", r"\b(store|storage|closet|wardrobe|walk-in|walk in|wic|kho|linen|cupboard)\b"),
        ("entry", r"\b(entry|entrance|foyer|vestibule|porch|sanh)\b"),
        ("corridor", r"\b(hall|hallway|corridor|passage|lobby|hanh lang)\b"),
    ]
]
# when one space carries several labels (open-plan living/kitchen/dining) the first type here wins
TYPE_PRIORITY = ("balcony", "bathroom", "bedroom", "study", "living", "kitchen", "dining", "laundry", "storage",
                 "entry", "corridor")
# block-name / layer tokens -> fixture kind (the KIND attribute of our own exports wins over these)
FIXTURE_WORDS: list[tuple[str, tuple[str, ...]]] = [
    ("bathtub", ("bathtub", "bath_tub", "bath-tub", "tub", "bon_tam", "bon tam")),
    ("shower", ("shower", "sen_tam", "voi_sen")),
    ("toilet", ("toilet", "wc", "water_closet", "bon_cau", "bon cau")),
    ("basin", ("basin", "washbasin", "lavabo", "lavatory", "vanity")),
    ("sink", ("sink", "chau_rua", "chau rua")),
    ("stove", ("stove", "cooktop", "cooker", "hob", "oven", "range", "bep_ga")),
    ("fridge", ("fridge", "refrigerator", "tu_lanh", "tu lanh")),
    ("kitchen_counter", ("kitchen_counter", "counter", "worktop", "kitchen_unit", "tu_bep")),
    ("washer", ("washer", "washing", "laundry_machine", "may_giat")),
    ("dining_table", ("dining_table", "dining", "ban_an", "ban an")),
    ("tv_unit", ("tv_unit", "tv", "ke_tv")),
    ("sofa", ("sofa", "couch", "settee", "armchair")),
    ("bed", ("bed", "giuong")),
    ("wardrobe", ("wardrobe", "closet", "tu_ao", "tu ao", "cabinet")),
    ("desk", ("desk", "work_table", "ban_lam_viec")),
    ("shelves", ("shelf", "shelves", "bookcase", "ke_sach")),
]
FIXTURE_MIN, FIXTURE_MAX = 0.2, 3.5  # m; a block outside this size is not a fixture of ours
_TOKEN = re.compile(r"[^a-z0-9]+")
NUMERIC_TEXT = re.compile(r"^[\d\s.,'\"x×*+\-–/mMcCfFtTinN²³%°()]*$")


class NoFloorPlan(EditError):
    """The file holds no floor plan (no wall linework, or walls that enclose no room): it is imported as a drawing."""


class ImportReport(BaseModel):
    units: str  # "mm", "m", ...
    unit_source: str  # how the unit was decided
    layers_used: list[str] = Field(default_factory=list)
    layers_ignored: list[str] = Field(default_factory=list)
    plans_found: int = 0
    doors: int = 0
    windows: int = 0
    unlabeled_rooms: int = 0
    unmapped_texts: list[str] = Field(default_factory=list)
    fixtures: int = 0  # furniture and sanitary items kept
    skipped: list[str] = Field(default_factory=list)  # what the model cannot hold, with counts
    warnings: list[str] = Field(default_factory=list)


class ImportedPlan(BaseModel):
    index: int
    name: str
    model: BuildingModel
    room_count: int
    net_area: float


class ImportResult(BaseModel):
    plans: list[ImportedPlan]
    report: ImportReport


# ---------------------------------------------------------------- reading


@dataclass
class _Marker:
    kind: str  # "door" | "window"
    geom: Polygon  # bounding box of the symbol, metres
    layer: str
    whole: bool = False  # a block reference is one complete symbol; loose lines and arcs are grouped by touching
    anchor: tuple[float, float] | None = None  # a block's insertion point: on the wall for most door/window blocks
    handles: list[str] = field(default_factory=list)  # entity handles in the file (top-level entities only)
    # loose pieces of a door drawn as a leaf line and a swing arc: the arc (hinge, end A, end B) or the line (end, end)
    arc: tuple[tuple[float, float], tuple[float, float], tuple[float, float]] | None = None
    seg: tuple[tuple[float, float], tuple[float, float]] | None = None


@dataclass
class _Fixture:
    kind: str
    geom: Polygon  # bounding box of the block, metres
    name: str
    src: list[str] = field(default_factory=list)


@dataclass
class _Raw:
    walls: list[LineString] = field(default_factory=list)
    markers: list[_Marker] = field(default_factory=list)
    texts: list[tuple[str, float, float, str]] = field(default_factory=list)  # text, x, y, source handle ("" if none)
    fixtures: list[_Fixture] = field(default_factory=list)
    skipped: dict[str, int] = field(default_factory=dict)  # unsupported things dropped, by description
    dims: list[float] = field(default_factory=list)  # DIMENSION measurements in drawing units
    layers_used: set[str] = field(default_factory=set)
    layers_ignored: set[str] = field(default_factory=set)


def _norm(text: str) -> str:
    s = unicodedata.normalize("NFD", text.replace("đ", "d").replace("Đ", "D"))
    return "".join(c for c in s if unicodedata.category(c) != "Mn").lower().strip()


def _has(name: str, words: tuple[str, ...]) -> bool:
    n = _norm(name)
    return any(_norm(w) in n for w in words)


def _layer_role(layer: str) -> str:
    """wall | door | window | text | ignore | other"""
    n = _norm(layer)
    if _has(n, DOOR_WORDS) and "wall" not in n:
        return "door"
    if _has(n, WINDOW_WORDS):
        return "window"
    if _has(n, WALL_WORDS) and not _has(n, WALL_NOT):
        return "wall"
    if _has(n, IGNORE_WORDS):
        return "ignore"
    return "other"


def _fixture_kind(name: str, layer: str, attribs: dict[str, str]) -> str | None:
    """Our fixture kind for a block, from its KIND attribute, then its name, then its layer."""
    kind = _norm(attribs.get("KIND", "")).replace(" ", "_")
    if kind in FIXTURE_KINDS:
        return kind
    for source in (name, layer):
        toks = set(_TOKEN.split(_norm(source))) | {_norm(source).replace("-", "_")}
        flat = _norm(source)
        for k, words in FIXTURE_WORDS:
            for w in words:
                w = _norm(w)
                if w in toks or (len(w) > 4 and w in flat):
                    return k
    return None


def read_doc(data: bytes):
    import ezdxf
    from ezdxf import recover

    if len(data) > MAX_BYTES:
        raise EditError("the DXF is larger than 20 MB")
    try:
        doc, _ = recover.read(io.BytesIO(data))
    except Exception as e:  # ezdxf raises several error types on malformed files
        raise EditError(f"cannot read the DXF: {e}") from e
    if len(doc.modelspace()) == 0:
        raise EditError("the DXF has no drawing content in its model space")
    return doc


def _polylines(e, tol: float) -> list[list[tuple[float, float]]]:
    from ezdxf import path as ezpath

    out: list[list[tuple[float, float]]] = []
    try:
        p = ezpath.make_path(e)
    except Exception:
        return out
    for sub in p.sub_paths():
        pts = [(v.x, v.y) for v in sub.flattening(tol)]
        if sub.is_closed and pts and pts[0] != pts[-1]:
            pts.append(pts[0])
        if len(pts) >= 2:
            out.append(pts)
    return out


def _extents_poly(entities, tol: float) -> Polygon | None:
    xs: list[float] = []
    ys: list[float] = []
    for e in entities:
        for pts in _polylines(e, tol):
            for x, y in pts:
                xs.append(x)
                ys.append(y)
    if not xs:
        return None
    return box(min(xs), min(ys), max(xs), max(ys))


def _handles(e, depth: int, kind: str = "") -> list[str]:
    """The entity's handle when it is a top-level entity of the file (entities from an exploded block have none)."""
    if depth != 0:
        return []
    h = getattr(e.dxf, "handle", None)
    return [h] if h else []


def _walk(msp, depth: int = 0, parent_layer: str | None = None):
    """Model-space entities, with block references either yielded (door/window layers) or exploded."""
    for e in msp:
        yield e, parent_layer, depth


def collect(doc, scale_hint: float) -> _Raw:
    """Raw geometry in drawing units (scaled later). `scale_hint` only sets curve flattening."""
    from ezdxf import bbox as ezbbox  # noqa: F401  (kept for symmetry with older ezdxf)

    tol = 0.02 / max(scale_hint, 1e-9)
    raw = _Raw()
    layer_len: dict[str, float] = {}
    candidates: list[tuple[str, LineString]] = []
    n_entities = 0

    def handle(e, layer_override: str | None, depth: int) -> None:
        nonlocal n_entities
        n_entities += 1
        if n_entities > MAX_ENTITIES:
            raise EditError("the DXF has too many entities (limit 400000)")
        t = e.dxftype()
        layer = layer_override if (layer_override and e.dxf.layer == "0") else e.dxf.layer
        role = _layer_role(layer)
        if t == "DIMENSION":
            try:
                raw.dims.append(float(e.get_measurement()))
            except Exception:
                pass
            return  # drawn as part of the drawing items
        if t in ("TEXT", "MTEXT"):
            if role == "ignore" and "dim" in _norm(layer):
                return
            try:
                txt = e.plain_text() if t == "MTEXT" else e.dxf.text
                p = e.dxf.insert
                raw.texts.append((txt, float(p.x), float(p.y), _handles(e, depth, t)[0] if _handles(e, depth, t) else ""))
            except Exception:
                pass
            return
        if t == "INSERT":
            if role in ("door", "window") or _has(e.dxf.name, DOOR_WORDS + WINDOW_WORDS):
                kind = "door" if (role == "door" or (role != "window" and _has(e.dxf.name, DOOR_WORDS))) else "window"
                try:
                    poly = _extents_poly(e.virtual_entities(), tol)
                except Exception:
                    poly = None
                if poly is not None:
                    raw.markers.append(_Marker(kind, poly, layer, whole=True, anchor=(float(e.dxf.insert.x), float(e.dxf.insert.y)),
                                               handles=_handles(e, depth)))
                    raw.layers_used.add(layer)
                return
            if depth == 0:
                atts = {}
                for att in getattr(e, "attribs", []):
                    try:
                        atts[att.dxf.tag.upper()] = att.dxf.text
                    except Exception:
                        pass
                kind = _fixture_kind(e.dxf.name, layer, atts)
                if kind is not None:
                    try:
                        poly = _extents_poly(e.virtual_entities(), tol)
                    except Exception:
                        poly = None
                    if poly is not None:
                        raw.fixtures.append(_Fixture(kind, poly, e.dxf.name, _handles(e, depth)))
                        raw.layers_used.add(layer)
                        return
            # attributes of fixture, door and window blocks are tags (ID, KIND, ROOM), not room labels
            if _has(layer, ("iden", "text", "label", "room", "name")) and "dim" not in _norm(layer):
                for att in getattr(e, "attribs", []):
                    try:
                        hh = _handles(e, depth)
                        raw.texts.append((att.dxf.text, float(att.dxf.insert.x), float(att.dxf.insert.y),
                                          f"{hh[0]}:{att.dxf.tag}" if hh else ""))
                    except Exception:
                        pass
            if depth < 3 and role != "ignore":
                try:
                    for ve in e.virtual_entities():
                        handle(ve, layer, depth + 1)
                except Exception:
                    pass
            return
        if t not in ("LINE", "LWPOLYLINE", "POLYLINE", "ARC", "CIRCLE", "SPLINE", "ELLIPSE"):
            return
        if role in ("door", "window"):
            poly = _extents_poly([e], tol)
            if poly is not None:
                mk = _Marker(role, poly, layer, handles=_handles(e, depth))
                try:
                    if t == "ARC":
                        c, a, b = e.dxf.center, e.start_point, e.end_point
                        mk.arc = ((float(c.x), float(c.y)), (float(a.x), float(a.y)), (float(b.x), float(b.y)))
                    elif t == "LINE":
                        mk.seg = ((float(e.dxf.start.x), float(e.dxf.start.y)), (float(e.dxf.end.x), float(e.dxf.end.y)))
                except Exception:
                    pass
                raw.markers.append(mk)
                raw.layers_used.add(layer)
            return
        for pts in _polylines(e, tol):
            ls = LineString(pts)
            layer_len[layer] = layer_len.get(layer, 0.0) + ls.length
            if role == "wall":
                raw.walls.append(ls)
                raw.layers_used.add(layer)
            elif role == "other":
                candidates.append((layer, ls))
            else:
                raw.layers_ignored.add(layer)

    for e in doc.modelspace():
        handle(e, None, 0)

    if not raw.walls and candidates:  # no layer is named like a wall: use the longest unclassified layer
        best = max({lyr for lyr, _ in candidates}, key=lambda k: layer_len.get(k, 0.0))
        raw.walls = [ls for lyr, ls in candidates if lyr == best]
        raw.layers_used.add(best)
        raw.layers_ignored |= {lyr for lyr, _ in candidates} - {best}
    else:
        raw.layers_ignored |= {lyr for lyr, _ in candidates}
    raw.layers_ignored -= raw.layers_used
    return raw


# ---------------------------------------------------------------- units


def detect_units(doc, raw: _Raw, override: str | None) -> tuple[str, float, str]:
    if override:
        if override not in UNIT_FACTORS:
            raise EditError(f"unknown unit {override!r}; use one of {', '.join(UNIT_FACTORS)}")
        return override, UNIT_FACTORS[override], "chosen by you"
    if not raw.walls:
        raise NoFloorPlan("no wall linework found: name the wall layer like 'A-WALL' or draw the walls as lines")
    xs = [x for ls in raw.walls for x, _ in ls.coords]
    ys = [y for ls in raw.walls for _, y in ls.coords]
    extent = max(max(xs) - min(xs), max(ys) - min(ys))
    code = int(doc.header.get("$INSUNITS", 0) or 0)
    if code in INSUNITS:
        name = {1: "in", 2: "ft", 4: "mm", 5: "cm", 6: "m", 10: "yd"}.get(code, "m")
        return name, INSUNITS[code], "the file's $INSUNITS"
    # unitless: plans (a sheet of them) span 3-80 m after conversion, and the tallest DIMENSION must fit in it
    fits = [(n, f) for n, f in GUESS_ORDER if 3.0 <= extent * f <= 80.0]
    if not fits:
        raise EditError(f"cannot tell the unit: the drawing is {extent:g} units across; choose mm, cm, m, in or ft")
    name, factor = fits[0]
    how = f"guessed from the {extent:g}-unit extent (the file sets no $INSUNITS)"
    if raw.dims and max(raw.dims) * factor <= extent * factor * 1.02:
        how += f"; its dimensions (up to {max(raw.dims):g}) agree"
    return name, factor, how


# ---------------------------------------------------------------- geometry


def _scaled(raw: _Raw, k: float) -> _Raw:
    from shapely import affinity

    out = _Raw(layers_used=raw.layers_used, layers_ignored=raw.layers_ignored, dims=raw.dims, skipped=raw.skipped)
    out.fixtures = [_Fixture(f.kind, affinity.scale(f.geom, k, k, origin=(0, 0)), f.name, f.src) for f in raw.fixtures]
    out.walls = [affinity.scale(w, k, k, origin=(0, 0)) for w in raw.walls]
    sc = lambda p: (p[0] * k, p[1] * k)  # noqa: E731
    out.markers = [_Marker(m.kind, affinity.scale(m.geom, k, k, origin=(0, 0)), m.layer, m.whole,
                      (m.anchor[0] * k, m.anchor[1] * k) if m.anchor else None, m.handles,
                      tuple(sc(p) for p in m.arc) if m.arc else None, tuple(sc(p) for p in m.seg) if m.seg else None)
        for m in raw.markers]
    out.texts = [(t, x * k, y * k, h) for t, x, y, h in raw.texts]
    return out


def _faces(walls: list[LineString]) -> list[Polygon]:
    noded = unary_union(walls)
    return [p for p in polygonize(noded) if p.area > 1e-4]


def _is_room(p: Polygon) -> bool:
    return p.area >= MIN_ROOM_AREA and not p.buffer(-ROOM_INSCRIBED / 2).is_empty


def _unit(dx: float, dy: float) -> tuple[float, float]:
    n = math.hypot(dx, dy) or 1.0
    return dx / n, dy / n


def _fit_axis(solids, anchor: tuple[float, float], bounds: tuple[float, float, float, float]):
    """Where an opening symbol sits on a wall: (point on the wall face line, unit direction along the wall,
    distance to that face, side of the solid). The direction is the candidate edge direction near the symbol
    for which the wall continues on BOTH ends of the opening (the opening is a gap in the wall run); a wall
    that merely passes by, or a short jamb, scores lower. This is robust where the wall stubs beside a door
    are shorter than a wall thickness."""
    c = Point(anchor)
    minx, miny, maxx, maxy = bounds
    reach = max(maxx - minx, maxy - miny)
    solid_probe = solids.buffer(0.03)
    edges: list[tuple[tuple[float, float], tuple[float, float], float]] = []
    geoms = list(solids.geoms) if hasattr(solids, "geoms") else [solids]
    for g in geoms:
        for ring in [g.exterior, *g.interiors]:
            pts = list(ring.coords)
            for a_, b_ in zip(pts, pts[1:]):
                ln = math.hypot(b_[0] - a_[0], b_[1] - a_[1])
                if ln < 0.1:
                    continue
                dist = LineString([a_, b_]).distance(c)
                if dist <= max(1.5, reach):
                    edges.append((a_, b_, dist))
    if not edges:
        return None
    dirs: dict[int, tuple[float, float]] = {}
    for a_, b_, _ in edges:
        u = _unit(b_[0] - a_[0], b_[1] - a_[1])
        if u[0] < -1e-9 or (abs(u[0]) <= 1e-9 and u[1] < 0):
            u = (-u[0], -u[1])  # one orientation per line, but the edge's own numbers: no trigonometry noise
        dirs.setdefault(round(math.degrees(math.atan2(u[1], u[0])) % 180) % 180, u)
    best = None
    half = reach / 2 + 0.1
    for key, u in dirs.items():
        ends = sum(solid_probe.contains(Point(anchor[0] + sgn * u[0] * half, anchor[1] + sgn * u[1] * half)) for sgn in (-1, 1))
        par = [(a_, b_, d_) for a_, b_, d_ in edges
               if abs(_unit(b_[0] - a_[0], b_[1] - a_[1])[0] * u[1] - _unit(b_[0] - a_[0], b_[1] - a_[1])[1] * u[0]) < 0.05]
        if not par:
            continue
        near = min(d_ for _, _, d_ in par)
        score = (ends, -round(near, 2))
        if best is None or score > best[0]:
            best = (score, u, par)
    if best is None:
        return None
    _, u, par = best
    a_, b_, dist = min(par, key=lambda e: e[2])
    t = (c.x - a_[0]) * u[0] + (c.y - a_[1]) * u[1]
    mx, my = (a_[0] + b_[0]) / 2, (a_[1] + b_[1]) / 2
    ux, uy = _unit(b_[0] - a_[0], b_[1] - a_[1])
    side = 1.0 if solid_probe.contains(Point(mx - uy * 0.06, my + ux * 0.06)) else -1.0
    # `side` is relative to (-u_y, u_x) of the chosen direction u; edges may run the other way round
    if (ux * u[0] + uy * u[1]) < 0:
        side = -side
    return (a_[0] + ux * ((c.x - a_[0]) * ux + (c.y - a_[1]) * uy), a_[1] + uy * ((c.x - a_[0]) * ux + (c.y - a_[1]) * uy)), u, dist, side


def _thin_thickness(solids) -> float:
    if solids is None or solids.is_empty or solids.length <= 0:
        return DEFAULT_WALL
    return max(0.05, min(0.6, 2 * solids.area / solids.length))


@dataclass
class _Door:
    center: tuple[float, float]
    d: tuple[float, float]
    width: float
    kind: str
    patch: Polygon | None
    src: list[str] = field(default_factory=list)


def _door_anchor(group: list[_Marker]) -> tuple[float, float] | None:
    """The middle of the doorway of a door drawn as a swing arc (hinge at its centre) and a leaf line from the
    hinge: the arc's end away from the leaf is the opposite jamb, so hinge and jamb are the wall line and their
    midpoint is on it. The bounding box centre of leaf and arc lies in the room instead, up to half a door
    width off the wall, and fits the wrong wall. None when the group is not a leaf and an arc."""
    arcs = [m.arc for m in group if m.arc]
    segs = [m.seg for m in group if m.seg]
    if len(arcs) != 1 or not segs:
        return None
    hinge, a, b = arcs[0]
    tol = 0.05 * max(1.0, math.hypot(a[0] - hinge[0], a[1] - hinge[1]))
    for p, q in segs:
        for h, leaf in ((p, q), (q, p)):
            if math.hypot(h[0] - hinge[0], h[1] - hinge[1]) <= tol:
                jamb = b if math.hypot(a[0] - leaf[0], a[1] - leaf[1]) <= math.hypot(b[0] - leaf[0], b[1] - leaf[1]) else a
                return ((hinge[0] + jamb[0]) / 2, (hinge[1] + jamb[1]) / 2)
    return None


def _symbols(markers: list[_Marker]) -> list[tuple[str, tuple[float, float, float, float], tuple[float, float], list[str]]]:
    """One bounding box per symbol: loose pieces of one door (leaf + swing) are grouped by proximity.
    The last item is the handles of the file entities that draw it."""
    out: list[tuple[str, tuple[float, float, float, float], tuple[float, float], list[str]]] = []
    for kind in ("door", "window"):
        for m in markers:
            if m.kind == kind and m.whole:
                b = m.geom.bounds
                out.append((kind, b, m.anchor if m.anchor and m.geom.buffer(0.3).contains(Point(m.anchor)) else ((b[0] + b[2]) / 2, (b[1] + b[3]) / 2), m.handles))
        ms = [m for m in markers if m.kind == kind and not m.whole]
        if not ms:
            continue
        merged = unary_union([m.geom.buffer(0.04) for m in ms])
        for comp in (merged.geoms if hasattr(merged, "geoms") else [merged]):
            group = [m for m in ms if m.geom.buffer(0.04).intersects(comp)]
            members = [m.geom for m in group]
            bb = (min(g.bounds[0] for g in members), min(g.bounds[1] for g in members),
                  max(g.bounds[2] for g in members), max(g.bounds[3] for g in members))
            hs = [h for m in group for h in m.handles]
            hinge = _door_anchor(group) if kind == "door" else None
            # a piece with no handle (inside a nested block) makes the symbol unpatchable as a whole
            out.append((kind, bb, hinge or ((bb[0] + bb[2]) / 2, (bb[1] + bb[3]) / 2), hs if all(m.handles for m in group) else []))
    return out


def _door_bars(solids, markers: list[_Marker]) -> list[LineString]:
    """A line across each doorway and window opening on the wall face nearest its symbol. CAD exports cut
    both out of the wall solids, so without it the room leaks into the outside (or its neighbour) and no
    closed face is found."""
    bars: list[LineString] = []
    if solids is None or solids.is_empty:
        return bars
    for kind, bb, anchor, _ in _symbols(markers):
        fit = _fit_axis(solids, anchor, bb)
        if fit is None or fit[2] > 1.5:
            continue
        p, d, _, _ = fit
        minx, miny, maxx, maxy = bb
        along = [(x - p[0]) * d[0] + (y - p[1]) * d[1] for x, y in ((minx, miny), (maxx, miny), (maxx, maxy), (minx, maxy))]
        lo, hi = min(along) - 0.1, max(along) + 0.1
        bars.append(LineString([(p[0] + d[0] * lo, p[1] + d[1] * lo), (p[0] + d[0] * hi, p[1] + d[1] * hi)]))
    return bars


def _place_markers(markers: list[_Marker], solids, t_est: float) -> list[_Door]:
    """Fit each symbol onto the nearest wall solid."""
    out: list[_Door] = []
    for kind, (minx, miny, maxx, maxy), anchor, handles in _symbols(markers):
        if True:
            c = Point(anchor)
            near = _fit_axis(solids, anchor, (minx, miny, maxx, maxy))
            if near is None:
                continue
            p, d, dist, side = near
            if dist > 1.5:
                continue
            nx, ny = -d[1], d[0]
            # extent of the symbol along the wall = the doorway width
            corners = [(minx, miny), (maxx, miny), (maxx, maxy), (minx, maxy)]
            along = [(x - p[0]) * d[0] + (y - p[1]) * d[1] for x, y in corners]
            lo, hi = min(along), max(along)
            width = hi - lo
            if width < 0.4 or width > 4.0:
                continue
            mid_along = (lo + hi) / 2
            # the wall's own thickness here: measure beside the doorway, where the wall is solid
            probes = []
            for sgn in (-1.0, 1.0):
                a_ = mid_along + sgn * (width / 2 + 0.12)
                base = Point(p[0] + d[0] * a_, p[1] + d[1] * a_)
                probes.append(_edge_thickness(base, (side * nx, side * ny), solids))
            # the smaller hit: a probe next to a corner can run through the crossing wall
            hits = [x for x in probes if x >= 0.05]
            t_loc = min(min(hits), MAX_WALL) if hits else t_est
            cx = p[0] + d[0] * mid_along + side * nx * t_loc / 2
            cy = p[1] + d[1] * mid_along + side * ny * t_loc / 2
            patch = None
            if True:  # doors and windows alike: the opening is filled into the wall solids
                hw, ht = width / 2 + 0.02, t_loc / 2 + 0.01
                patch = Polygon([(cx - d[0] * hw - nx * ht, cy - d[1] * hw - ny * ht),
                                 (cx + d[0] * hw - nx * ht, cy + d[1] * hw - ny * ht),
                                 (cx + d[0] * hw + nx * ht, cy + d[1] * hw + ny * ht),
                                 (cx - d[0] * hw + nx * ht, cy - d[1] * hw + ny * ht)])
            out.append(_Door((cx, cy), d, round(width, 3), kind, patch, handles))
    return out


def _room_type(labels: list[str]) -> str:
    found = {t for lab in labels for t, rx in LABELS if rx.search(_norm(lab))}
    for t in TYPE_PRIORITY:
        if t in found:
            return t
    return "other"


def _title(label: str) -> str:
    s = re.sub(r"\s+", " ", label.replace("\\P", " ").replace("\n", " ")).strip()
    return s[:1].upper() + s[1:].lower() if s.isupper() else s


def _edge_thickness(mid: Point, n: tuple[float, float], solids) -> float:
    seg = LineString([(mid.x, mid.y), (mid.x + n[0] * 1.0, mid.y + n[1] * 1.0)])
    inter = seg.intersection(solids)
    best = 0.0
    for g in (inter.geoms if hasattr(inter, "geoms") else [inter]):
        if g.geom_type == "LineString" and g.distance(mid) < 0.03:
            best = max(best, g.length)
    return best


def _walls_from_rooms(rooms: list[Polygon], solids, t_default: float) -> tuple[list[Wall], dict[int, list[Side]]]:
    """Wall centerlines from the room edges (and the room's sides that face outside)."""
    ext_sides: dict[int, list[Side]] = {}
    groups: dict[tuple, list[tuple[float, float, float]]] = {}
    meta: dict[tuple, tuple[float, float, float, str, float]] = {}
    for ri, room in enumerate(rooms):
        poly = orient(room.simplify(0.02, preserve_topology=True), 1.0)
        pts = list(poly.exterior.coords)
        for a, b in zip(pts, pts[1:]):
            length = math.hypot(b[0] - a[0], b[1] - a[1])
            if length < 0.1:
                continue
            ux, uy = _unit(b[0] - a[0], b[1] - a[1])
            n = (uy, -ux)  # CCW polygon: the outside is on the right
            mid = Point((a[0] + b[0]) / 2, (a[1] + b[1]) / 2)
            # sample along the edge and keep the smallest hit: near a corner a ray can run along a crossing wall
            samples = [_edge_thickness(Point(a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f), n, solids)
                       for f in ((0.5,) if length < 0.8 else (0.3, 0.5, 0.7))]
            hits = [x for x in samples if x >= 0.05]
            t = min(min(hits), MAX_WALL) if hits else 0.0
            if t < 0.05:
                continue  # open edge (a balcony rail): no wall
            if length < 0.5 and min(hits) > 2.5 * length:
                continue  # the free end of a dangling wall: the ray ran along it, not through it (its sides give the wall)
            probe = Point(mid.x + n[0] * (t + 0.06), mid.y + n[1] * (t + 0.06))
            interior = any(o is not room and o.contains(probe) for o in rooms)
            kind = "interior" if interior else "exterior"
            if kind == "exterior" and length >= 0.3:
                sd = _side_of(n)
                if sd not in ext_sides.setdefault(ri, []):
                    ext_sides[ri].append(sd)
            ang = math.atan2(uy, ux) % math.pi
            # canonical direction: angle in [0, pi)
            cu = (math.cos(ang), math.sin(ang))
            cx, cy = (a[0] + n[0] * t / 2, a[1] + n[1] * t / 2)
            off = cy * cu[0] - cx * cu[1]  # signed distance of the centerline from the origin
            key = (round(ang / 0.01), round(off / 0.03), round(t / 0.02), kind)
            s0 = (a[0] + n[0] * t / 2) * cu[0] + (a[1] + n[1] * t / 2) * cu[1]
            s1 = (b[0] + n[0] * t / 2) * cu[0] + (b[1] + n[1] * t / 2) * cu[1]
            groups.setdefault(key, []).append((min(s0, s1), max(s0, s1), t))
            meta[key] = (cu[0], cu[1], off, kind, t)
    walls: list[Wall] = []
    for key, ivs in groups.items():
        ux, uy, off, kind, t = meta[key]
        ivs.sort()
        merged: list[list[float]] = []
        for lo, hi, _ in ivs:
            if merged and lo <= merged[-1][1] + GAP:
                merged[-1][1] = max(merged[-1][1], hi)
            else:
                merged.append([lo, hi])
        for lo, hi in merged:
            lo, hi = lo - t / 2, hi + t / 2  # reach the corner
            # a point on the line: perpendicular foot of the origin is off * (-uy, ux)
            bx, by = -uy * off, ux * off
            walls.append(Wall(id="", start=(round(bx + ux * lo, 3), round(by + uy * lo, 3)),
                              end=(round(bx + ux * hi, 3), round(by + uy * hi, 3)),
                              thickness=round(t, 3), kind=kind))
    for i, w in enumerate(sorted(walls, key=lambda w: (w.start[1], w.start[0], w.end[1], w.end[0])), 1):
        w.id = f"w{i}"
    return walls, ext_sides


def _side_of(n: tuple[float, float]) -> Side:
    if abs(n[0]) >= abs(n[1]):
        return "east" if n[0] > 0 else "west"
    return "north" if n[1] > 0 else "south"


def _build_plan(index: int, rooms: list[Polygon], solids, doors: list[_Door], texts: list[tuple[str, float, float, str]],
                t_est: float, report: ImportReport, fixtures: list[_Fixture] | None = None,
                scale: float = 1.0) -> ImportedPlan | None:
    from shapely import affinity

    region = unary_union([*rooms, solids])
    minx, miny, maxx, maxy = region.bounds
    shift = lambda g: affinity.translate(g, -minx, -miny)  # noqa: E731
    rooms = [shift(r) for r in rooms]
    solids = shift(solids)
    doors = [_Door((d.center[0] - minx, d.center[1] - miny), d.d, d.width, d.kind, None, d.src) for d in doors]
    texts = [(t, x - minx, y - miny, h) for t, x, y, h in texts]
    fixtures = [_Fixture(f.kind, shift(f.geom), f.name, f.src) for f in fixtures or []]

    # label rooms
    labels: dict[int, list[str]] = {i: [] for i in range(len(rooms))}
    label_src: dict[int, list[str]] = {i: [] for i in range(len(rooms))}
    for t, x, y, h in texts:
        pt = Point(x, y)
        hit = [i for i, r in enumerate(rooms) if r.buffer(0.05).contains(pt)]
        if not hit:
            continue
        # a label block is often "Kitchen\P11.4 m²": keep the words, drop the area and dimension lines
        for line in re.split(r"\\P|\n|\r", t):
            txt = line.strip()
            if txt and not NUMERIC_TEXT.match(txt):
                labels[hit[0]].append(txt)
                if h and h not in label_src[hit[0]]:
                    label_src[hit[0]].append(h)
    order = sorted(range(len(rooms)), key=lambda i: (round(rooms[i].centroid.y, 1) * -1, rooms[i].centroid.x))
    placed: list[PlacedRoom] = []
    poly_of: dict[str, Polygon] = {}
    unlabeled = 0
    for n, i in enumerate(order, 1):
        r = orient(rooms[i].simplify(0.02, preserve_topology=True), 1.0)
        labs = []
        for lab in labels[i]:
            for part in re.split(r"\s*[/&]\s*", lab):
                if part.strip() and _title(part) not in labs:
                    labs.append(_title(part))
        typ = _room_type(labs)
        if typ == "other" and labs:
            report.unmapped_texts += [f"{lab} (in room {n})" for lab in labs]
        if not labs:
            unlabeled += 1
        name = " / ".join(labs) if labs else f"Room {n}"
        x0, y0, x1, y1 = r.bounds
        rid = f"r{n}"
        coords = [(round(x, 3), round(y, 3)) for x, y in list(r.exterior.coords)[:-1]]
        placed.append(PlacedRoom(id=rid, name=name[:60], type=typ, rect=(x0, y0, x1, y1), net_rect=(x0, y0, x1, y1),
                                 net_area=round(r.area, 2), target_area=round(r.area, 2), exterior_sides=[],
                                 polygon=coords, src=label_src[i]))
        poly_of[rid] = r
    report.unlabeled_rooms += unlabeled

    walls, ext_sides = _walls_from_rooms([poly_of[p.id] for p in placed], solids, t_est)
    for ri, p in enumerate(placed):
        p.exterior_sides = ext_sides.get(ri, [])
    by_wall = {w.id: w for w in walls}

    # openings: fit each symbol to its wall
    openings: list[Opening] = []
    n_door = n_win = 0
    ids = list(poly_of)
    for dd in doors:
        c = Point(dd.center)
        wall = min(walls, key=lambda w: LineString([w.start, w.end]).distance(c), default=None)
        if wall is None:
            continue
        line = LineString([wall.start, wall.end])
        if line.distance(c) > wall.thickness / 2 + 0.3:
            report.warnings.append(f"a {dd.kind} symbol at ({dd.center[0]:.1f}, {dd.center[1]:.1f}) m is not on a wall; skipped")
            continue
        pc = line.interpolate(line.project(c))
        ux, uy = _unit(wall.end[0] - wall.start[0], wall.end[1] - wall.start[1])
        nx, ny = -uy, ux
        off = wall.thickness / 2 + 0.25
        sides = []
        for sgn in (1, -1):
            q = Point(pc.x + nx * off * sgn, pc.y + ny * off * sgn)
            hit = next((rid for rid in ids if poly_of[rid].buffer(0.05).contains(q)), "outside")
            sides.append(hit)
        if sides[0] == sides[1]:
            continue
        typ = {p.id: p.type for p in placed}
        if dd.kind == "window":
            inner = [s for s in sides if s != "outside"]
            if not inner:
                continue
            inner_room = next((s for s in inner if typ[s] != "balcony"), inner[0])
            conn = ["outside", inner_room]
            kind, swing = "window", None
            n_win += 1
        else:
            conn = sides
            kind = "entrance" if "outside" in conn else "door"
            inside = [s for s in conn if s != "outside"]
            swing = min(inside, key=lambda s: poly_of[s].area) if inside else None
            n_door += 1
        openings.append(Opening(id="", kind=kind, wall_id=wall.id, center=(round(pc.x, 3), round(pc.y, 3)),
                                width=min(dd.width, max(0.5, line.length - 0.3)), rooms=conn, swing_into=swing,
                                src=dd.src))
    placed_fixtures = _place_fixtures(fixtures, poly_of, report)

    # only one entrance: the first stays, further exterior doors become plain doors
    seen = False
    for o in openings:
        if o.kind == "entrance":
            if seen:
                o.kind = "door"
            seen = True
    for i, o in enumerate(sorted(openings, key=lambda o: (o.kind != "entrance", o.kind, o.center[1], o.center[0])), 1):
        o.id = f"{'w' if o.kind == 'window' else 'd'}{i}"
    report.doors += n_door
    report.windows += n_win

    outline_geom = unary_union([*poly_of.values(), solids]).buffer(0.01).buffer(-0.01)
    if isinstance(outline_geom, MultiPolygon):
        outline_geom = max(outline_geom.geoms, key=lambda g: g.area)
    outline = orient(Polygon(outline_geom.exterior).simplify(0.02), 1.0)
    outline_pts = [(round(x, 3), round(y, 3)) for x, y in list(outline.exterior.coords)[:-1]]
    ox0, oy0, ox1, oy1 = outline.bounds

    ent = next((o for o in openings if o.kind == "entrance"), None)
    entrance_side: Side = "south"
    if ent is not None:
        w = by_wall[ent.wall_id]
        ux, uy = _unit(w.end[0] - w.start[0], w.end[1] - w.start[1])
        inner = poly_of[ent.rooms[1] if ent.rooms[0] == "outside" else ent.rooms[0]].centroid
        n = (-uy, ux)
        if (inner.x - ent.center[0]) * n[0] + (inner.y - ent.center[1]) * n[1] > 0:
            n = (-n[0], -n[1])  # point outward
        entrance_side = _side_of(n)
    elif not openings:
        report.warnings.append(f"plan {index}: no doors recognised; every room is reported unreachable")

    model = BuildingModel(
        footprint=(round(ox0, 3), round(oy0, 3), round(ox1, 3), round(oy1, 3)), outline=outline_pts,
        lot=(round(ox1 - ox0, 2), round(oy1 - oy0, 2)), entrance_side=entrance_side, source="imported",
        origin=Origin(scale=scale, dx=-minx, dy=-miny),
        rooms=placed, walls=walls, openings=openings, fixtures=placed_fixtures)
    area = round(sum(p.net_area for p in placed if p.type != "balcony"), 1)
    return ImportedPlan(index=index, name=f"Plan {index}", model=model, room_count=len(placed), net_area=area)


def _place_fixtures(fixtures: list[_Fixture], poly_of: dict[str, Polygon], report: ImportReport) -> list[PlacedFixture]:
    """Each fixture goes to the room holding its centre; it faces away from the nearest room edge."""
    out: list[PlacedFixture] = []
    counts: dict[str, int] = {}
    for f in sorted(fixtures, key=lambda f: (f.geom.centroid.y, f.geom.centroid.x)):
        c = f.geom.centroid
        x0, y0, x1, y1 = f.geom.bounds
        if not (FIXTURE_MIN <= max(x1 - x0, y1 - y0) <= FIXTURE_MAX):
            report.skipped.append(f"{f.kind} block '{f.name}': size {max(x1 - x0, y1 - y0):.1f} m is not a fixture")
            continue
        rid = next((r for r, p in poly_of.items() if p.buffer(0.05).contains(c)), None)
        if rid is None:
            report.skipped.append(f"{f.kind} block '{f.name}' is not inside a room")
            continue
        room = poly_of[rid]
        rx0, ry0, rx1, ry1 = room.bounds
        near = {"west": c.x - rx0, "east": rx1 - c.x, "south": c.y - ry0, "north": ry1 - c.y}
        wall_side: Side = min(near, key=near.get)  # type: ignore[arg-type]
        facing: Side = {"south": "north", "north": "south", "west": "east", "east": "west"}[wall_side]  # type: ignore[assignment]
        counts[f.kind] = counts.get(f.kind, 0) + 1
        out.append(PlacedFixture(id=f"{rid}-{f.kind}-{counts[f.kind]}", kind=f.kind, room_id=rid,  # type: ignore[arg-type]
                                 rect=(round(x0, 3), round(y0, 3), round(x1, 3), round(y1, 3)), facing=facing, src=f.src))
    report.fixtures += len(out)
    return out


# ---------------------------------------------------------------- entry point


def import_dxf(data: bytes, unit: str | None = None) -> ImportResult:
    """A floor plan when the file has one (rooms, walls, doors, windows, fixtures, plus every other entity as
    drawing items); any other CAD file as a drawing of all its entities."""
    doc = read_doc(data)
    try:
        return _import_floorplan(doc, unit)
    except NoFloorPlan as why:
        return _import_drawing(doc, unit, str(why))


def _skipped_lines(skipped: dict[str, int]) -> list[str]:
    return [f"{n} x {what} (not imported)" for what, n in sorted(skipped.items())]


def _import_drawing(doc, unit: str | None, why: str) -> ImportResult:
    """No floor plan in the file: keep all its geometry as one read-only drawing plan."""
    items, skipped, truncated = dxf_drawing.collect_drawing(doc)
    box = dxf_drawing.bounds(items)
    if box is None:
        what = ", ".join(f"{n} {w}" for w, n in sorted(skipped.items())) or "nothing"
        raise EditError(f"the DXF has nothing that can be drawn ({why}); it holds {what}")
    extent = max(box[2] - box[0], box[3] - box[1])
    code = int(doc.header.get("$INSUNITS", 0) or 0)
    if unit:
        if unit not in UNIT_FACTORS:
            raise EditError(f"unknown unit {unit!r}; use one of {', '.join(UNIT_FACTORS)}")
        name, k, how = unit, UNIT_FACTORS[unit], "chosen by you"
    elif code in INSUNITS:
        name, k, how = {1: "in", 2: "ft", 4: "mm", 5: "cm", 6: "m", 10: "yd"}.get(code, "m"), INSUNITS[code], "the file's $INSUNITS"
    else:
        fits = [(n, f) for n, f in GUESS_ORDER if 3.0 <= extent * f <= 80.0]
        name, k = fits[0] if fits else ("m", 1.0)
        how = (f"guessed from the {extent:g}-unit extent (the file sets no $INSUNITS)" if fits
               else "assumed 1 unit = 1 m (the file sets no $INSUNITS and its size fits no unit)")
    dx, dy = -box[0] * k, -box[1] * k
    placed = dxf_drawing.scale_items(items, k, dx, dy)
    w, h = round((box[2] - box[0]) * k, 3), round((box[3] - box[1]) * k, 3)
    model = BuildingModel(footprint=(0.0, 0.0, w, h), lot=(round(w, 2), round(h, 2)), entrance_side="south",
                          source="imported", origin=Origin(scale=k, dx=dx, dy=dy), rooms=[], walls=[], openings=[],
                          drawing=placed)
    report = ImportReport(units=name, unit_source=how, layers_used=sorted({i.layer for i in placed}), plans_found=1,
                          skipped=_skipped_lines(skipped))
    report.warnings.append(f"no floor plan in the file ({why}): imported as a drawing of its {len(placed)} entities")
    if truncated:
        report.warnings.append(f"the drawing is too big: only the first {len(placed)} entities were kept")
    plan = ImportedPlan(index=1, name="Drawing", model=model, room_count=0, net_area=0.0)
    return ImportResult(plans=[plan], report=report)


def _attach_drawing(doc, raw: _Raw, plans: list[ImportedPlan], zones: list[Polygon], k: float, report: ImportReport) -> None:
    """Every entity the floor-plan reader did not use (not on a wall, door or window layer, not a fixture or
    opening block, not a room label) becomes a drawing item of the plan whose area holds it."""
    items, skipped, truncated = dxf_drawing.collect_drawing(doc)
    used_layers = set(raw.layers_used)
    used_src = {h for f in raw.fixtures for h in f.src} | {h for m in raw.markers for h in m.handles}
    used_src |= {h.split(":")[0] for p in plans for r in p.model.rooms for h in r.src}
    left = [i for i in items if i.layer not in used_layers and i.src not in used_src]
    metres = dxf_drawing.scale_items(left, k, 0.0, 0.0)
    buckets: list[list[DrawItem]] = [[] for _ in plans]
    for it in metres:  # the plan whose area is nearest (dimension rows, legends and title blocks lie outside it)
        pt = Point(it.at) if it.at else Point(*it.paths[0][0])
        buckets[min(range(len(zones)), key=lambda i: zones[i].distance(pt))].append(it)
    for plan, group in zip(plans, buckets):
        o = plan.model.origin
        plan.model.drawing = dxf_drawing.scale_items(group, 1.0, o.dx, o.dy) if o else group
        for n, it in enumerate(plan.model.drawing, 1):
            it.id = f"g{n}"
    for line in _skipped_lines(skipped):
        report.skipped.append(line)
    if truncated:
        report.warnings.append("the drawing is too big: only the first entities were kept")


def _import_floorplan(doc, unit: str | None = None) -> ImportResult:
    probe = collect(doc, 1.0)  # unit unknown yet: flatten finely enough for any unit but cheaply
    name, k, how = detect_units(doc, probe, unit)
    raw = _scaled(collect(doc, k), k) if k != 1.0 else collect(doc, 1.0)
    report = ImportReport(units=name, unit_source=how, layers_used=sorted(raw.layers_used),
                          layers_ignored=sorted(raw.layers_ignored))
    if not raw.walls:
        raise NoFloorPlan("no wall linework found: name the wall layer like 'A-WALL' or draw the walls as lines")

    thin0 = [f for f in _faces(raw.walls) if not _is_room(f)]
    t0 = _thin_thickness(unary_union(thin0)) if thin0 else DEFAULT_WALL
    # a jamb (a wall's end face) is about one wall thickness long; wall edges are longer
    faces = _faces(raw.walls + _door_bars(unary_union(thin0) if thin0 else Polygon(), raw.markers))
    thin = [f for f in faces if not _is_room(f)]
    big = [f for f in faces if _is_room(f)]
    if not big:
        raise NoFloorPlan("no closed rooms found: the wall lines do not enclose any space (check for gaps)")
    solids = unary_union(thin) if thin else Polygon()
    thin_area = solids.area if not solids.is_empty else 0.0
    centerline_mode = thin_area < 0.02 * sum(f.area for f in big)
    t_est = DEFAULT_WALL if centerline_mode else _thin_thickness(solids)
    if centerline_mode:
        report.warnings.append("the walls are single lines, not double lines: assumed a 0.2 m wall thickness")
        big = [f.buffer(-DEFAULT_WALL / 2, join_style="mitre") for f in big]
        big = [g for g in (b if isinstance(b, Polygon) else None for b in big) if g is not None and not g.is_empty]
        solids = unary_union([f.buffer(DEFAULT_WALL / 2, join_style="mitre").difference(f.buffer(-DEFAULT_WALL / 2, join_style="mitre")) for f in faces if _is_room(f)])
        solids = solids if not solids.is_empty else Polygon()

    doors = _place_markers(raw.markers, solids, t_est) if not solids.is_empty else []
    patches = [d.patch for d in doors if d.patch is not None]
    if patches:
        solids = unary_union([solids, *patches])
    # rooms = free space; doorway patches split the rooms they join
    free = unary_union(big)
    if patches:
        free = free.difference(unary_union(patches))
    pieces: list[Polygon] = []
    tiny = 0
    for g in (free.geoms if hasattr(free, "geoms") else [free]):
        if not isinstance(g, Polygon):
            continue
        opened = g.buffer(-0.04, join_style="mitre").buffer(0.04, join_style="mitre")
        for h in (opened.geoms if hasattr(opened, "geoms") else [opened]):
            if isinstance(h, Polygon) and _is_room(h):
                pieces.append(Polygon(h.exterior))  # columns inside a room are not separate spaces
            elif isinstance(h, Polygon) and h.area > 0.2:
                tiny += 1
    if tiny:
        raw.skipped[f"space under {MIN_ROOM_AREA} m² or narrower than {ROOM_INSCRIBED} m"] = tiny
    if not pieces:
        raise NoFloorPlan("no rooms found after separating the doorways")

    # one plan per connected cluster (a sheet may hold several plans)
    zone = unary_union([*(p.buffer(0.8) for p in pieces)])
    clusters = list(zone.geoms) if hasattr(zone, "geoms") else [zone]
    clusters.sort(key=lambda c: (-round(c.centroid.y / 5), c.centroid.x))
    plans: list[ImportedPlan] = []
    zones: list[Polygon] = []
    for i, zc in enumerate(clusters, 1):
        in_rooms = [p for p in pieces if zc.contains(p.centroid)]
        if not in_rooms:
            continue
        sol = solids.intersection(zc) if not solids.is_empty else Polygon()
        ds = [d for d in doors if zc.contains(Point(d.center))]
        txt = [(t, x, y, h) for t, x, y, h in raw.texts if zc.contains(Point(x, y))]
        fx = [f for f in raw.fixtures if zc.contains(f.geom.centroid)]
        plan = _build_plan(len(plans) + 1, in_rooms, sol, ds, txt, t_est, report, fx, k)
        if plan is not None:
            plans.append(plan)
            zones.append(zc)
    report.plans_found = len(plans)
    stray = [f for f in raw.fixtures if not any(Polygon(zc).contains(f.geom.centroid) for zc in clusters)]
    if stray:
        report.skipped.append(f"{len(stray)} fixture block(s) outside every plan")
    for what, n in sorted(raw.skipped.items()):
        report.skipped.append(f"{n} x {what} (not imported)")
    if not plans:
        raise NoFloorPlan("no plan could be built from the file")
    _attach_drawing(doc, raw, plans, zones, k, report)
    if not raw.markers:
        report.warnings.append("no door or window symbols found (expected blocks on layers like A-DOOR / A-GLAZ): rooms are not connected")
    return ImportResult(plans=plans, report=report)
