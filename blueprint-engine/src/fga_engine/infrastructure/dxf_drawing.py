"""DXF -> drawing items: every visible entity of a CAD file as flat, read-only geometry. No LLM.

`dxf_plan` reads the semantic part of a floor plan (walls, doors, windows, rooms, fixtures). This module reads
the rest, so that nothing in the file is silently lost and a file with no floor plan at all (a part, a sketch,
a map) still imports as a drawing:

  * LINE, ARC, CIRCLE, ELLIPSE, SPLINE, LWPOLYLINE, POLYLINE (2D, 3D, polyface and mesh) -> polylines;
  * HATCH (any boundary: polyline, arcs, ellipses, splines), SOLID, TRACE, 3DFACE -> filled outlines;
  * POINT, TEXT, MTEXT, block attributes -> point and text items;
  * INSERT (nested blocks, arrays), DIMENSION, LEADER, MLEADER, MLINE, TABLE -> exploded into their parts;
  * entities on frozen or switched-off layers are left out; what cannot be drawn (images, 3D solids, infinite
    rays, ...) is counted in the returned `skipped` map, never dropped silently.

Curves are flattened with a tolerance relative to the drawing's size, so a 10 mm part and a 100 m site both
keep their shape. Items come back in file units; `scale_items` moves them into metres and model coordinates.
"""

from __future__ import annotations

import math

from ..domain.schemas import DrawItem

MAX_ITEMS = 120_000
MAX_VERTICES = 2_000_000
MAX_DEPTH = 6  # nested block references
TEXT_MIN_HEIGHT = 1e-6

# entity types the drawing cannot show, with the words used in the report
UNDRAWABLE = {
    "IMAGE": "image", "WIPEOUT": "wipeout mask", "UNDERLAY": "underlay", "PDFUNDERLAY": "underlay",
    "DGNUNDERLAY": "underlay", "DWFUNDERLAY": "underlay", "OLE2FRAME": "OLE object", "VIEWPORT": "viewport",
    "RAY": "infinite ray", "XLINE": "construction line", "3DSOLID": "3D solid", "BODY": "3D body",
    "REGION": "region", "SURFACE": "3D surface", "EXTRUDEDSURFACE": "3D surface", "LOFTEDSURFACE": "3D surface",
    "REVOLVEDSURFACE": "3D surface", "SWEPTSURFACE": "3D surface", "PLANESURFACE": "3D surface",
    "LIGHT": "light", "SUN": "sun", "CAMERA": "camera", "ATTDEF": "attribute definition",
}
PATHS = {"LINE", "ARC", "CIRCLE", "ELLIPSE", "SPLINE", "LWPOLYLINE", "POLYLINE", "HATCH", "SOLID", "TRACE",
         "3DFACE", "HELIX"}
FILLED = {"HATCH", "SOLID", "TRACE", "3DFACE"}
EXPLODE = {"DIMENSION", "ARC_DIMENSION", "LARGE_RADIAL_DIMENSION", "LEADER", "MLEADER", "MULTILEADER", "MLINE",
           "ACAD_TABLE", "TABLE", "MESH", "TOLERANCE"}


def _r(v: float) -> float:
    return round(v, 4)


def _hex(rgb: tuple[int, int, int]) -> str:
    return "#%02x%02x%02x" % rgb


class _Reader:
    def __init__(self, doc, tol: float):
        self.doc = doc
        self.tol = tol
        self.items: list[DrawItem] = []
        self.skipped: dict[str, int] = {}
        self.vertices = 0
        self.truncated = False

    # ---- helpers

    def skip(self, what: str) -> None:
        self.skipped[what] = self.skipped.get(what, 0) + 1

    def _layer_hidden(self, name: str) -> bool:
        try:
            lay = self.doc.layers.get(name)
        except Exception:
            return False
        try:
            return bool(lay.is_off() or lay.is_frozen())
        except Exception:
            return False

    def _color(self, e, layer: str, parent: str) -> str:
        """The entity's ink as #rrggbb. ACI 7 is white-or-black (it follows the background): drawn dark."""
        from ezdxf import colors

        try:
            tc = e.dxf.get("true_color", None)
            if tc is not None:
                return _hex(colors.int2rgb(tc))
            aci = e.dxf.get("color", 256)
            if aci == 0:  # BYBLOCK: the insert's colour
                return parent
            if aci == 256:  # BYLAYER
                lay = self.doc.layers.get(layer)
                ltc = lay.dxf.get("true_color", None)
                if ltc is not None:
                    return _hex(colors.int2rgb(ltc))
                aci = abs(lay.dxf.get("color", 7))
            if aci in (7, 255, 250):
                return ""
            if 1 <= aci <= 255:
                return _hex(colors.aci2rgb(aci))
        except Exception:
            pass
        return ""

    def add(self, item: DrawItem) -> None:
        if len(self.items) >= MAX_ITEMS or self.vertices >= MAX_VERTICES:
            self.truncated = True
            return
        self.vertices += sum(len(p) for p in item.paths)
        self.items.append(item)

    def _new_id(self) -> str:
        return f"g{len(self.items) + 1}"

    # ---- entity handlers

    def paths(self, e, layer: str, color: str, src: str) -> None:
        from ezdxf import path as ezpath

        t = e.dxftype()
        if t == "POLYLINE" and (e.is_polygon_mesh or e.is_poly_face_mesh):
            self.mesh_edges(e, layer, color, src)
            return
        p = ezpath.make_path(e)
        if t == "HATCH" and not len(p):
            return
        out: list[list[tuple[float, float]]] = []
        for sub in p.sub_paths():
            pts = [(_r(v.x), _r(v.y)) for v in sub.flattening(self.tol)]
            closed = sub.is_closed or t in FILLED
            if closed and pts and pts[0] != pts[-1]:
                pts.append(pts[0])
            if len(pts) >= 2:
                out.append(pts)
        if out:
            self.add(DrawItem(id=self._new_id(), type=t, layer=layer, color=color, paths=out, filled=t in FILLED, src=src))

    def mesh_edges(self, e, layer: str, color: str, src: str) -> None:
        """Polyface and polygon meshes: their edges, seen from above."""
        verts = [(_r(v.dxf.location.x), _r(v.dxf.location.y)) for v in e.vertices if not v.is_face_record] \
            if e.is_poly_face_mesh else []
        out: list[list[tuple[float, float]]] = []
        if e.is_poly_face_mesh:
            for v in e.vertices:
                if not v.is_face_record:
                    continue
                idx = [abs(getattr(v.dxf, f"vtx{i}", 0)) for i in range(4)]
                face = [verts[i - 1] for i in idx if 0 < i <= len(verts)]
                if len(face) >= 2:
                    out.append([*face, face[0]])
        else:
            pts = [(_r(v.dxf.location.x), _r(v.dxf.location.y)) for v in e.vertices]
            m, n = e.dxf.m_count, e.dxf.n_count
            for i in range(m):
                out.append(pts[i * n:(i + 1) * n])
            for j in range(n):
                out.append(pts[j::n][:m])
        out = [p for p in out if len(p) >= 2]
        if out:
            self.add(DrawItem(id=self._new_id(), type="POLYLINE", layer=layer, color=color, paths=out, src=src))

    def point(self, e, layer: str, color: str, src: str) -> None:
        p = e.dxf.location
        self.add(DrawItem(id=self._new_id(), type="POINT", layer=layer, color=color, at=(_r(p.x), _r(p.y)), src=src))

    def text(self, e, layer: str, color: str, src: str) -> None:
        t = e.dxftype()
        if t == "MTEXT":
            txt = e.plain_text()
            p = e.dxf.insert
            h = float(e.dxf.get("char_height", 0) or 0)
            d = e.dxf.get("text_direction", None)
            rot = math.degrees(math.atan2(d.y, d.x)) if d is not None else float(e.dxf.get("rotation", 0) or 0)
        else:  # TEXT, ATTRIB
            txt = e.dxf.text
            p = e.dxf.get("align_point", None) if (e.dxf.get("halign", 0) or e.dxf.get("valign", 0)) else None
            p = p if p is not None else e.dxf.insert
            h = float(e.dxf.get("height", 0) or 0)
            rot = float(e.dxf.get("rotation", 0) or 0)
        if not txt or not txt.strip():
            return
        self.add(DrawItem(id=self._new_id(), type=t, layer=layer, color=color, at=(_r(p.x), _r(p.y)),
                          text=txt.replace("\\P", "\n").strip(), height=max(h, TEXT_MIN_HEIGHT), rotation=round(rot, 2), src=src))

    def visit(self, e, depth: int = 0, parent_layer: str | None = None, parent_color: str = "", src: str = "") -> None:
        t = e.dxftype()
        layer = e.dxf.get("layer", "0")
        if layer == "0" and parent_layer:
            layer = parent_layer  # entities on layer 0 inside a block take the insert's layer
        if self._layer_hidden(layer) or e.dxf.get("invisible", 0):
            self.skip("entity on a hidden layer")
            return
        color = self._color(e, layer, parent_color)
        src = src or (e.dxf.get("handle", "") or "")
        try:
            if t in PATHS:
                self.paths(e, layer, color, src)
            elif t == "POINT":
                self.point(e, layer, color, src)
            elif t in ("TEXT", "MTEXT", "ATTRIB"):
                self.text(e, layer, color, src)
            elif t == "INSERT":
                self.insert(e, depth, layer, color, src)
            elif t in EXPLODE or hasattr(e, "virtual_entities"):
                if depth >= MAX_DEPTH:
                    self.skip("nested too deep")
                    return
                parts = list(e.virtual_entities())
                if not parts:
                    self.skip(f"{t.lower()} with no drawable parts")
                for part in parts:
                    self.visit(part, depth + 1, layer, color, src)
            elif t in UNDRAWABLE:
                self.skip(UNDRAWABLE[t])
            else:
                self.skip(f"{t.lower()} entity")
        except Exception:
            self.skip(f"unreadable {t.lower()}")

    def parts(self, ins) -> list:
        """The entities of a block reference in world space. One nested reference with a degenerate scale or
        extrusion makes ezdxf refuse the whole block, so on failure the block is exploded child by child and
        only the bad reference is lost."""
        try:
            return list(ins.virtual_entities())
        except Exception:
            pass
        out: list = []
        try:
            m = ins.matrix44()
            children = list(ins.block())
        except Exception:
            self.skip("unreadable block reference")
            return out
        for child in children:
            try:
                kids = self.parts(child) if child.dxftype() == "INSERT" else [child.copy()]
                for kid in kids:
                    kid.transform(m)
                    out.append(kid)
            except Exception:
                self.skip("unreadable nested block reference")
        return out

    def insert(self, e, depth: int, layer: str, color: str, src: str) -> None:
        if depth >= MAX_DEPTH:
            self.skip("block nested too deep")
            return
        for part in self.parts(e):
            self.visit(part, depth + 1, layer, color, src)
        for att in getattr(e, "attribs", []):  # attribute values are visible text
            if not att.dxf.get("invisible", 0):
                self.visit(att, depth + 1, layer, color, src)


def _extent(doc) -> float:
    """Longest side of the drawing in file units (0 when it cannot be measured)."""
    try:
        from ezdxf import bbox

        box = bbox.extents(doc.modelspace(), fast=True)
        if box.has_data:
            return float(max(box.size.x, box.size.y))
    except Exception:
        pass
    return 0.0


def collect_drawing(doc) -> tuple[list[DrawItem], dict[str, int], bool]:
    """Every drawable entity of the model space, in file units: (items, skipped counts, truncated)."""
    ext = _extent(doc)
    rd = _Reader(doc, tol=ext / 4000 if ext > 0 else 0.01)
    for e in doc.modelspace():
        rd.visit(e)
    # re-id so ids are g1..gN in file order
    for i, it in enumerate(rd.items, 1):
        it.id = f"g{i}"
    return rd.items, rd.skipped, rd.truncated


def bounds(items: list[DrawItem]) -> tuple[float, float, float, float] | None:
    xs: list[float] = []
    ys: list[float] = []
    for it in items:
        for path in it.paths:
            for x, y in path:
                xs.append(x)
                ys.append(y)
        if it.at is not None:
            xs.append(it.at[0])
            ys.append(it.at[1])
    return (min(xs), min(ys), max(xs), max(ys)) if xs else None


def scale_items(items: list[DrawItem], k: float, dx: float, dy: float) -> list[DrawItem]:
    """Items in file units -> metres in model coordinates: model = file * k + (dx, dy)."""
    out: list[DrawItem] = []
    for it in items:
        out.append(it.model_copy(update={
            "paths": [[(_r(x * k + dx), _r(y * k + dy)) for x, y in p] for p in it.paths],
            "at": (_r(it.at[0] * k + dx), _r(it.at[1] * k + dy)) if it.at else None,
            "height": _r(it.height * k) if it.height else 0.0,
        }))
    return out
