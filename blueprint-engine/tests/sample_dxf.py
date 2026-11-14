"""Authors the three reference DXFs of the sample sheet ("Architectural Apartment Floor Plan Collection").

The sheet is a PNG, so these plans are rebuilt from its printed dimension chains (millimetres): an L-shaped
2-bed with two baths and a balcony (11.2 x 9.8 m), a 1-bed with a notched outline (8.9 x 6.2 m) and a 2-bed
with a chamfered, angled bay (13.6 x 7.5 m). Living, kitchen and dining are one open space in all three.
They are drawn the way CAD exports are: double-line walls with closed jambs on A-WALL, door blocks (leaf and
swing) on A-DOOR, window blocks on A-GLAZ, labels as TEXT, DIMENSION entities, plus furniture and a title that
must be ignored. The expected rooms are computed from the same polygons, so the tests are known-answer.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field

import ezdxf
from shapely import affinity
from shapely.geometry import LineString, Polygon, box
from shapely.ops import unary_union

INNER = 0.10  # half of an interior wall
OUTER = 0.15  # exterior wall thickness added outside the centerline region


@dataclass
class Spec:
    name: str
    outline: list[tuple[float, float]]
    rooms: dict[str, list[tuple[float, float]]]  # centerline polygons; the open space is the rest
    open_space: str
    doors: list[tuple[float, float, float, str]]  # x, y, width, axis h|v
    windows: list[tuple[float, float, float, float]]  # x, y, width, angle (deg)
    labels: dict[str, list[tuple[str, float, float]]]  # room -> [(text, x, y)]
    dims: list[tuple[tuple[float, float], tuple[float, float], str]] = field(default_factory=list)
    offset: tuple[float, float] = (0.0, 0.0)


def _poly(pts):
    return Polygon(pts)


def plan1() -> Spec:
    outline = [(0, 0), (2.5, 0), (2.5, 1.4), (8.0, 1.4), (8.0, 3.6), (11.2, 3.6), (11.2, 6.0), (10.2, 6.0),
               (10.2, 9.8), (0, 9.8)]
    rooms = {
        "balcony": [(0, 8.2), (3.5, 8.2), (3.5, 9.8), (0, 9.8)],
        "bed1": [(3.5, 6.0), (6.8, 6.0), (6.8, 9.8), (3.5, 9.8)],
        "bed2": [(6.8, 6.0), (10.2, 6.0), (10.2, 9.8), (6.8, 9.8)],
        "bath1": [(8.4, 3.6), (11.2, 3.6), (11.2, 6.0), (8.4, 6.0)],
        "bath2": [(6.0, 1.4), (8.0, 1.4), (8.0, 4.2), (6.0, 4.2)],
    }
    return Spec(
        "plan1", outline, rooms, "open",
        doors=[(4.7, 6.0, 0.9, "h"), (7.6, 6.0, 0.9, "h"), (8.4, 4.8, 0.9, "v"), (6.8, 4.2, 0.9, "h"),
               (2.6, 8.2, 0.9, "h"), (0.0, 1.2, 1.0, "v")],
        windows=[(5.1, 9.8, 1.5, 0), (8.5, 9.8, 1.5, 0), (0.0, 5.0, 1.8, 90), (4.2, 1.4, 1.5, 0)],
        labels={
            "balcony": [("BALCONY", 1.2, 9.0)], "bed1": [("BEDROOM 1", 4.6, 8.0)], "bed2": [("BEDROOM 2", 8.0, 8.0)],
            "bath1": [("BATH", 9.6, 4.8)], "bath2": [("WC", 6.6, 2.8)],
            "open": [("LIVING", 1.5, 5.5), ("DINING", 1.5, 3.2), ("KITCHEN", 4.2, 1.9)],
        },
        dims=[((0, 0), (11.2, 0), "11200"), ((0, 0), (0, 9.8), "9800")],
    )


def plan2() -> Spec:
    outline = [(2.3, 0), (8.9, 0), (8.9, 6.2), (0, 6.2), (0, 2.8), (2.3, 2.8)]
    rooms = {
        "balcony": [(0, 4.6), (3.6, 4.6), (3.6, 6.2), (0, 6.2)],
        "bed": [(3.6, 3.4), (6.6, 3.4), (6.6, 6.2), (3.6, 6.2)],
        "bath": [(3.5, 0), (5.4, 0), (5.4, 2.4), (3.5, 2.4)],
    }
    return Spec(
        "plan2", outline, rooms, "open",
        doors=[(2.2, 4.6, 0.9, "h"), (4.8, 3.4, 0.9, "h"), (4.4, 2.4, 0.8, "h"), (8.9, 1.2, 1.0, "v")],
        windows=[(5.1, 6.2, 1.5, 0), (0.0, 3.7, 1.2, 90), (7.0, 0.0, 1.5, 0)],
        labels={"balcony": [("BALCONY", 1.8, 5.4)], "bed": [("BEDROOM", 5.0, 5.0)], "bath": [("WC", 4.4, 1.2)],
                "open": [("LIVING", 1.2, 3.3), ("KITCHEN", 7.8, 4.5)]},
        dims=[((0, 0), (8.9, 0), "8900")],
        offset=(14.0, 4.0),
    )


def plan3() -> Spec:
    outline = [(0, 7.5), (13.6, 7.5), (13.6, 3.0), (11.6, 3.0), (11.6, 2.0), (9.2, 2.0), (9.2, 0.0), (4.9, 0.0),
               (4.9, 0.8), (1.9, 0.8), (0, 3.7)]
    rooms = {
        "bed3": [(7.1, 4.7), (10.1, 4.7), (10.1, 7.5), (7.1, 7.5)],
        "bed4": [(10.1, 4.7), (13.6, 4.7), (13.6, 7.5), (10.1, 7.5)],
        "bath3": [(4.9, 0.0), (6.8, 0.0), (6.8, 2.2), (4.9, 2.2)],
        "bath4": [(6.8, 0.0), (9.2, 0.0), (9.2, 2.2), (6.8, 2.2)],
    }
    return Spec(
        "plan3", outline, rooms, "open",
        doors=[(8.6, 4.7, 0.9, "h"), (11.85, 4.7, 0.9, "h"), (5.8, 2.2, 0.8, "h"), (8.0, 2.2, 0.8, "h"),
               (13.6, 3.8, 1.0, "v")],
        windows=[(8.6, 7.5, 1.5, 0), (11.85, 7.5, 1.5, 0), (4.0, 7.5, 2.4, 0), (0.95, 2.25, 1.5, 123.0)],
        labels={"bed3": [("PHÒNG NGỦ 1", 8.0, 6.3)], "bed4": [("PHÒNG NGỦ 2", 11.0, 6.3)],
                "bath3": [("WC", 5.4, 1.0)], "bath4": [("VỆ SINH", 7.4, 1.0)],
                "open": [("PHÒNG KHÁCH", 3.0, 5.0), ("BẾP", 3.0, 7.0), ("PHÒNG ĂN", 6.0, 4.0)]},
        dims=[((0, 0), (13.6, 0), "13600")],
        offset=(4.0, -12.0),
    )


@dataclass
class Built:
    spec: Spec
    nets: dict[str, Polygon]  # expected net rooms (metres, drawing placement removed)


def _geometry(spec: Spec):
    centers = {n: _poly(p) for n, p in spec.rooms.items()}
    outline = _poly(spec.outline)
    centers[spec.open_space] = outline.difference(unary_union(list(centers.values())))
    nets = {n: g.buffer(-INNER, join_style="mitre") for n, g in centers.items()}
    walls = outline.buffer(OUTER, join_style="mitre").difference(unary_union(list(nets.values())))
    return centers, nets, walls


def _door_cut(x, y, w, axis):
    return box(x - w / 2, y - 0.6, x + w / 2, y + 0.6) if axis == "h" else box(x - 0.6, y - w / 2, x + 0.6, y + w / 2)


def build(specs: list[Spec], units: str = "mm", set_units: bool = True) -> tuple[bytes, list[Built]]:
    k = {"mm": 1000.0, "m": 1.0}[units]
    doc = ezdxf.new("R2010", setup=True)
    doc.header["$INSUNITS"] = 0  # unitless unless asked (ezdxf's default header says metres)
    if set_units:
        doc.units = ezdxf.units.MM if units == "mm" else ezdxf.units.M
    for name, color in (("A-WALL", 7), ("A-DOOR", 3), ("A-GLAZ", 5), ("A-ANNO-TEXT", 2), ("A-FURN", 8),
                        ("A-ANNO-DIMS", 1), ("A-ANNO-TTLB", 7)):
        doc.layers.add(name, color=color)
    msp = doc.modelspace()
    built: list[Built] = []

    def pt(x, y, off):
        return ((x + off[0]) * k, (y + off[1]) * k)

    for spec in specs:
        off = spec.offset
        centers, nets, walls = _geometry(spec)
        # doorways cut through the wall (jambs stay as closed ring edges)
        cut = unary_union([_door_cut(x, y, w, a) for x, y, w, a in spec.doors])
        solid = walls.difference(cut)
        for poly in (solid.geoms if hasattr(solid, "geoms") else [solid]):
            for ring in [poly.exterior, *poly.interiors]:
                msp.add_lwpolyline([pt(x, y, off) for x, y in ring.coords], close=True, dxfattribs={"layer": "A-WALL"})
        # door blocks: leaf + swing arc
        for i, (x, y, w, axis) in enumerate(spec.doors):
            name = f"DOOR_{spec.name}_{i}"
            blk = doc.blocks.new(name)
            if axis == "h":
                hx, hy = -w / 2, 0.0
                blk.add_line((hx * k, hy), (hx * k, w * k), dxfattribs={"layer": "0"})
                blk.add_arc((hx * k, hy), w * k, 0, 90, dxfattribs={"layer": "0"})
            else:
                hx, hy = 0.0, -w / 2
                blk.add_line((hx, hy * k), (w * k, hy * k), dxfattribs={"layer": "0"})
                blk.add_arc((hx, hy * k), w * k, 0, 90, dxfattribs={"layer": "0"})
            msp.add_blockref(name, ((x + off[0]) * k, (y + off[1]) * k), dxfattribs={"layer": "A-DOOR"})
        for i, (x, y, w, ang) in enumerate(spec.windows):
            name = f"WINDOW_{spec.name}_{i}"
            blk = doc.blocks.new(name)
            for t in (-0.1, 0.0, 0.1):
                blk.add_line((-w / 2 * k, t * k), (w / 2 * k, t * k), dxfattribs={"layer": "0"})
            msp.add_blockref(name, ((x + off[0]) * k, (y + off[1]) * k),
                             dxfattribs={"layer": "A-GLAZ", "rotation": ang})
        for room, items in spec.labels.items():
            for text, x, y in items:
                msp.add_text(text, height=0.2 * k, dxfattribs={"layer": "A-ANNO-TEXT", "insert": pt(x, y, off)})
        # decoys: furniture rectangles and a title that must not become rooms
        ox, oy = pt(0.6, 0.6, off)
        msp.add_lwpolyline([(ox, oy), (ox + 0.9 * k, oy), (ox + 0.9 * k, oy + 0.6 * k), (ox, oy + 0.6 * k)], close=True,
                           dxfattribs={"layer": "A-FURN"})
        for (a, b, label) in spec.dims:
            d = msp.add_linear_dim(base=(((a[0] + off[0]) * k), (a[1] + off[1] - 0.8) * k),
                                   p1=pt(a[0], a[1], off), p2=pt(b[0], b[1], off),
                                   angle=0 if a[1] == b[1] else 90, dxfattribs={"layer": "A-ANNO-DIMS"})
            d.render()
        built.append(Built(spec, {n: g for n, g in nets.items()}))
    msp.add_text("APARTMENT FLOOR PLAN COLLECTION", height=0.3 * k, dxfattribs={"layer": "A-ANNO-TTLB", "insert": (0, -20 * k)})
    import io

    stream = io.StringIO()
    doc.write(stream)
    return stream.getvalue().encode(), built


def sheet(units: str = "mm", set_units: bool = True):
    return build([plan1(), plan2(), plan3()], units, set_units)
