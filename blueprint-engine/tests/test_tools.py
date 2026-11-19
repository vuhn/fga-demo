"""T6: the tool listing, the catalog, draft normalization and the brief-ops result."""

import pytest

from fga_engine.domain import plan_edits
from fga_engine.domain.draft import DraftBrief, DraftRoom
from fga_engine.domain.edits import EditError, UnsupportedEdit
from fga_engine.domain.op_docs import op_classes, op_name
from fga_engine.domain.schemas import EditOps, MIN_AREA
from fga_engine.domain.tools import get_catalog, list_tools
from fga_engine.wire import build_engine
from briefs import STUDIO


def _brief():
    return STUDIO.model_copy(deep=True)


def test_list_tools_covers_every_brief_and_plan_op():
    tl = list_tools()
    names = {(t.kind, t.name) for t in tl.tools}
    for cls in op_classes(EditOps):
        assert ("brief_op", op_name(cls)) in names
    for cls in op_classes(plan_edits.PlanOps):
        assert ("plan_op", op_name(cls)) in names
    assert ("draft", "draft_brief") in names
    assert tl.ops_json_schema and tl.plan_ops_json_schema and tl.draft_json_schema


def test_each_tool_has_schema_signature_and_description():
    for t in list_tools().tools:
        assert t.json_schema.get("properties"), t.name
        assert t.signature.startswith(t.name + " {"), t.name
        assert t.description, t.name
    sig = {t.name: t.signature for t in list_tools().tools if t.kind == "brief_op"}
    assert sig["resize_room"] == "resize_room {room, target_area}"


def test_catalog_snapshot():
    c = get_catalog()
    assert c.min_area == MIN_AREA
    assert c.max_nested == 2 and c.nestable_types == ["bathroom", "laundry", "storage"]
    assert "fixture_toilet" in {r.code for r in c.rate_catalog}
    assert c.priced_fixtures == sorted(c.priced_fixtures) and "toilet" in c.priced_fixtures
    assert set(c.default_markups) and all(isinstance(v, float) for v in c.default_markups.values())
    assert c.default_fixtures["bathroom"] == "toilet,basin,shower"


def test_error_codes_are_distinct():
    assert EditError.code == "edit_error" and UnsupportedEdit.code == "unsupported"
    assert issubclass(UnsupportedEdit, EditError)


def test_normalize_draft_drops_halls_and_raises_minimums():
    e = build_engine()
    draft = DraftBrief(title="t", lot_width=20, lot_depth=30, entrance_side="south", rooms=[
        DraftRoom(name="Hall", type="entry", target_area=5),
        DraftRoom(name="Bedroom", type="bedroom", target_area=2)])
    brief, notes = e.draft_normalizer.normalize_draft(draft)
    assert [r.name for r in brief.rooms] == ["Bedroom"]
    assert brief.rooms[0].target_area == MIN_AREA["bedroom"]
    assert any("hall" in n for n in notes) and any("minimum" in n for n in notes)
    with pytest.raises(EditError):
        e.draft_normalizer.normalize_draft(draft.model_copy(update={"rooms": draft.rooms[:1]}))


def test_apply_brief_ops_reports_score_targets_and_change():
    e = build_engine()
    brief = _brief()
    room = brief.rooms[0]
    ops = EditOps.model_validate({"ops": [{"op": "resize_room", "room": room.name, "target_area": room.target_area + 1}]})
    r = e.brief_ops_applier.apply_brief_ops(brief, ops)
    assert r.changed and r.brief.rooms[0].target_area == room.target_area + 1
    assert r.room_targets[room.id] == room.target_area + 1
    assert r.total_target_area == sum(x.target_area for x in r.brief.rooms)
    assert r.issue_score >= 0 and r.checks_line.startswith(("Checks", "The brief"))
    assert (r.error_rules == []) == (r.issue_score < 10 and r.model is not None)
    same = e.brief_ops_applier.apply_brief_ops(r.brief, ops)
    assert not same.changed


def test_apply_brief_ops_raises_unsupported_and_edit_errors():
    e = build_engine()
    brief = _brief()
    with pytest.raises(UnsupportedEdit):
        e.brief_ops_applier.apply_brief_ops(brief, EditOps.model_validate({"ops": [{"op": "unsupported", "reason": "x"}]}))
    with pytest.raises(EditError):
        e.brief_ops_applier.apply_brief_ops(brief, EditOps.model_validate(
            {"ops": [{"op": "remove_room", "room": "no such room"}]}))


def test_generate_brief_matches_generate():
    e = build_engine()
    brief = _brief()
    r = e.generator.generate_brief(brief)
    assert r.ok == e.generator.generate(brief).result.ok and r.model is not None


# --- deterministic checks that used to be exercised through the HTTP app (T16) -------------------------------------


def _conflicting_sides() -> dict:
    """Four public rooms all asking for the east wall: three side checks fail."""
    from briefs import FAMILY_3BR

    d = FAMILY_3BR.model_dump()
    for r in d["rooms"]:
        if r["id"] in ("kitchen", "living", "dining", "entry"):
            r["side"] = "east"
    return d


def test_checks_carry_issue_ids_categories_and_hints():
    from fga_engine.domain.schemas import Brief

    res = build_engine().generator.generate_brief(Brief.model_validate(_conflicting_sides()))
    side = [x for x in res.checks if x.rule == "side" and not x.ok]
    assert side and all(x.issue_id.startswith("side:") and x.hint.startswith("set_side") for x in side)
    ids = [x.issue_id for x in res.checks]
    assert len(ids) == len(set(ids))
    assert {x.category for x in res.checks} >= {"program", "geometry", "collision", "information", "design"}


def test_a_draft_nests_rooms_merges_fixtures_and_builds():
    e = build_engine()
    draft = DraftBrief.model_validate({
        "title": "Two bed", "lot_width": 18, "lot_depth": 28, "entrance_side": "south",
        "rooms": [
            {"name": "Living", "type": "living", "target_area": 24},
            {"name": "Master bedroom", "type": "bedroom", "target_area": 15, "fixtures": ["desk", "bed"]},
            {"name": "Ensuite", "type": "bathroom", "target_area": 4.5, "inside": "master bedroom", "side": "north"},
            {"name": "Bedroom 2", "type": "bedroom", "target_area": 11, "inside": "Master bedroom"},
            {"name": "Bathroom", "type": "bathroom", "target_area": 5},
        ]})
    brief, notes = e.draft_normalizer.normalize_draft(draft)
    rooms = {r.id: r for r in brief.rooms}
    assert rooms["ensuite"].parent == "master_bedroom" and rooms["ensuite"].side is None
    assert rooms["bedroom_2"].parent is None  # a bedroom cannot be nested
    assert rooms["master_bedroom"].fixtures == ["bed", "wardrobe", "desk"]
    assert any("Bedroom 2" in n for n in notes)
    built = e.generator.generate_brief(brief)
    assert built.ok, [c.message for c in built.checks if not c.ok]


def test_a_draft_can_ask_for_open_plan():
    draft = DraftBrief.model_validate({
        "title": "Open house", "lot_width": 20, "lot_depth": 30, "entrance_side": "south", "open_plan": True,
        "rooms": [{"name": "Living", "type": "living", "target_area": 22}, {"name": "Kitchen", "type": "kitchen", "target_area": 10},
                  {"name": "Bedroom", "type": "bedroom", "target_area": 12}]})
    brief, _ = build_engine().draft_normalizer.normalize_draft(draft)
    assert brief.open_plan is True


def test_set_open_plan_opens_the_walls_between_the_public_rooms():
    from briefs import FAMILY_3BR

    r = build_engine().brief_ops_applier.apply_brief_ops(
        FAMILY_3BR.model_copy(deep=True), EditOps.model_validate({"ops": [{"op": "set_open_plan", "open": True}]}))
    assert r.brief.open_plan is True and r.model is not None and any(o.kind == "open" for o in r.model.openings)
