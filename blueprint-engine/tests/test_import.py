"""DXF import: the three reconstructed sample plans are known-answer cases."""

from __future__ import annotations

import json
import math

import pytest
from fastapi.testclient import TestClient
from shapely.geometry import Polygon

import sample_dxf
from fga_engine.app import create_app
from fga_engine.domain.checks import failed, run_imported_checks
from fga_engine.draw import build_dxf, to_dxf_bytes, to_svg
from fga_engine.dxf_plan import import_dxf
from fga_engine.domain.edits import EditError
from fga_engine.ifc import to_ifc

EXPECT = {  # plan -> (rooms, doors, windows, entrance side)
    1: (6, 6, 4, "west"),
    2: (4, 4, 3, "east"),
    3: (5, 5, 4, "east"),
}


@pytest.fixture(scope="module")
def sheet():
    data, built = sample_dxf.sheet()
    return data, built, import_dxf(data)


def _match(plan, built):
    """Each expected net room against the imported room with the largest overlap."""
    nets = built.nets
    got = {r.id: Polygon(r.net_polygon()) for r in plan.model.rooms}
    out = {}
    for name, exp in nets.items():
        # drawn at an offset: compare areas and shapes by best IoU after aligning centroids
        best = None
        for rid, g in got.items():
            dx, dy = exp.centroid.x - g.centroid.x, exp.centroid.y - g.centroid.y
            from shapely import affinity

            moved = affinity.translate(g, dx, dy)
            iou = moved.intersection(exp).area / max(moved.union(exp).area, 1e-9)
            if best is None or iou > best[0]:
                best = (iou, rid)
        out[name] = best
    return out


def test_sheet_splits_into_three_plans(sheet):
    _, _, res = sheet
    assert res.report.plans_found == 3 and len(res.plans) == 3
    assert res.report.units == "mm" and "INSUNITS" in res.report.unit_source
    assert "A-FURN" in res.report.layers_ignored  # furniture is not geometry


@pytest.mark.parametrize("n", [1, 2, 3])
def test_known_answer(sheet, n):
    _, built, res = sheet
    plan, b = res.plans[n - 1], built[n - 1]
    rooms, doors, windows, side = EXPECT[n]
    m = plan.model
    assert plan.room_count == rooms == len(b.nets)
    assert sum(o.kind in ("door", "entrance") for o in m.openings) == doors
    assert sum(o.kind == "window" for o in m.openings) == windows
    assert sum(o.kind == "entrance" for o in m.openings) == 1
    assert m.entrance_side == side and m.source == "imported"
    for name, (iou, rid) in _match(plan, b).items():
        area = Polygon(m.room(rid).net_polygon()).area
        assert iou > 0.97, (name, iou)
        assert abs(area - b.nets[name].area) / b.nets[name].area < 0.03, name


def test_labels_and_types(sheet):
    _, _, res = sheet
    p1 = res.plans[0].model
    by = {r.name: r.type for r in p1.rooms}
    assert by["Balcony"] == "balcony" and by["Wc"] == "bathroom" and by["Bath"] == "bathroom"
    assert by["Living / Dining / Kitchen"] == "living"
    p3 = res.plans[2].model  # Vietnamese labels
    assert {r.type for r in p3.rooms} == {"bedroom", "bathroom", "living"}
    assert not res.report.unmapped_texts


def test_angled_wall_is_kept(sheet):
    _, _, res = sheet
    angles = {round(math.degrees(math.atan2(w.end[1] - w.start[1], w.end[0] - w.start[0])) % 90) for w in res.plans[2].model.walls}
    assert any(a not in (0, 90) for a in angles)  # the chamfered bay
    assert all(a in (0, 90) for a in {round(math.degrees(math.atan2(w.end[1] - w.start[1], w.end[0] - w.start[0])) % 90) for w in res.plans[0].model.walls})


@pytest.mark.parametrize("n", [0, 1, 2])
def test_imported_plans_render_and_check(sheet, n):
    _, _, res = sheet
    m = res.plans[n].model
    checks = run_imported_checks(m)
    assert not failed(checks), [c.message for c in failed(checks)]  # every room is reachable, none overlap
    doc, sheet_ = build_dxf(m)
    assert to_dxf_bytes(doc) and "<svg" in to_svg(doc, sheet_)
    assert to_ifc(m).startswith(b"ISO-10303-21")


def test_unitless_file_is_guessed_and_confirmed_by_dimensions():
    data, _ = sample_dxf.sheet(set_units=False)
    res = import_dxf(data)
    assert res.report.units == "mm" and "guessed" in res.report.unit_source
    assert len(res.plans) == 3


def test_unit_override():
    data, _ = sample_dxf.sheet(units="m", set_units=False)
    assert import_dxf(data, "m").report.units == "m"
    with pytest.raises(EditError):
        import_dxf(data, "parsec")


def test_garbage_and_empty_files_are_refused():
    with pytest.raises(EditError):
        import_dxf(b"not a dxf at all")
    import ezdxf, io

    doc = ezdxf.new()
    buf = io.StringIO()
    doc.write(buf)
    with pytest.raises(EditError):
        import_dxf(buf.getvalue().encode())


def test_endpoint_returns_plans_with_checks_and_svg(sheet):
    data, _, _ = sheet
    c = TestClient(create_app(llm=object()))
    r = c.post("/v1/import/dxf", files={"file": ("sheet.dxf", data, "image/vnd.dxf")})
    assert r.status_code == 200
    body = r.json()
    assert len(body["plans"]) == 3 and all(p["ok"] and p["svg"].startswith("<?xml") or "<svg" in p["svg"] for p in body["plans"])
    bad = c.post("/v1/import/dxf", files={"file": ("x.dxf", b"junk", "image/vnd.dxf")})
    assert bad.status_code == 422


def test_single_line_walls_are_read_with_a_default_thickness():
    import io

    import ezdxf

    doc = ezdxf.new("R2010")
    doc.header["$INSUNITS"] = 6
    doc.layers.add("WALLS")
    msp = doc.modelspace()
    for pts in ([(0, 0), (8, 0), (8, 5), (0, 5)], [(4, 0), (4, 5)]):
        msp.add_lwpolyline(pts, close=len(pts) > 2, dxfattribs={"layer": "WALLS"})
    msp.add_text("BEDROOM", height=0.2, dxfattribs={"insert": (1.5, 2.5)})
    msp.add_text("LIVING", height=0.2, dxfattribs={"insert": (5.5, 2.5)})
    buf = io.StringIO()
    doc.write(buf)
    res = import_dxf(buf.getvalue().encode())
    assert any("single lines" in w for w in res.report.warnings)
    assert sorted(r.type for r in res.plans[0].model.rooms) == ["bedroom", "living"]
    assert 15 < sum(r.net_area for r in res.plans[0].model.rooms) < 40


def _own_export_cases():
    from briefs import FEASIBLE, STUDIO, WEST_ENTRY_STUDY

    return [("studio", STUDIO), ("west_entry_study", WEST_ENTRY_STUDY)] + [(k, v) for k, v in list(FEASIBLE.items())[:3]]


@pytest.mark.parametrize("name,brief", _own_export_cases())
def test_our_own_dxf_export_imports_back_to_the_same_rooms(name, brief):
    """Exports cut doors and windows out of the wall solids; the importer must still find every room."""
    from fga_engine.pipeline import generate

    gen = generate(brief).model
    assert gen is not None
    doc, _ = build_dxf(gen)
    res = import_dxf(to_dxf_bytes(doc))
    assert len(res.plans) == 1
    got = res.plans[0].model
    # rooms the exporter drew (nested rooms are inside a parent and stay separate rooms)
    assert len(got.rooms) == len(gen.rooms), (name, [r.name for r in got.rooms], [r.name for r in gen.rooms])
    want = sorted(r.net_area for r in gen.rooms)
    have = sorted(r.net_area for r in got.rooms)
    for w, h in zip(want, have):
        assert abs(w - h) / w < 0.06, (name, want, have)
    assert not failed(run_imported_checks(got)), [c.message for c in failed(run_imported_checks(got))]


def test_round_trip_keeps_fixtures_and_reports_unsupported_items():
    """Our export carries furniture blocks with a KIND attribute: every one comes back as a fixture."""
    from briefs import FEASIBLE
    from fga_engine.pipeline import generate

    gen = None
    for brief in FEASIBLE.values():
        g = generate(brief).model
        if g is not None and len(g.fixtures) >= 4:
            gen = g
            break
    assert gen is not None
    doc, _ = build_dxf(gen)
    res = import_dxf(to_dxf_bytes(doc))
    got = res.plans[0].model
    assert res.report.fixtures == len(got.fixtures) == len(gen.fixtures)
    assert sorted(f.kind for f in got.fixtures) == sorted(f.kind for f in gen.fixtures)
    assert all(f.room_id in {r.id for r in got.rooms} for f in got.fixtures)


def test_dimensions_and_other_entities_are_kept_as_drawing_items(sheet):
    res = sheet[2]
    assert not any("dimension" in s for s in res.report.skipped), res.report.skipped
    assert sum(len(p.model.drawing) for p in res.plans) > 0  # the sheet's dimension rows and notes are drawn


# ---------------------------------------------------------------- chat edits on an imported plan


class _Scripted:
    def __init__(self, *answers):
        self.answers, self.prompts = list(answers), []

    def structured(self, system, user, schema):
        self.prompts.append(user)
        a = self.answers.pop(0) if len(self.answers) > 1 else self.answers[0]
        return schema.model_validate(a)


def _plan_client(*answers):
    return TestClient(create_app(_Scripted(*answers)))


def test_plan_revise_renames_adds_and_resizes(sheet):
    model = sheet[2].plans[0].model
    room = model.rooms[0]
    door = next(o for o in model.openings if o.kind == "door")
    c = _plan_client({"ops": [
        {"op": "rename_room", "room": room.name, "name": "Library"},
        {"op": "set_room_type", "room": "Library", "type": "study"},
        {"op": "resize_opening", "opening": door.id, "width": 0.8},
    ]})
    r = c.post("/v1/plan/revise", json={"model": model.model_dump(mode="json"), "message": "make it a library"})
    assert r.status_code == 200, r.text
    out = r.json()["result"]["model"]
    assert out["rooms"][0]["name"] == "Library" and out["rooms"][0]["type"] == "study"
    assert next(o for o in out["openings"] if o["id"] == door.id)["width"] == 0.8
    assert out["source"] == "imported" and out["walls"] == model.model_dump(mode="json")["walls"]
    assert r.json()["result"]["checks"]


def test_plan_revise_adds_and_removes_fixtures(sheet):
    model = sheet[2].plans[0].model
    big = max(model.rooms, key=lambda r: r.net_area)
    c = _plan_client({"ops": [{"op": "add_fixture", "room": big.name, "fixture": "sofa"}]})
    r = c.post("/v1/plan/revise", json={"model": model.model_dump(mode="json"), "message": "add a sofa"})
    assert r.status_code == 200, r.text
    fx = [f for f in r.json()["result"]["model"]["fixtures"] if f["room_id"] == big.id]
    assert [f["kind"] for f in fx] == ["sofa"]
    c2 = _plan_client({"ops": [{"op": "remove_fixture", "room": big.name, "fixture": "sofa"}]})
    back = c2.post("/v1/plan/revise", json={"model": r.json()["result"]["model"], "message": "remove the sofa"})
    assert back.status_code == 200 and not back.json()["result"]["model"]["fixtures"]


def test_plan_revise_refuses_geometry_changes_and_unknown_names(sheet):
    model = sheet[2].plans[0].model.model_dump(mode="json")
    c = _plan_client({"ops": [{"op": "unsupported", "reason": "moving walls"}]})
    r = c.post("/v1/plan/revise", json={"model": model, "message": "move the wall"})
    assert r.status_code == 422 and "not supported" in r.json()["message"]
    c = _plan_client({"ops": [{"op": "rename_room", "room": "Nonexistent", "name": "x"}]})
    r = c.post("/v1/plan/revise", json={"model": model, "message": "rename"})
    assert r.status_code == 422 and "no room called" in r.json()["message"]
    c = _plan_client({"ops": [{"op": "remove_opening", "opening": "zzz"}]})
    assert c.post("/v1/plan/revise", json={"model": model, "message": "x"}).status_code == 422


def test_plan_revise_stream_reports_result(sheet):
    model = sheet[2].plans[0].model
    c = _plan_client({"ops": [{"op": "rename_room", "room": model.rooms[0].name, "name": "Den"}]})
    with c.stream("POST", "/v1/plan/revise/stream", json={"model": model.model_dump(mode="json"), "message": "rename"}) as r:
        lines = [json.loads(x) for x in r.iter_lines() if x]
    assert lines[-1]["event"] == "result" and lines[-1]["data"]["result"]["model"]["rooms"][0]["name"] == "Den"


def _rect(msp, x0, y0, x1, y1, layer="WALLS"):
    msp.add_lwpolyline([(x0, y0), (x1, y0), (x1, y1), (x0, y1)], close=True, dxfattribs={"layer": layer})


def _hand_drawn_plan() -> bytes:
    """A 9 x 8 m flat drawn the way hand-made CAD files are (mm): every wall a closed rectangle of its own, a
    partition cut for a door that is only a leaf line and a swing arc on DOORS (hinge on the wall's lower face,
    leaf into the room below, T-junction walls beside it), and a wall stub with a free end."""
    import io

    import ezdxf

    doc = ezdxf.new("R2010")
    doc.header["$INSUNITS"] = 4
    for name in ("WALLS", "DOORS"):
        doc.layers.add(name)
    msp = doc.modelspace()
    _rect(msp, -100, -100, 9100, 100)  # south
    _rect(msp, -100, 7900, 9100, 8100)  # north
    _rect(msp, -100, 100, 100, 7900)  # west
    _rect(msp, 8900, 100, 9100, 7900)  # east
    _rect(msp, 100, 3450, 6850, 3550, "WALLS")  # partition, left of the doorway
    _rect(msp, 7700, 3450, 8900, 3550, "WALLS")  # partition, right of it
    _rect(msp, 6600, 3550, 6700, 7900, "WALLS")  # a wall rising from the partition (T) just left of the door
    _rect(msp, 5650, 100, 5750, 3450, "WALLS")  # a wall below it, ending on the partition
    _rect(msp, 2000, 5500, 2100, 7900, "WALLS")  # a stub that hangs from the north wall and ends free
    msp.add_line((6850, 3450), (6850, 2600), dxfattribs={"layer": "DOORS"})  # leaf
    msp.add_arc((6850, 3450), 850, 270, 360, dxfattribs={"layer": "DOORS"})  # swing, to the jamb at (7700, 3450)
    buf = io.StringIO()
    doc.write(buf)
    return buf.getvalue().encode()


def test_a_loose_leaf_and_arc_door_sits_in_its_wall_and_a_free_wall_end_adds_no_wall():
    m = import_dxf(_hand_drawn_plan()).plans[0].model
    doors = [o for o in m.openings if o.kind == "door"]
    assert len(doors) == 1 and abs(doors[0].width - 0.85) < 0.05, [(o.id, o.kind, o.width) for o in m.openings]
    wall = next(w for w in m.walls if w.id == doors[0].wall_id)
    assert abs(wall.start[1] - wall.end[1]) < 0.05  # the horizontal partition, not a crossing wall that happens to be near
    o = m.origin
    cx, cy = (doors[0].center[0] - o.dx) / o.scale, (doors[0].center[1] - o.dy) / o.scale
    assert abs(cx - 7275) < 60 and abs(cy - 3500) < 60, (cx, cy)
    assert all(w.thickness < 0.3 for w in m.walls), [(w.id, w.thickness) for w in m.walls if w.thickness >= 0.3]
