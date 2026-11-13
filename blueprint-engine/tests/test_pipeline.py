import ezdxf
import pytest

from blueprint_engine.checks import failed
from blueprint_engine.draw import build_dxf, to_dxf_bytes, to_pdf, to_svg
from blueprint_engine.edits import EditError, apply_ops
from blueprint_engine.pipeline import generate
from blueprint_engine.schemas import Brief, EditOps

from briefs import FAMILY_3BR, FEASIBLE, NORTH_ENTRY_2BR, TOO_BIG


@pytest.mark.parametrize("name", sorted(FEASIBLE))
def test_feasible_briefs_pass_every_error_check(name):
    r = generate(FEASIBLE[name])
    assert r.issues == []
    assert r.model is not None
    assert failed(r.checks) == [], [c.message for c in failed(r.checks)]


def test_infeasible_brief_reports_area_fact():
    r = generate(TOO_BIG)
    assert r.model is None and not r.ok
    assert "buildable area" in r.issues[0]


def test_deterministic():
    a = generate(FAMILY_3BR).model
    b = generate(FAMILY_3BR).model
    assert a == b


def test_rooms_tile_the_footprint_without_overlap():
    m = generate(FAMILY_3BR).model
    fx0, fy0, fx1, fy1 = m.footprint
    total = sum((r.rect[2] - r.rect[0]) * (r.rect[3] - r.rect[1]) for r in m.rooms)
    assert total == pytest.approx((fx1 - fx0) * (fy1 - fy0), rel=1e-6)


def test_side_constraint_and_entrance():
    m = generate(NORTH_ENTRY_2BR).model
    assert "west" in m.room("kitchen").exterior_sides
    entrance = next(o for o in m.openings if o.kind == "entrance")
    assert entrance.wall_id == "ext_north"


def test_revision_moves_one_room_and_keeps_the_rest_stable():
    before = generate(FAMILY_3BR)
    ops = EditOps.model_validate({"ops": [{"op": "set_side", "room": "kitchen", "side": "north"}]})
    brief = apply_ops(FAMILY_3BR, ops)
    after = generate(brief, previous=before.model)
    assert failed(after.checks) == []
    assert "north" in after.model.room("kitchen").exterior_sides
    # stability: most rooms that were not mentioned keep their position
    unchanged = sum(
        1 for r in before.model.rooms
        if r.id not in ("kitchen", "hall") and after.model.room(r.id).rect == r.rect
    )
    assert unchanged >= 3


def test_outputs_render(tmp_path):
    m = generate(FAMILY_3BR).model
    doc, sheet = build_dxf(m)
    dxf = to_dxf_bytes(doc)
    p = tmp_path / "plan.dxf"
    p.write_bytes(dxf)
    reread = ezdxf.readfile(p)
    layers = {e.dxf.layer for e in reread.modelspace()}
    assert {"A-WALL-EXTR", "A-WALL-INTR", "A-WALL-PATT", "A-DOOR", "A-GLAZ", "A-AREA-BNDY", "A-ANNO-DIMS", "A-ANNO-DIMS-OVAL", "A-ANNO-BORD", "A-ANNO-NARW", "A-ANNO-TTLB"} <= layers
    # every door, window and fixture is one named block reference with attributes
    refs = [e for e in reread.modelspace() if e.dxftype() == "INSERT"]
    assert len(refs) == len(m.fixtures) + sum(o.kind != "open" for o in m.openings)
    assert len({e.dxf.name for e in refs}) == len(refs)
    tables = [e for e in refs if e.dxf.layer == "A-FURN-TABL"]
    assert tables and all({a.dxf.tag: a.dxf.text for a in e.attribs}["ROOM"] for e in tables)
    assert {"A-DOOR-SWNG"} <= {e.dxf.layer for r in refs if r.dxf.layer == "A-DOOR" for e in r.block()}
    assert to_svg(doc, sheet).lstrip().startswith("<?xml")
    assert to_pdf(doc, sheet).startswith(b"%PDF")


def test_brief_rejects_unknown_adjacency():
    with pytest.raises(ValueError):
        Brief.model_validate({
            "lot": {"width": 10, "depth": 10},
            "rooms": [{"id": "a", "name": "A", "type": "living", "target_area": 20, "adjacent_to": ["zz"]}],
        })


def _with_ensuite_and_closet() -> Brief:
    d = FAMILY_3BR.model_dump()
    for r in d["rooms"]:
        if r["id"] == "bath2":
            r["parent"], r["adjacent_to"] = "bed1", []
    d["rooms"].append({"id": "wic", "name": "Walk-in closet", "type": "storage", "target_area": 3, "parent": "bed1"})
    return Brief.model_validate(d)


def test_nested_rooms_sit_inside_their_parent_block_and_open_only_into_it():
    brief = _with_ensuite_and_closet()
    r = generate(brief)
    assert failed(r.checks) == [], [c.message for c in failed(r.checks)]
    m = r.model
    for child in ("bath2", "wic"):
        doors = [o for o in m.openings if o.kind == "door" and child in o.rooms]
        assert [set(o.rooms) for o in doors] == [{"bed1", child}]
        assert any(c.rule == "nested" and c.ok and c.room_id == child for c in r.checks)
    # the rooms still tile the footprint
    fx0, fy0, fx1, fy1 = m.footprint
    total = sum((p.rect[2] - p.rect[0]) * (p.rect[3] - p.rect[1]) for p in m.rooms)
    assert total == pytest.approx((fx1 - fx0) * (fy1 - fy0), rel=1e-6)


def test_nested_room_without_hall_keeps_the_door_chain():
    d = FEASIBLE["studio"].model_dump()
    d["rooms"].append({"id": "pantry", "name": "Pantry", "type": "storage", "target_area": 2, "parent": "kitchen"})
    r = generate(Brief.model_validate(d))
    assert r.model is not None
    assert failed(r.checks) == [], [c.message for c in failed(r.checks)]


def test_brief_rejects_bad_nesting():
    base = FAMILY_3BR.model_dump()
    for bad in ({"id": "bed2", "parent": "bed1"},  # a bedroom cannot be nested
                {"id": "bath2", "parent": "zz"},  # unknown parent
                {"id": "bath2", "parent": "bed1", "side": "north"}):  # nested rooms follow the parent
        d = {**base, "rooms": [{**r, **bad} if r["id"] == bad["id"] else r for r in base["rooms"]]}
        with pytest.raises(ValueError):
            Brief.model_validate(d)


def test_fixtures_default_by_type_and_stay_inside_their_room():
    assert FAMILY_3BR.room("bed1").fixtures == ["bed", "wardrobe"]
    assert FAMILY_3BR.room("bath1").fixtures == ["toilet", "basin", "shower"]
    m = generate(FAMILY_3BR).model
    assert m.unplaced_fixtures == []
    assert {f.kind for f in m.fixtures if f.room_id == "kitchen"} == {"kitchen_counter", "sink", "stove", "fridge"}
    for f in m.fixtures:
        x0, y0, x1, y1 = m.room(f.room_id).net_rect
        assert x0 - 1e-6 <= f.rect[0] and f.rect[2] <= x1 + 1e-6 and y0 - 1e-6 <= f.rect[1] and f.rect[3] <= y1 + 1e-6
    same_room = {}
    for f in m.fixtures:
        for g in same_room.get(f.room_id, []):
            assert not (f.rect[0] < g.rect[2] - 1e-6 and g.rect[0] < f.rect[2] - 1e-6
                        and f.rect[1] < g.rect[3] - 1e-6 and g.rect[1] < f.rect[3] - 1e-6), (f.id, g.id)
        same_room.setdefault(f.room_id, []).append(f)
    doc, _ = build_dxf(m)
    assert any(e.dxf.layer.startswith(("A-FURN", "P-FIXT", "A-FLOR-CASE", "A-EQPM")) for e in doc.modelspace())


def test_explicit_empty_fixtures_and_unfit_fixture_warns():
    d = FAMILY_3BR.model_dump()
    for r in d["rooms"]:
        if r["id"] == "entry":
            r["fixtures"] = ["bathtub", "dining_table", "bed"]
        if r["id"] == "living":
            r["fixtures"] = []
    r = generate(Brief.model_validate(d))
    assert not [f for f in r.model.fixtures if f.room_id == "living"]
    assert r.model.unplaced_fixtures
    assert any(c.rule == "fixture_fit" and not c.ok and c.severity == "warning" for c in r.checks)
    assert failed(r.checks) == []


def test_edit_ops_for_nesting_and_fixtures():
    ops = EditOps.model_validate({"ops": [
        {"op": "set_inside", "room": "ensuite", "inside": "master bedroom"},
        {"op": "add_room", "name": "Walk-in closet", "type": "storage", "target_area": 3, "inside": "Master bedroom"},
        {"op": "add_fixture", "room": "bedroom 2", "fixture": "desk"},
        {"op": "remove_fixture", "room": "bathroom", "fixture": "shower"},
        {"op": "add_fixture", "room": "bathroom", "fixture": "bathtub"},
    ]})
    b = apply_ops(FAMILY_3BR, ops)
    assert b.room("bath2").parent == "bed1" and b.room("walk_in_closet").parent == "bed1"
    assert b.room("bed2").fixtures == ["bed", "wardrobe", "desk"]
    assert b.room("bath1").fixtures == ["toilet", "basin", "bathtub"]
    r = generate(b)
    assert failed(r.checks) == [], [c.message for c in failed(r.checks)]
    # removing the parent puts its nested rooms back on the hall
    b2 = apply_ops(b, EditOps.model_validate({"ops": [{"op": "remove_room", "room": "master bedroom"}]}))
    assert b2.room("bath2").parent is None
    with pytest.raises(EditError):
        apply_ops(FAMILY_3BR, EditOps.model_validate({"ops": [{"op": "set_inside", "room": "bedroom 2", "inside": "bed1"}]}))
    with pytest.raises(EditError):
        apply_ops(FAMILY_3BR, EditOps.model_validate({"ops": [{"op": "remove_fixture", "room": "living", "fixture": "bed"}]}))


def test_add_fixture_is_idempotent():
    ops = EditOps.model_validate({"ops": [{"op": "add_fixture", "room": "bathroom", "fixture": "toilet"}]})
    assert apply_ops(FAMILY_3BR, ops).room("bath1").fixtures == ["toilet", "basin", "shower"]


def test_plans_are_drawn_at_1_to_50_on_a_sheet_that_fits():
    for brief in FEASIBLE.values():
        _, sheet = build_dxf(generate(brief).model)
        assert sheet.denom == 50
        assert sheet.paper in ("A3", "A2", "A1", "A0")


def test_defensive_rules_fire_on_a_broken_model():
    from blueprint_engine.checks import run_checks

    m = generate(FAMILY_3BR).model.model_copy(deep=True)
    living = m.room("living")
    kitchen = m.room("kitchen")
    kitchen.rect = (kitchen.rect[0], kitchen.rect[1], kitchen.rect[2] + 1.0, kitchen.rect[3])  # into its neighbour
    fs = [f for f in m.fixtures if f.room_id == "living"]
    door = next(o for o in m.openings if o.id == "door_living")
    fs[0].rect = (door.center[0] - 0.3, living.net_rect[3] - 0.5, door.center[0] + 0.3, living.net_rect[3])
    fs[1].rect = fs[0].rect  # two fixtures on top of each other, in the door swing
    m.openings.append(door.model_copy(update={"id": "bad", "width": 0.0}))
    rules = {c.rule for c in failed(run_checks(FAMILY_3BR, m))}
    assert {"overlap", "coverage", "fixture_overlap", "fixture_door", "element_info"} <= rules


def test_design_warning_for_a_bedroom_on_the_street():
    d = FAMILY_3BR.model_dump()
    for r in d["rooms"]:
        if r["id"] == "bed3":
            r["side"] = None
    d["rooms"] = [r for r in d["rooms"] if r["type"] != "bedroom" or r["id"] == "bed3"]
    for r in d["rooms"]:
        r["adjacent_to"] = [a for a in r["adjacent_to"] if a in {x["id"] for x in d["rooms"]}]
    m = generate(Brief.model_validate(d))
    # whatever the layout, the rule reports the bedroom's relation to the street as a design warning
    assert any(c.rule == "bedroom_street" and c.category == "design" for c in m.checks)


def test_ifc_export_matches_the_model(tmp_path):
    import ifcopenshell
    import ifcopenshell.util.element as eu

    from blueprint_engine.ifc import to_ifc

    model = generate(_with_ensuite_and_closet()).model
    path = tmp_path / "plan.ifc"
    path.write_bytes(to_ifc(model, "Family house"))
    f = ifcopenshell.open(str(path))
    assert f.schema == "IFC4"
    assert len(f.by_type("IfcWall")) == len(model.walls)
    doors = [o for o in model.openings if o.kind != "window"]
    windows = [o for o in model.openings if o.kind == "window"]
    assert len(f.by_type("IfcDoor")) == len(doors) and len(f.by_type("IfcWindow")) == len(windows)
    assert len(f.by_type("IfcOpeningElement")) == len(model.openings)
    assert all(o.HasFillings for o in f.by_type("IfcOpeningElement"))
    spaces = {s.Name: s for s in f.by_type("IfcSpace")}
    assert set(spaces) == {r.name for r in model.rooms}
    for r in model.rooms:
        q = eu.get_psets(spaces[r.name])["Qto_SpaceBaseQuantities"]
        assert abs(q["NetFloorArea"] - r.net_area) < 0.01
    assert len(f.by_type("IfcSlab")) == 1
    furnishing = f.by_type("IfcFurniture") + f.by_type("IfcSanitaryTerminal") + f.by_type("IfcElectricAppliance")
    assert len(furnishing) == len(model.fixtures) > 0
    toilet = next(e for e in f.by_type("IfcSanitaryTerminal") if e.PredefinedType == "TOILETPAN")
    assert eu.get_container(toilet).is_a("IfcSpace")


def test_open_plan_removes_the_walls_between_hall_living_kitchen_and_dining():
    brief = apply_ops(FAMILY_3BR, EditOps.model_validate({"ops": [{"op": "set_open_plan", "open": True}]}))
    assert brief.open_plan and [r.id for r in brief.rooms] == [r.id for r in FAMILY_3BR.rooms]
    r = generate(brief)
    assert failed(r.checks) == [], [c.message for c in failed(r.checks)]
    m = r.model
    types = {pr.id: pr.type for pr in m.rooms}
    public = {"corridor", "entry", "living", "kitchen", "dining"}
    opens = [o for o in m.openings if o.kind == "open"]
    assert opens and all(types[a] in public and types[b] in public for a, b in (o.rooms for o in opens))
    open_pairs = {frozenset(o.rooms) for o in opens}
    doors = [o for o in m.openings if o.kind == "door"]
    assert not any(frozenset(o.rooms) in open_pairs for o in doors)  # no door where the wall is gone
    # bedrooms and bathrooms keep their doors
    assert {types[o.swing_into] for o in doors} >= {"bedroom", "bathroom"}
    # the plan without open_plan has no open passages
    assert not [o for o in generate(FAMILY_3BR).model.openings if o.kind == "open"]
    build_dxf(m)  # draws without a door leaf for the passages

    from blueprint_engine.ifc import to_ifc
    assert b"IFCOPENINGELEMENT" in to_ifc(m)


def test_the_hall_cannot_be_edited_and_says_how_to_open_it():
    with pytest.raises(EditError, match="set_open_plan"):
        apply_ops(FAMILY_3BR, EditOps.model_validate({"ops": [{"op": "remove_room", "room": "Hall"}]}))
