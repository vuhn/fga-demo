"""BuildingModel -> one ezdxf document -> DXF / SVG / PDF.

Every output is rendered from the same ezdxf document, so the SVG preview and
the PDF are exactly what the DXF contains. Model units are metres. Plans are drawn at
1:50 on the smallest ISO A sheet (landscape, A3 up to A0) that fits the house and its
dimension chains; only a house too big for A0 at 1:50 drops to a smaller scale.
"""

from __future__ import annotations

import io
import math
import re
from dataclasses import dataclass
from datetime import date

import ezdxf
from ezdxf.addons.drawing import Frontend, RenderContext, layout, svg
from ezdxf.addons.drawing.config import BackgroundPolicy, ColorPolicy, Configuration
from ezdxf.document import Drawing
from shapely.geometry import LineString, MultiPolygon, Polygon, box
from shapely.ops import unary_union

from .schemas import BuildingModel, Opening, PlacedFixture, Wall

PAPERS: dict[str, tuple[float, float]] = {  # landscape, metres of paper
    "A3": (0.420, 0.297),
    "A2": (0.594, 0.420),
    "A1": (0.841, 0.594),
    "A0": (1.189, 0.841),
}
SCALES = (50, 100, 200, 500)
# AIA-style layer names: name -> (aci colour, lineweight in 1/100 mm, description)
LAYERS = {
    "C-PROP": (3, 18, "Lot boundary"),
    "A-WALL-EXTR": (7, 50, "Exterior wall outlines"),
    "A-WALL-INTR": (7, 35, "Interior wall outlines"),
    "A-WALL-PATT": (8, 13, "Wall poche (solid fill)"),
    "A-DOOR": (7, 25, "Door leaves"),
    "A-DOOR-SWNG": (7, 13, "Door swing arcs"),
    "A-GLAZ": (5, 18, "Windows"),
    "A-AREA-BNDY": (9, 13, "Room boundaries (net floor area, closed polylines)"),
    "A-AREA-IDEN": (7, 18, "Room names and areas"),
    "A-FURN-BEDS": (8, 13, "Beds"),
    "A-FURN-STOR": (8, 13, "Wardrobes and shelves"),
    "A-FURN-DESK": (8, 13, "Desks and chairs"),
    "A-FURN-SEAT": (8, 13, "Sofas"),
    "A-FURN-TABL": (8, 13, "Dining tables and chairs"),
    "A-FURN-EQPM": (8, 13, "TV units"),
    "A-FLOR-CASE": (8, 13, "Kitchen counters"),
    "A-EQPM-APPL": (8, 13, "Appliances (stove, fridge, washer)"),
    "P-FIXT-WC": (4, 13, "Toilets"),
    "P-FIXT-BASN": (4, 13, "Basins and sinks"),
    "P-FIXT-BATH": (4, 13, "Showers and bathtubs"),
    "A-ANNO-DIMS": (8, 13, "Room dimension chains"),
    "A-ANNO-DIMS-OVAL": (8, 13, "Overall dimensions"),
    "A-ANNO-NARW": (7, 18, "North arrow"),
    "A-ANNO-BORD": (7, 25, "Sheet border"),
    "A-ANNO-TTLB": (7, 25, "Title block"),
}
FIXTURE_LAYERS = {
    "bed": "A-FURN-BEDS", "wardrobe": "A-FURN-STOR", "shelves": "A-FURN-STOR", "desk": "A-FURN-DESK",
    "sofa": "A-FURN-SEAT", "dining_table": "A-FURN-TABL", "tv_unit": "A-FURN-EQPM",
    "kitchen_counter": "A-FLOR-CASE", "stove": "A-EQPM-APPL", "fridge": "A-EQPM-APPL", "washer": "A-EQPM-APPL",
    "toilet": "P-FIXT-WC", "basin": "P-FIXT-BASN", "sink": "P-FIXT-BASN",
    "shower": "P-FIXT-BATH", "bathtub": "P-FIXT-BATH",
}


@dataclass
class TitleBlock:
    project: str = "Single-storey house"
    sheet: str = "A-101"
    sheet_title: str = "Ground floor plan"
    version: int = 1
    issued: str = ""
    author: str = ""


@dataclass(frozen=True)
class Sheet:
    """The printed sheet: its world rectangle, scale denominator and paper."""

    x0: float
    y0: float
    x1: float
    y1: float
    denom: int
    paper: str

    @property
    def paper_mm(self) -> tuple[float, float]:
        w, h = PAPERS[self.paper]
        return (w * 1000, h * 1000)


def _fits(w: float, d: float, denom: int, paper: str) -> bool:
    pw, ph = PAPERS[paper]
    sw, sh = pw * denom, ph * denom
    gap = 0.025 * denom  # frame margins plus breathing room (dimension rows are already in w, d)
    return w <= sw - gap and d <= sh - 0.045 * denom - gap


def _scale_for(model: BuildingModel) -> tuple[int, str]:
    """1:50 on the smallest paper that fits the house and its dimension chains (the lot is context
    only); a smaller scale only when A0 is not enough."""
    fx0, fy0, fx1, fy1 = model.footprint
    for denom in SCALES:
        margin = 2 * 2.2 * denom / 100  # two dimension rows each side
        for paper in PAPERS:
            if _fits(fx1 - fx0 + margin, fy1 - fy0 + margin, denom, paper):
                return denom, paper
    return SCALES[-1], "A0"


def _block_name(*parts: str) -> str:
    return re.sub(r"[^A-Za-z0-9_$-]", "_", "-".join(parts))[:80]


def _new_block(doc: Drawing, name: str, center: tuple[float, float], attribs: dict[str, str]):
    """A block whose geometry is relative to `center`; the caller inserts it there. Attributes are
    invisible ATTDEFs (CAD shows them in the properties palette; BIM-ish tools read them)."""
    blk = doc.blocks.new(_unique(doc, name), base_point=(0, 0))
    for i, (tag, _) in enumerate(attribs.items()):
        blk.add_attdef(tag, (0, 0), height=0.1, dxfattribs={"flags": 1})  # 1 = invisible
    return blk


def _unique(doc: Drawing, name: str) -> str:
    out, n = name, 1
    while out in doc.blocks:
        n += 1
        out = f"{name}-{n}"
    return out


def _insert(msp, blk, center: tuple[float, float], layer: str, attribs: dict[str, str]):
    ref = msp.add_blockref(blk.name, center, dxfattribs={"layer": layer})
    ref.add_auto_attribs(attribs)
    return ref


def _wall_poly(w: Wall):
    return LineString([w.start, w.end]).buffer(w.thickness / 2, cap_style="square", join_style="mitre")


def _opening_cut(o: Opening, w: Wall):
    (x0, y0), (x1, y1) = w.start, w.end
    horizontal = abs(y1 - y0) < 1e-9
    cx, cy = o.center
    h = o.width / 2
    t = w.thickness / 2 + 0.01
    return box(cx - h, cy - t, cx + h, cy + t) if horizontal else box(cx - t, cy - h, cx + t, cy + h)


def _polys(geom) -> list[Polygon]:
    if isinstance(geom, Polygon):
        return [geom]
    if isinstance(geom, MultiPolygon):
        return list(geom.geoms)
    return [g for g in getattr(geom, "geoms", []) if isinstance(g, Polygon)]


def build_dxf(model: BuildingModel, tb: TitleBlock | None = None) -> tuple[Drawing, Sheet]:
    tb = tb or TitleBlock()
    denom, paper = _scale_for(model)
    k = denom / 100  # annotation sizes are tuned for 1:100

    doc = ezdxf.new("R2010", setup=True, units=ezdxf.units.M)
    for name, (color, lw, desc) in LAYERS.items():
        layer = doc.layers.add(name, color=color, lineweight=lw)
        layer.description = desc
    msp = doc.modelspace()
    walls = {w.id: w for w in model.walls}
    rooms = {r.id: r for r in model.rooms}

    # lot boundary, only when the whole lot fits on the sheet at this scale
    lw_, ld_ = model.lot
    fx0, fy0, fx1, fy1 = model.footprint
    show_lot = _fits(lw_, ld_, denom, paper)
    sheet_cx, sheet_cy = ((lw_ / 2, ld_ / 2) if show_lot else ((fx0 + fx1) / 2, (fy0 + fy1) / 2))
    if show_lot:
        msp.add_lwpolyline([(0, 0), (lw_, 0), (lw_, ld_), (0, ld_)], close=True,
                           dxfattribs={"layer": "C-PROP", "linetype": "DASHED", "ltscale": 0.5 * k})

    # walls: thick centerline segments minus openings, solid hatched; exterior and interior on their
    # own layers (interior solids stop at the exterior so the two outlines never overlap)
    cuts = unary_union([_opening_cut(o, walls[o.wall_id]) for o in model.openings])
    ext_raw = unary_union([_wall_poly(w) for w in model.walls if w.kind == "exterior"])
    int_raw = unary_union([_wall_poly(w) for w in model.walls if w.kind == "interior"])
    for geom, layer in ((ext_raw.difference(cuts), "A-WALL-EXTR"),
                        (int_raw.difference(ext_raw).difference(cuts), "A-WALL-INTR")):
        for poly in _polys(geom):
            hatch = msp.add_hatch(color=7, dxfattribs={"layer": "A-WALL-PATT"})
            hatch.paths.add_polyline_path(list(poly.exterior.coords)[:-1], is_closed=True, flags=1)
            for ring in poly.interiors:
                hatch.paths.add_polyline_path(list(ring.coords)[:-1], is_closed=True, flags=16)
            msp.add_lwpolyline(list(poly.exterior.coords), close=True, dxfattribs={"layer": layer})
            for ring in poly.interiors:
                msp.add_lwpolyline(list(ring.coords), close=True, dxfattribs={"layer": layer})

    # doors and windows: one named block reference each (select, move, delete as a unit)
    for o in model.openings:
        w = walls[o.wall_id]
        (x0, y0), (x1, y1) = w.start, w.end
        L = math.hypot(x1 - x0, y1 - y0)
        ux, uy = (x1 - x0) / L, (y1 - y0) / L  # along the wall
        cx, cy = o.center
        h = o.width / 2
        t = w.thickness / 2
        if o.kind == "open":  # open plan: the wall is cut away, nothing to draw
            continue
        names = [rooms[r].name if r in rooms else r for r in o.rooms]
        attribs = {"ID": o.id, "KIND": o.kind, "WIDTH": f"{o.width:.2f}", "CONNECTS": " / ".join(names)}
        blk = _new_block(doc, _block_name(o.kind, o.id), (cx, cy), attribs)

        def rel(x: float, y: float) -> tuple[float, float]:
            return (x - cx, y - cy)

        if o.kind == "window":
            for off in (-t, 0.0, t):
                nx, ny = -uy * off, ux * off
                blk.add_line(rel(cx - ux * h + nx, cy - uy * h + ny), rel(cx + ux * h + nx, cy + uy * h + ny),
                             dxfattribs={"layer": "A-GLAZ"})
            _insert(msp, blk, (cx, cy), "A-GLAZ", attribs)
            continue
        # swing towards the room the leaf opens into
        target = rooms.get(o.swing_into or "")
        nx, ny = -uy, ux
        if target is not None:
            rx = (target.rect[0] + target.rect[2]) / 2
            ry = (target.rect[1] + target.rect[3]) / 2
            if (rx - cx) * nx + (ry - cy) * ny < 0:
                nx, ny = -nx, -ny
        hx, hy = cx - ux * h + nx * t, cy - uy * h + ny * t  # hinge on the room face
        blk.add_line(rel(hx, hy), rel(hx + nx * o.width, hy + ny * o.width), dxfattribs={"layer": "A-DOOR"})
        a_along = math.degrees(math.atan2(uy, ux))
        a_leaf = math.degrees(math.atan2(ny, nx))
        start, end = (a_along, a_leaf) if (a_leaf - a_along) % 360 == 90 else (a_leaf, a_along)
        blk.add_arc(rel(hx, hy), o.width, start, end, dxfattribs={"layer": "A-DOOR-SWNG"})
        _insert(msp, blk, (cx, cy), "A-DOOR", attribs)

    for f in model.fixtures:
        _draw_fixture(doc, msp, f, rooms[f.room_id].name if f.room_id in rooms else f.room_id)

    # room boundaries (net floor area): one closed polyline per room, what area take-offs read
    for r in model.rooms:
        x0, y0, x1, y1 = r.net_rect
        msp.add_lwpolyline([(x0, y0), (x1, y0), (x1, y1), (x0, y1)], close=True, dxfattribs={"layer": "A-AREA-BNDY"})

    # room labels, moved off the fixtures where the room has space
    for r in model.rooms:
        x0, y0, x1, y1 = r.net_rect
        text = r.name if r.type == "corridor" else f"{r.name}\\P{r.net_area:.1f} m²"
        fit = (x1 - x0) * 0.9 / (0.6 * max(len(r.name), 6))  # rough glyph width 0.6 x height
        h = min(0.22 * k, fit)
        mt = msp.add_mtext(text, dxfattribs={"layer": "A-AREA-IDEN", "char_height": h, "style": "OpenSans"})
        mt.set_location(_label_spot(r.net_rect, 0.6 * h * max(len(r.name), 7), 2.6 * h,
                                    [f.rect for f in model.fixtures if f.room_id == r.id]), attachment_point=5)

    # dimensions to wall centerlines: room chains on all four sides, overall on south and west
    dim_over = {"dimlfac": 1.0, "dimtxt": 0.18 * k, "dimasz": 0.1 * k, "dimtsz": 0.08 * k, "dimexo": 0.1 * k,
                "dimexe": 0.08 * k, "dimgap": 0.05 * k, "dimdec": 2, "dimtad": 1}

    def chain(coords: list[float], fixed: float, horizontal: bool, offset: float, layer: str = "A-ANNO-DIMS") -> None:
        cs = sorted({round(c, 3) for c in coords})
        for a, b in zip(cs, cs[1:]):
            if horizontal:
                d = msp.add_linear_dim(base=(a, fixed + offset), p1=(a, fixed), p2=(b, fixed), angle=0,
                                       dimstyle="EZDXF", override=dim_over, dxfattribs={"layer": layer})
            else:
                d = msp.add_linear_dim(base=(fixed + offset, a), p1=(fixed, a), p2=(fixed, b), angle=90,
                                       dimstyle="EZDXF", override=dim_over, dxfattribs={"layer": layer})
            d.render()

    def touching(side: str) -> list[float]:
        out: list[float] = []
        for r in model.rooms:
            x0, y0, x1, y1 = r.rect
            if side == "south" and abs(y0 - fy0) < 1e-6 or side == "north" and abs(y1 - fy1) < 1e-6:
                out += [x0, x1]
            if side == "west" and abs(x0 - fx0) < 1e-6 or side == "east" and abs(x1 - fx1) < 1e-6:
                out += [y0, y1]
        return out

    step = 0.9 * k
    chain(touching("south"), fy0, True, -step)
    chain([fx0, fx1], fy0, True, -2 * step, "A-ANNO-DIMS-OVAL")
    chain(touching("north"), fy1, True, step)
    chain(touching("west"), fx0, False, -step)
    chain([fy0, fy1], fx0, False, -2 * step, "A-ANNO-DIMS-OVAL")
    chain(touching("east"), fx1, False, step)

    # sheet frame, north arrow, title block
    sw, sh = PAPERS[paper][0] * denom, PAPERS[paper][1] * denom
    strip = 0.045 * denom
    m = 0.01 * denom
    ox = sheet_cx - sw / 2
    oy = sheet_cy - strip / 2 - sh / 2  # centre the plan in the area above the strip
    sheet = Sheet(ox, oy, ox + sw, oy + sh, denom, paper)
    ttl = {"layer": "A-ANNO-TTLB"}
    msp.add_lwpolyline([(ox + m, oy + m), (ox + sw - m, oy + m), (ox + sw - m, oy + sh - m), (ox + m, oy + sh - m)],
                       close=True, dxfattribs={"layer": "A-ANNO-BORD"})
    msp.add_line((ox + m, oy + m + strip), (ox + sw - m, oy + m + strip), dxfattribs=ttl)

    nax, nay = ox + sw - m - 0.02 * denom, oy + m + strip * 0.42  # in the title strip
    r_ = 0.0075 * denom
    nar = {"layer": "A-ANNO-NARW"}
    msp.add_circle((nax, nay), r_, dxfattribs=nar)
    msp.add_lwpolyline([(nax, nay + r_), (nax - r_ * 0.4, nay - r_ * 0.6), (nax, nay - r_ * 0.2),
                        (nax + r_ * 0.4, nay - r_ * 0.6)], close=True, dxfattribs=nar)
    msp.add_text("N", height=0.004 * denom, dxfattribs=nar).set_placement(
        (nax, nay + r_ * 1.3), align=ezdxf.enums.TextEntityAlignment.BOTTOM_CENTER)

    cols = [
        (0.0, [tb.project, f"{tb.sheet}  {tb.sheet_title}"]),
        (0.45, [f"Scale 1:{denom} @ {paper}   Version v{tb.version}",
                f"Issued {tb.issued or date.today().isoformat()}" + (f"   By {tb.author}" if tb.author else "")]),
        (0.68, ["Dimensions in metres to wall centerlines",
                "Text2Blueprint POC - not for construction"]),
    ]
    inner_w = sw - 2 * m
    for frac, lines in cols:
        x = ox + m + 0.006 * denom + frac * inner_w
        for i, line in enumerate(lines):
            hgt = (0.0045 if i == 0 and frac == 0.0 else 0.003) * denom
            y = oy + m + strip - (0.014 + i * 0.016) * denom
            msp.add_text(line, height=hgt, dxfattribs={**ttl, "style": "OpenSans"}).set_placement((x, y))
        if frac > 0:
            fx = ox + m + frac * inner_w
            msp.add_line((fx, oy + m), (fx, oy + m + strip), dxfattribs=ttl)

    return doc, sheet


def _label_spot(net, w: float, h: float, bodies: list) -> tuple[float, float]:
    """Room centre, or the first spot on a coarse grid whose label box misses every fixture."""
    x0, y0, x1, y1 = net
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
    spots = [(cx, cy)] + sorted(
        ((x0 + (x1 - x0) * i / 6, y0 + (y1 - y0) * j / 6) for i in range(1, 6) for j in range(1, 6)),
        key=lambda p: (p[0] - cx) ** 2 + (p[1] - cy) ** 2,
    )
    def covered(px: float, py: float) -> float:
        lb = (px - w / 2, py - h / 2, px + w / 2, py + h / 2)
        return sum(max(0.0, min(lb[2], b[2]) - max(lb[0], b[0])) * max(0.0, min(lb[3], b[3]) - max(lb[1], b[1]))
                   for b in bodies)

    best, best_cov = (cx, cy), covered(cx, cy)
    for px, py in spots:
        if px - w / 2 < x0 or px + w / 2 > x1 or py - h / 2 < y0 or py + h / 2 > y1:
            continue
        cov = covered(px, py)
        if cov < best_cov - 1e-9:  # spots are nearest-first, so ties keep the more central one
            best, best_cov = (px, py), cov
        if cov == 0:
            break
    return best


def _draw_fixture(doc: Drawing, msp, f: PlacedFixture, room_name: str) -> None:
    """Plan symbol in the fixture's own frame: u along its wall, v out from the wall. The symbol is
    a named block (`<kind>-<id>`) inserted at the fixture's centre, so each table, bed or toilet is
    one selectable object with ROOM/KIND attributes."""
    x0, y0, x1, y1 = f.rect
    ccx, ccy = (x0 + x1) / 2, (y0 + y1) / 2
    attribs = {"ID": f.id, "KIND": f.kind, "ROOM": room_name}
    blk = _new_block(doc, _block_name(f.kind, room_name, f.id), (ccx, ccy), attribs)
    if f.facing in ("north", "south"):
        w, d = x1 - x0, y1 - y0
    else:
        w, d = y1 - y0, x1 - x0

    def world(u: float, v: float) -> tuple[float, float]:
        if f.facing == "north":
            return (x0 + u, y0 + v)
        if f.facing == "south":
            return (x1 - u, y1 - v)
        if f.facing == "east":
            return (x0 + v, y1 - u)
        return (x1 - v, y0 + u)  # west

    def pt(u: float, v: float) -> tuple[float, float]:
        wx, wy = world(u, v)
        return (wx - ccx, wy - ccy)  # block geometry is relative to the insertion point

    at = {"layer": "0"}  # block content takes the layer of its INSERT

    def rect(u0: float, v0: float, u1: float, v1: float) -> None:
        blk.add_lwpolyline([pt(u0, v0), pt(u1, v0), pt(u1, v1), pt(u0, v1)], close=True, dxfattribs=at)

    def line(u0: float, v0: float, u1: float, v1: float) -> None:
        blk.add_line(pt(u0, v0), pt(u1, v1), dxfattribs=at)

    def circle(u: float, v: float, r: float) -> None:
        blk.add_circle(pt(u, v), r, dxfattribs=at)

    def ellipse(u: float, v: float, ru: float, rv: float) -> None:
        cx, cy = pt(u, v)
        ax, ay = pt(u, v + rv) if rv >= ru else pt(u + ru, v)
        blk.add_ellipse((cx, cy), major_axis=(ax - cx, ay - cy), ratio=min(ru, rv) / max(ru, rv), dxfattribs=at)

    kind = f.kind
    if kind == "toilet":
        rect(w / 2 - 0.2, 0, w / 2 + 0.2, 0.18)
        ellipse(w / 2, 0.18 + 0.25, 0.18, 0.25)
        _insert(msp, blk, (ccx, ccy), FIXTURE_LAYERS[kind], attribs)
        return
    rect(0, 0, w, d)
    if kind == "bed":
        pillows = 2 if w >= 1.3 else 1
        pw = (w - 0.1 * (pillows + 1)) / pillows
        for i in range(pillows):
            u = 0.1 + i * (pw + 0.1)
            rect(u, 0.08, u + pw, 0.35)
        line(0, 0.55, w, 0.55)
    elif kind == "wardrobe":
        line(0.05, d / 2, w - 0.05, d / 2)
        line(0, d, w / 2, 0)
        line(w / 2, 0, w, d)
    elif kind == "desk":
        rect(w / 2 - 0.22, d + 0.08, w / 2 + 0.22, d + 0.48)
    elif kind == "shelves":
        line(0, 0, w, d)
    elif kind == "sofa":
        line(0.2, 0.25, w - 0.2, 0.25)
        line(0.2, 0, 0.2, d)
        line(w - 0.2, 0, w - 0.2, d)
    elif kind == "tv_unit":
        rect(0.15, 0.05, w - 0.15, 0.12)
    elif kind == "dining_table":
        chairs = max(1, int(w / 0.6))
        gap = w / chairs
        for i in range(chairs):
            u = gap * (i + 0.5)
            rect(u - 0.2, -0.45, u + 0.2, -0.05)
            rect(u - 0.2, d + 0.05, u + 0.2, d + 0.45)
    elif kind == "kitchen_counter":
        line(0, d - 0.05, w, d - 0.05)
    elif kind == "sink":
        rect(0.08, 0.08, w - 0.08, d - 0.08)
        circle(w / 2, d / 2, 0.03)
    elif kind == "stove":
        for u in (w * 0.28, w * 0.72):
            for v in (d * 0.28, d * 0.72):
                circle(u, v, min(w, d) * 0.15)
    elif kind == "fridge":
        line(0, 0, w, d)
        line(0, d, w, 0)
    elif kind == "basin":
        ellipse(w / 2, d / 2, w * 0.35, d * 0.35)
    elif kind == "shower":
        line(0, 0, w, d)
        line(0, d, w, 0)
        circle(w / 2, d / 2, 0.04)
    elif kind == "bathtub":
        rect(0.08, 0.08, w - 0.08, d - 0.08)
        circle(0.25, d / 2, 0.03)
    elif kind == "washer":
        circle(w / 2, d / 2, min(w, d) * 0.35)
    _insert(msp, blk, (ccx, ccy), FIXTURE_LAYERS[kind], attribs)


def _config() -> Configuration:
    return Configuration(background_policy=BackgroundPolicy.WHITE, color_policy=ColorPolicy.BLACK)


def to_dxf_bytes(doc: Drawing) -> bytes:
    stream = io.StringIO()
    doc.write(stream)
    return stream.getvalue().encode("utf-8")


def to_svg(doc: Drawing, sheet: Sheet) -> str:
    from ezdxf.math import BoundingBox2d

    backend = svg.SVGBackend()
    Frontend(RenderContext(doc), backend, config=_config()).draw_layout(doc.modelspace())
    page = layout.Page(*sheet.paper_mm, layout.Units.mm)
    settings = layout.Settings(fit_page=False, scale=1000 / sheet.denom, min_stroke_width=0.05, max_stroke_width=0.004)
    box2d = BoundingBox2d([(sheet.x0, sheet.y0), (sheet.x1, sheet.y1)])
    return backend.get_string(page, settings=settings, render_box=box2d)


def to_pdf(doc: Drawing, sheet: Sheet) -> bytes:
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from ezdxf.addons.drawing.matplotlib import MatplotlibBackend

    pw, ph = sheet.paper_mm
    fig = plt.figure(figsize=(pw / 25.4, ph / 25.4))
    ax = fig.add_axes((0, 0, 1, 1))
    ax.set_axis_off()
    Frontend(RenderContext(doc), MatplotlibBackend(ax), config=_config()).draw_layout(doc.modelspace(), finalize=False)
    ax.set_autoscale_on(False)
    ax.set_aspect("equal", adjustable="box")
    ax.set_xlim(sheet.x0, sheet.x1)
    ax.set_ylim(sheet.y0, sheet.y1)
    buf = io.BytesIO()
    fig.savefig(buf, format="pdf")
    plt.close(fig)
    return buf.getvalue()
