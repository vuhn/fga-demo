"""IFC4 export of a BuildingModel (Text2BIM's BIM output, from our deterministic model).

Project -> Site -> Building -> one Storey. Walls are extruded from their centerlines; each
door, window and entrance cuts an IfcOpeningElement that its IfcDoor or IfcWindow fills;
an open-plan passage cuts the full wall height and stays unfilled.
Rooms become IfcSpace with their net floor area as a quantity; the footprint becomes a
floor slab; fixtures become furniture, sanitary terminals or appliances inside their space.
Code only: nothing here comes from the LLM.
"""

from __future__ import annotations

import math

import ifcopenshell
import ifcopenshell.api.aggregate
import ifcopenshell.api.context
import ifcopenshell.api.feature
import ifcopenshell.api.geometry
import ifcopenshell.api.project
import ifcopenshell.api.pset
import ifcopenshell.api.root
import ifcopenshell.api.spatial
import ifcopenshell.api.unit
import numpy as np
from ifcopenshell.util.shape_builder import ShapeBuilder

from .schemas import BuildingModel, Opening, Wall

WALL_HEIGHT = 2.7
SLAB_DEPTH = 0.2
DOOR_HEIGHT = 2.1
WINDOW_SILL = 0.9
WINDOW_HEIGHT = 1.2

# fixture kind -> (IFC class, predefined type, height in m)
FIXTURE_CLASS: dict[str, tuple[str, str, float]] = {
    "bed": ("IfcFurniture", "BED", 0.5),
    "wardrobe": ("IfcFurniture", "USERDEFINED", 2.0),
    "desk": ("IfcFurniture", "DESK", 0.75),
    "shelves": ("IfcFurniture", "SHELF", 1.8),
    "sofa": ("IfcFurniture", "SOFA", 0.8),
    "tv_unit": ("IfcFurniture", "USERDEFINED", 0.5),
    "dining_table": ("IfcFurniture", "TABLE", 0.75),
    "kitchen_counter": ("IfcFurniture", "USERDEFINED", 0.9),
    "sink": ("IfcSanitaryTerminal", "SINK", 0.9),
    "stove": ("IfcElectricAppliance", "ELECTRICCOOKER", 0.9),
    "fridge": ("IfcElectricAppliance", "FRIDGE_FREEZER", 1.8),
    "toilet": ("IfcSanitaryTerminal", "TOILETPAN", 0.4),
    "basin": ("IfcSanitaryTerminal", "WASHHANDBASIN", 0.85),
    "shower": ("IfcSanitaryTerminal", "SHOWER", 0.1),
    "bathtub": ("IfcSanitaryTerminal", "BATH", 0.55),
    "washer": ("IfcElectricAppliance", "WASHINGMACHINE", 0.85),
}


def _matrix(x: float, y: float, z: float = 0.0, angle: float = 0.0) -> np.ndarray:
    m = np.eye(4)
    c, s = math.cos(angle), math.sin(angle)
    m[0][0], m[0][1], m[1][0], m[1][1] = c, -s, s, c
    m[0][3], m[1][3], m[2][3] = x, y, z
    return m


def _place(f: ifcopenshell.file, product, m: np.ndarray) -> None:
    ifcopenshell.api.geometry.edit_object_placement(f, product=product, matrix=m, is_si=True)


def _box(f: ifcopenshell.file, body, rect: tuple[float, float, float, float], height: float):
    """Representation of an axis-aligned box whose local origin is the rect's lower-left corner."""
    b = ShapeBuilder(f)
    w, d = rect[2] - rect[0], rect[3] - rect[1]
    profile = b.polyline([(0.0, 0.0), (w, 0.0), (w, d), (0.0, d)], closed=True)
    return b.get_representation(body, [b.extrude(profile, height)])


def _wall_frame(w: Wall) -> tuple[float, float, float]:
    (ax, ay), (bx, by) = w.start, w.end
    return math.atan2(by - ay, bx - ax), math.hypot(bx - ax, by - ay), 0.0


def _along(w: Wall, o: Opening) -> float:
    """Distance from the wall's start to the opening's centre, along the wall."""
    return math.hypot(o.center[0] - w.start[0], o.center[1] - w.start[1])


def to_ifc(model: BuildingModel, project_name: str = "Single-storey house") -> bytes:
    f = ifcopenshell.api.project.create_file(version="IFC4")
    project = ifcopenshell.api.root.create_entity(f, ifc_class="IfcProject", name=project_name)
    units = [ifcopenshell.api.unit.add_si_unit(f, unit_type=t) for t in ("LENGTHUNIT", "AREAUNIT", "VOLUMEUNIT")]
    ifcopenshell.api.unit.assign_unit(f, units=units)
    model3d = ifcopenshell.api.context.add_context(f, context_type="Model")
    body = ifcopenshell.api.context.add_context(f, context_type="Model", context_identifier="Body",
                                                target_view="MODEL_VIEW", parent=model3d)

    site = ifcopenshell.api.root.create_entity(f, ifc_class="IfcSite", name="Lot")
    building = ifcopenshell.api.root.create_entity(f, ifc_class="IfcBuilding", name=project_name)
    storey = ifcopenshell.api.root.create_entity(f, ifc_class="IfcBuildingStorey", name="Ground floor")
    ifcopenshell.api.aggregate.assign_object(f, products=[site], relating_object=project)
    ifcopenshell.api.aggregate.assign_object(f, products=[building], relating_object=site)
    ifcopenshell.api.aggregate.assign_object(f, products=[storey], relating_object=building)
    for p in (site, building, storey):
        _place(f, p, _matrix(0, 0))

    contained = []

    # floor slab under the footprint
    x0, y0, x1, y1 = model.footprint
    slab = ifcopenshell.api.root.create_entity(f, ifc_class="IfcSlab", predefined_type="FLOOR", name="Ground slab")
    _place(f, slab, _matrix(x0, y0, -SLAB_DEPTH))
    ifcopenshell.api.geometry.assign_representation(f, product=slab, representation=ifcopenshell.api.geometry
        .add_slab_representation(f, context=body, depth=SLAB_DEPTH,
                                 polyline=[(0.0, 0.0), (x1 - x0, 0.0), (x1 - x0, y1 - y0), (0.0, y1 - y0)]))
    contained.append(slab)

    # walls, extruded symmetrically about their centerline
    walls: dict[str, object] = {}
    for w in model.walls:
        angle, length, _ = _wall_frame(w)
        wall = ifcopenshell.api.root.create_entity(f, ifc_class="IfcWall", predefined_type="STANDARD", name=w.id)
        wall.Description = w.kind
        _place(f, wall, _matrix(w.start[0], w.start[1], 0.0, angle))
        rep = ifcopenshell.api.geometry.add_wall_representation(
            f, context=body, length=length, height=WALL_HEIGHT, thickness=w.thickness, offset=-w.thickness / 2)
        ifcopenshell.api.geometry.assign_representation(f, product=wall, representation=rep)
        walls[w.id] = wall
        contained.append(wall)

    # openings cut the wall and are filled by a door or window
    by_wall = {w.id: w for w in model.walls}
    for o in model.openings:
        w = by_wall.get(o.wall_id)
        if w is None:
            continue
        angle, _, _ = _wall_frame(w)
        t = _along(w, o) - o.width / 2
        sx = w.start[0] + math.cos(angle) * t
        sy = w.start[1] + math.sin(angle) * t
        is_window = o.kind == "window"
        z = WINDOW_SILL if is_window else 0.0
        h = WINDOW_HEIGHT if is_window else WALL_HEIGHT + 0.1 if o.kind == "open" else DOOR_HEIGHT
        void = ifcopenshell.api.root.create_entity(f, ifc_class="IfcOpeningElement", name=f"{o.id} opening")
        _place(f, void, _matrix(sx, sy, z, angle))
        cut = w.thickness + 0.1
        ifcopenshell.api.geometry.assign_representation(f, product=void, representation=ifcopenshell.api.geometry
            .add_wall_representation(f, context=body, length=o.width, height=h, thickness=cut, offset=-cut / 2))
        ifcopenshell.api.feature.add_feature(f, feature=void, element=walls[o.wall_id])
        if o.kind == "open":  # open plan: the void cuts the whole wall height and stays unfilled
            continue

        if is_window:
            fill = ifcopenshell.api.root.create_entity(f, ifc_class="IfcWindow", predefined_type="WINDOW", name=o.id)
            fill.OverallWidth, fill.OverallHeight = o.width, h
            rep = ifcopenshell.api.geometry.add_window_representation(f, context=body, overall_width=o.width,
                                                                      overall_height=h)
        else:
            fill = ifcopenshell.api.root.create_entity(f, ifc_class="IfcDoor", predefined_type="DOOR", name=o.id)
            fill.OverallWidth, fill.OverallHeight = o.width, h
            fill.Description = "entrance" if o.kind == "entrance" else "door"
            rep = ifcopenshell.api.geometry.add_door_representation(f, context=body, overall_width=o.width,
                                                                    overall_height=h)
        # the door or window stands on the wall's right-hand face (local -y), spanning its thickness
        half = w.thickness / 2
        _place(f, fill, _matrix(sx + math.sin(angle) * half, sy - math.cos(angle) * half, z, angle))
        if rep is not None:
            ifcopenshell.api.geometry.assign_representation(f, product=fill, representation=rep)
        ifcopenshell.api.feature.add_filling(f, opening=void, element=fill)
        contained.append(fill)

    # spaces (rooms) with their net area; fixtures live in their space
    spaces: dict[str, object] = {}
    for r in model.rooms:
        sp = ifcopenshell.api.root.create_entity(f, ifc_class="IfcSpace", predefined_type="INTERNAL", name=r.name)
        sp.LongName = r.type
        _place(f, sp, _matrix(r.net_rect[0], r.net_rect[1]))
        ifcopenshell.api.geometry.assign_representation(f, product=sp, representation=_box(f, body, r.net_rect,
                                                                                             WALL_HEIGHT))
        qto = ifcopenshell.api.pset.add_qto(f, product=sp, name="Qto_SpaceBaseQuantities")
        ifcopenshell.api.pset.edit_qto(f, qto=qto, properties={"NetFloorArea": round(r.net_area, 2),
                                                                "Height": WALL_HEIGHT})
        spaces[r.id] = sp
    if spaces:
        ifcopenshell.api.aggregate.assign_object(f, products=list(spaces.values()), relating_object=storey)

    per_space: dict[str, list] = {}
    for fx in model.fixtures:
        cls, ptype, height = FIXTURE_CLASS[fx.kind]
        el = ifcopenshell.api.root.create_entity(f, ifc_class=cls, predefined_type=ptype, name=fx.id)
        if ptype == "USERDEFINED":
            el.ObjectType = fx.kind
        _place(f, el, _matrix(fx.rect[0], fx.rect[1]))
        ifcopenshell.api.geometry.assign_representation(f, product=el, representation=_box(f, body, fx.rect, height))
        per_space.setdefault(fx.room_id, []).append(el)
    for room_id, els in per_space.items():
        if room_id in spaces:
            ifcopenshell.api.spatial.assign_container(f, products=els, relating_structure=spaces[room_id])
        else:
            contained += els

    ifcopenshell.api.spatial.assign_container(f, products=contained, relating_structure=storey)
    return f.to_string().encode()
