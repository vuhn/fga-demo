"""Edit an imported plan and write the changes back into the ORIGINAL CAD file.

The model of an imported plan carries, for every room label, door, window and fixture, the handles of the
entities that draw it in the file (`src`) and how the plan sits in the file (`origin`). `patch_dxf` compares
the plan as first imported (`base`) with the plan now (`current`) and applies each difference to the
file itself: it moves, rotates, resizes, renames or deletes exactly those entities and leaves everything else
(layers, blocks, dimensions, hatches, text, XDATA) as it was. What the file cannot express is reported, never
dropped silently: a change we cannot place is listed under `skipped`, and an added item that has no template
in the file is drawn as simple linework on the layer `FGA-ADDED`.
"""

from __future__ import annotations

import io
import math

from ezdxf.math import Matrix44, Vec3
from pydantic import BaseModel, Field
from shapely.geometry import Polygon

from .dxf_plan import _layer_role, read_doc
from .domain.schemas import BuildingModel, Opening, PlacedFixture, PlacedRoom

ADDED_LAYER = "FGA-ADDED"
EPS = 0.005  # metres: smaller changes are rounding noise
_ANGLE = {"east": 0.0, "north": 90.0, "west": 180.0, "south": 270.0}


class PatchReport(BaseModel):
    applied: list[str] = Field(default_factory=list)  # changes written into existing entities
    added: list[str] = Field(default_factory=list)  # new entities drawn on FGA-ADDED (or copied from a template)
    skipped: list[str] = Field(default_factory=list)  # changes the file cannot take, with the reason


class _Ctx:
    def __init__(self, doc, model: BuildingModel, report: PatchReport):
        self.doc, self.report = doc, report
        o = model.origin
        self.k = o.scale if o else 1.0
        self.dx, self.dy = (o.dx, o.dy) if o else (0.0, 0.0)
        self.msp = doc.modelspace()
        self._verts: list[tuple[float, float]] | None = None
        self.wall_layers = [la.dxf.name for la in doc.layers if _layer_role(la.dxf.name) == "wall"]

    def to_file(self, x: float, y: float) -> tuple[float, float]:
        return ((x - self.dx) / self.k, (y - self.dy) / self.k)

    def length(self, metres: float) -> float:
        return metres / self.k

    def entity(self, handle: str):
        e = self.doc.entitydb.get(handle)
        return e if e is not None and e.is_alive else None

    def delete(self, e) -> None:
        layout = e.get_layout() or self.msp
        layout.delete_entity(e)

    def snap(self, pt: tuple[float, float]) -> tuple[float, float]:
        """The nearest wall vertex of the file within 3 cm, else the point: lines added across a gap must
        meet the existing wall lines exactly, or the plan's rooms still leak into each other."""
        if self._verts is None:
            self._verts = []
            for e in self.msp:
                if _layer_role(e.dxf.layer) != "wall":
                    continue
                t = e.dxftype()
                if t == "LINE":
                    self._verts += [(e.dxf.start.x, e.dxf.start.y), (e.dxf.end.x, e.dxf.end.y)]
                elif t == "LWPOLYLINE":
                    self._verts += [(x, y) for x, y in e.get_points("xy")]
        tol = self.length(0.03)
        best = min(self._verts, key=lambda v: math.hypot(v[0] - pt[0], v[1] - pt[1]), default=None)
        if best is not None and math.hypot(best[0] - pt[0], best[1] - pt[1]) <= tol:
            return (float(best[0]), float(best[1]))
        return pt

    def added_layer(self) -> str:
        if ADDED_LAYER not in self.doc.layers:
            self.doc.layers.add(ADDED_LAYER, color=3)
        return ADDED_LAYER


def _center(rect) -> tuple[float, float]:
    return ((rect[0] + rect[2]) / 2, (rect[1] + rect[3]) / 2)


def _dims(rect) -> tuple[float, float]:
    return tuple(sorted((round(rect[2] - rect[0], 3), round(rect[3] - rect[1], 3))))  # type: ignore[return-value]


def _transform(e, m: Matrix44) -> bool:
    try:
        e.transform(m)
        return True
    except Exception:  # NonUniformScalingError and friends: the entity cannot take this transform
        return False


# ---------------------------------------------------------------- fixtures


def _patch_fixture(cx: _Ctx, old: PlacedFixture, new: PlacedFixture) -> None:
    label = f"{new.kind} {new.id}"
    oc, nc = _center(old.rect), _center(new.rect)
    turn = (_ANGLE[new.facing] - _ANGLE[old.facing]) % 360
    moved = math.hypot(nc[0] - oc[0], nc[1] - oc[1]) > EPS
    if _dims(old.rect) != _dims(new.rect):
        cx.report.skipped.append(f"{label}: resizing a block is not written to the file")
        return
    if not moved and turn == 0:
        return
    ents = [cx.entity(h) for h in old.src]
    if not old.src or any(e is None for e in ents):
        cx.report.skipped.append(f"{label}: its block is not a top-level entity of the file, so it cannot be moved")
        return
    c0 = Vec3(*cx.to_file(*oc), 0)
    c1 = Vec3(*cx.to_file(*nc), 0)
    m = Matrix44.translate(-c0.x, -c0.y, 0)
    if turn:
        m @= Matrix44.z_rotate(math.radians(turn))
    m @= Matrix44.translate(c1.x, c1.y, 0)
    if all(_transform(e, m) for e in ents):
        cx.report.applied.append(f"{label}: " + ("moved" if moved else "") + (" and " if moved and turn else "") + (f"turned {turn:g}°" if turn else ""))
    else:
        cx.report.skipped.append(f"{label}: the block cannot be moved")


def _remove_fixture(cx: _Ctx, old: PlacedFixture) -> None:
    ents = [cx.entity(h) for h in old.src]
    if not old.src or any(e is None for e in ents):
        cx.report.skipped.append(f"{old.kind} {old.id}: removed in the app, but its block is not a top-level entity, so it stays in the file")
        return
    for e in ents:
        cx.delete(e)
    cx.report.applied.append(f"{old.kind} {old.id}: removed")


def _add_fixture(cx: _Ctx, new: PlacedFixture, templates: dict[str, PlacedFixture]) -> None:
    """Copy a block of the same kind already in the file (same look, layer and attributes); otherwise draw
    the footprint as a rectangle on FGA-ADDED."""
    label = f"{new.kind} {new.id}"
    tpl = templates.get(new.kind)
    src = cx.entity(tpl.src[0]) if tpl and tpl.src else None
    nc = _center(new.rect)
    if src is not None and src.dxftype() == "INSERT":
        copy = src.copy()
        cx.msp.add_entity(copy)
        tc = _center(tpl.rect)  # type: ignore[union-attr]
        turn = (_ANGLE[new.facing] - _ANGLE[tpl.facing]) % 360  # type: ignore[union-attr]
        c0 = Vec3(*cx.to_file(*tc), 0)
        c1 = Vec3(*cx.to_file(*nc), 0)
        m = Matrix44.translate(-c0.x, -c0.y, 0) @ Matrix44.z_rotate(math.radians(turn)) @ Matrix44.translate(c1.x, c1.y, 0)
        if _transform(copy, m):
            if "ID" in {a.dxf.tag.upper() for a in copy.attribs}:
                for a in copy.attribs:
                    if a.dxf.tag.upper() == "ID":
                        a.dxf.text = new.id
            cx.report.added.append(f"{label}: copied from the block '{src.dxf.name}'")
            return
        cx.msp.delete_entity(copy)
    x0, y0, x1, y1 = new.rect
    pts = [cx.to_file(x0, y0), cx.to_file(x1, y0), cx.to_file(x1, y1), cx.to_file(x0, y1)]
    cx.msp.add_lwpolyline(pts, close=True, dxfattribs={"layer": cx.added_layer()})
    cx.msp.add_text(new.kind.replace("_", " "), dxfattribs={"layer": cx.added_layer(), "height": cx.length(0.15),
                                                            "insert": cx.to_file(*nc)})
    cx.report.added.append(f"{label}: drawn as a rectangle on {ADDED_LAYER} (no block of this kind in the file)")


# ---------------------------------------------------------------- openings


def _wall_dir(model: BuildingModel, o: Opening) -> tuple[float, float]:
    w = next(w for w in model.walls if w.id == o.wall_id)
    dx, dy = w.end[0] - w.start[0], w.end[1] - w.start[1]
    n = math.hypot(dx, dy) or 1.0
    return dx / n, dy / n


def _patch_opening(cx: _Ctx, model: BuildingModel, old: Opening, new: Opening) -> None:
    label = f"{new.kind} {new.id}"
    moved = math.hypot(new.center[0] - old.center[0], new.center[1] - old.center[1]) > EPS
    ratio = new.width / old.width if old.width else 1.0
    resized = abs(new.width - old.width) > EPS
    if not moved and not resized:
        if new.swing_into != old.swing_into:
            cx.report.skipped.append(f"{label}: flipping the swing is not written to the file")
        return
    ents = [cx.entity(h) for h in old.src]
    if not old.src or any(e is None for e in ents):
        cx.report.skipped.append(f"{label}: its symbol is not made of top-level entities, so it cannot be changed")
        return
    c0 = Vec3(*cx.to_file(*old.center), 0)
    c1 = Vec3(*cx.to_file(*new.center), 0)
    ux, uy = _wall_dir(model, old)
    theta = math.atan2(uy, ux)
    m = Matrix44.translate(-c0.x, -c0.y, 0)
    if resized:
        if old.kind == "window":  # along the wall only: a window keeps its depth
            m @= Matrix44.z_rotate(-theta) @ Matrix44.scale(ratio, 1, 1) @ Matrix44.z_rotate(theta)
        else:  # a door scales as a whole: its leaf and swing grow with the doorway
            m @= Matrix44.scale(ratio, ratio, 1)
    m @= Matrix44.translate(c1.x, c1.y, 0)
    if all(_transform(e, m) for e in ents):
        what = ", ".join(x for x in (("moved" if moved else ""), (f"width {old.width:g} → {new.width:g} m" if resized else "")) if x)
        cx.report.applied.append(f"{label}: {what}")
    else:
        cx.report.skipped.append(f"{label}: the symbol cannot be rescaled (a block with a rotated non-uniform scale)")


def _close_gap(cx: _Ctx, model: BuildingModel, old: Opening) -> None:
    """The wall has a gap where the removed door or window was: draw its two faces across it."""
    w = next(w for w in model.walls if w.id == old.wall_id)
    ux, uy = _wall_dir(model, old)
    nx, ny = -uy, ux
    h, t = old.width / 2, w.thickness / 2
    layer = _nearest_wall_layer(cx, old.center) or (cx.wall_layers[0] if cx.wall_layers else cx.added_layer())
    for s in (-1, 1):
        a = (old.center[0] - ux * h + nx * t * s, old.center[1] - uy * h + ny * t * s)
        b = (old.center[0] + ux * h + nx * t * s, old.center[1] + uy * h + ny * t * s)
        cx.msp.add_line(cx.snap(cx.to_file(*a)), cx.snap(cx.to_file(*b)), dxfattribs={"layer": layer})


def _nearest_wall_layer(cx: _Ctx, at: tuple[float, float]) -> str | None:
    p = Vec3(*cx.to_file(*at), 0)
    best: tuple[float, str] | None = None
    for e in cx.msp:
        if e.dxftype() != "LINE" or _layer_role(e.dxf.layer) != "wall":
            continue
        d = p.distance(Vec3(e.dxf.start).lerp(Vec3(e.dxf.end), 0.5))
        if best is None or d < best[0]:
            best = (d, e.dxf.layer)
    return best[1] if best else None


def _remove_opening(cx: _Ctx, model: BuildingModel, old: Opening) -> None:
    label = f"{old.kind} {old.id}"
    ents = [cx.entity(h) for h in old.src]
    if not old.src or any(e is None for e in ents):
        cx.report.skipped.append(f"{label}: removed in the app, but its symbol is not made of top-level entities, so it stays in the file")
        return
    for e in ents:
        cx.delete(e)
    if old.kind != "open":
        _close_gap(cx, model, old)
    cx.report.applied.append(f"{label}: removed, wall closed across the gap")


# ---------------------------------------------------------------- room labels


def _label_entity(cx: _Ctx, ref: str):
    """(entity, attribute-or-None) for a handle "H" or "H:TAG"."""
    handle, _, tag = ref.partition(":")
    e = cx.entity(handle)
    if e is None:
        return None, None
    if not tag:
        return e, None
    for a in getattr(e, "attribs", []):
        if a.dxf.tag == tag:
            return e, a
    return None, None


def _retitle(text: str, name: str) -> str:
    """Keeps numeric lines ("11.4 m²") of a label block and replaces the words."""
    import re

    from .dxf_plan import NUMERIC_TEXT

    parts = re.split(r"(\\P|\n|\r)", text)
    out, done = [], False
    for part in parts:
        if part in ("\\P", "\n", "\r"):
            out.append(part)
            continue
        if part.strip() and not NUMERIC_TEXT.match(part.strip()):
            out.append("" if done else name)
            done = True
        else:
            out.append(part)
    if not done:
        out.insert(0, name + "\\P")
    return "".join(out)


def _patch_room(cx: _Ctx, old: PlacedRoom, new: PlacedRoom) -> None:
    if new.name != old.name:
        if not old.src:
            poly = Polygon(new.net_polygon())
            at = poly.representative_point()
            cx.msp.add_text(new.name, dxfattribs={"layer": cx.added_layer(), "height": cx.length(0.25), "insert": cx.to_file(at.x, at.y)})
            cx.report.added.append(f"room {old.name}: label '{new.name}' drawn on {ADDED_LAYER} (the room had none)")
        else:
            first = True
            for ref in old.src:
                e, att = _label_entity(cx, ref)
                if e is None:
                    cx.report.skipped.append(f"room {old.name}: label entity {ref} not found")
                    continue
                target = att if att is not None else e
                kind = target.dxftype()
                if kind == "MTEXT":
                    target.text = _retitle(target.text, new.name) if first else ""
                else:
                    target.dxf.text = new.name if first else ""
                first = False
            cx.report.applied.append(f"room {old.name}: renamed to '{new.name}'")
    if new.type != old.type:
        cx.report.skipped.append(f"room {new.name}: its type is kept in the app only (a CAD file has no room types)")


# ---------------------------------------------------------------- entry point


def patch_dxf(original: bytes, base: BuildingModel, current: BuildingModel) -> tuple[bytes, PatchReport]:
    """The original file with every difference between `base` (as imported) and `current` applied."""
    if base.source != "imported" or current.source != "imported" or base.origin is None:
        raise ValueError("only an imported plan can be written back to its original file")
    doc = read_doc(original)
    report = PatchReport()
    cx = _Ctx(doc, base, report)

    f_old = {f.id: f for f in base.fixtures}
    f_new = {f.id: f for f in current.fixtures}
    templates: dict[str, PlacedFixture] = {}
    for f in base.fixtures:
        if f.src and f.id in f_new:  # a block still in the file
            templates.setdefault(f.kind, f)
    for fid, old in f_old.items():
        if fid in f_new:
            _patch_fixture(cx, old, f_new[fid])
        else:
            _remove_fixture(cx, old)
    for fid, new in f_new.items():
        if fid not in f_old:
            _add_fixture(cx, new, templates)

    o_old = {o.id: o for o in base.openings}
    o_new = {o.id: o for o in current.openings}
    for oid, old in o_old.items():
        if oid in o_new:
            _patch_opening(cx, base, old, o_new[oid])
        else:
            _remove_opening(cx, base, old)
    for oid in o_new.keys() - o_old.keys():
        report.skipped.append(f"{o_new[oid].kind} {oid}: a new opening is not written to the file yet")

    r_old = {r.id: r for r in base.rooms}
    for r in current.rooms:
        if r.id in r_old:
            _patch_room(cx, r_old[r.id], r)
    for rid in r_old.keys() - {r.id for r in current.rooms}:
        report.skipped.append(f"room {r_old[rid].name}: removing a room is not written to the file")

    walls_old = {w.id: w for w in base.walls}
    for w in current.walls:
        o = walls_old.get(w.id)
        if o is not None and (math.hypot(w.start[0] - o.start[0], w.start[1] - o.start[1]) > EPS
                              or math.hypot(w.end[0] - o.end[0], w.end[1] - o.end[1]) > EPS or abs(w.thickness - o.thickness) > EPS):
            report.skipped.append(f"wall {w.id}: moving or resizing a wall is not written to the file yet")

    if not report.applied and not report.added:
        return original, report  # nothing was written: the file is returned byte for byte
    # the file's own encoding (UTF-8 from R2007 on, the code page of older files)
    raw = io.BytesIO()
    stream = io.TextIOWrapper(raw, encoding=doc.output_encoding, errors="dxfreplace", newline="")
    doc.write(stream)
    stream.flush()
    stream.detach()
    return raw.getvalue(), report

