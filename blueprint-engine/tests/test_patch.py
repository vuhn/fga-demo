"""Writing edits of an imported plan back into its original DXF."""

from __future__ import annotations

import pytest
from briefs import FEASIBLE

from fga_engine.infrastructure.dxf_patch import ADDED_LAYER, patch_dxf
from fga_engine.infrastructure.dxf_plan import import_dxf, read_doc
from fga_engine.infrastructure.draw import build_dxf, to_dxf_bytes
from fga_engine.pipeline import generate
from fga_engine.domain.schemas import BuildingModel


@pytest.fixture(scope="module")
def original():
    """Our own export of a plan with fixtures, imported: blocks with handles, labels, doors and windows."""
    for brief in FEASIBLE.values():
        g = generate(brief).model
        if g is not None and len(g.fixtures) >= 4 and sum(1 for o in g.openings if o.kind == "window") >= 1:
            doc, _ = build_dxf(g)
            data = to_dxf_bytes(doc)
            return data, import_dxf(data).plans[0].model
    raise AssertionError("no brief with fixtures")


def _entities(data: bytes) -> dict[str, tuple]:
    return {e.dxf.handle: (e.dxftype(), e.dxf.layer) for e in read_doc(data).modelspace()}


def _edit(base, fn):
    cur = base.model_copy(deep=True)
    fn(cur)
    return cur


def test_provenance_is_recorded(original):
    _, m = original
    assert m.origin is not None and m.origin.scale == 1.0
    assert all(f.src for f in m.fixtures)
    assert all(o.src for o in m.openings)
    assert any(r.src for r in m.rooms)


def test_no_change_leaves_the_file_as_it_was(original):
    data, base = original
    out, rep = patch_dxf(data, base, base.model_copy(deep=True))
    assert rep.applied == rep.added == rep.skipped == []
    assert out == data  # byte for byte: nothing was written, so the file is not rewritten
    assert _entities(out) == _entities(data)


def test_rename_changes_only_the_label(original):
    data, base = original
    room = next(r for r in base.rooms if r.src)
    out, rep = patch_dxf(data, base, _edit(base, lambda c: setattr(next(r for r in c.rooms if r.id == room.id), "name", "Study Nook")))
    assert rep.applied == [f"room {room.name}: renamed to 'Study Nook'"] and not rep.skipped
    assert _entities(out) == _entities(data)  # nothing added or removed
    assert import_dxf(out).plans[0].model.room(room.id).name == "Study Nook"


def test_every_opening_can_be_removed_without_merging_rooms(original):
    data, base = original
    for o in base.openings:
        out, rep = patch_dxf(data, base, _edit(base, lambda c: c.openings.remove(next(x for x in c.openings if x.id == o.id))))
        again = import_dxf(out).plans[0].model
        assert len(again.rooms) == len(base.rooms), (o.id, o.kind, rep)
        assert len(again.openings) == len(base.openings) - 1, (o.id, o.kind)
        assert not rep.skipped, rep.skipped


def test_window_resize_keeps_the_position(original):
    data, base = original
    win = next(o for o in base.openings if o.kind == "window")
    wider = round(win.width + 0.3, 3)
    out, rep = patch_dxf(data, base, _edit(base, lambda c: setattr(next(x for x in c.openings if x.id == win.id), "width", wider)))
    assert not rep.skipped, rep.skipped
    got = next(o for o in import_dxf(out).plans[0].model.openings if o.kind == "window" and
               abs(o.center[0] - win.center[0]) < 0.15 and abs(o.center[1] - win.center[1]) < 0.15)
    assert abs(got.width - wider) < 0.08, (got.width, wider)


def test_fixture_move_remove_and_add(original):
    data, base = original
    moved, gone = base.fixtures[0], base.fixtures[1]

    def go(c):
        f = next(x for x in c.fixtures if x.id == moved.id)
        x0, y0, x1, y1 = f.rect
        f.rect = (x0, y0 + 0.05, x1, y1 + 0.05)
        c.fixtures.remove(next(x for x in c.fixtures if x.id == gone.id))

    out, rep = patch_dxf(data, base, _edit(base, go))
    assert any("moved" in s for s in rep.applied) and any("removed" in s for s in rep.applied), rep
    ents = _entities(out)
    assert gone.src[0] not in ents and moved.src[0] in ents
    # an added fixture of a kind the file already has is a copy of that block
    twin = moved.model_copy(update={"id": "new-1", "src": [], "rect": (moved.rect[0], moved.rect[1] + 1.0, moved.rect[2], moved.rect[3] + 1.0)})
    out2, rep2 = patch_dxf(data, base, _edit(base, lambda c: c.fixtures.append(twin)))
    assert len(rep2.added) == 1 and "copied from the block" in rep2.added[0], rep2
    assert len(_entities(out2)) == len(_entities(data)) + 1


def test_added_fixture_without_a_template_is_drawn_on_the_added_layer(original):
    data, base = original
    kinds = {f.kind for f in base.fixtures}
    new_kind = next(k for k in ("piano", "bathtub", "sofa", "bed_double", "toilet") if k not in kinds and k in __import__("fga_engine.domain.schemas", fromlist=["FIXTURE_KINDS"]).FIXTURE_KINDS)
    room = base.rooms[0]
    x0, y0, x1, y1 = room.net_rect
    fx = base.fixtures[0].model_copy(update={"id": "n1", "kind": new_kind, "src": [], "room_id": room.id,
                                              "rect": (x0 + 0.3, y0 + 0.3, x0 + 0.8, y0 + 0.8)})
    out, rep = patch_dxf(data, base, _edit(base, lambda c: c.fixtures.append(fx)))
    assert rep.added and ADDED_LAYER in rep.added[0]
    assert any(layer == ADDED_LAYER for _, layer in _entities(out).values())


def test_unsupported_changes_are_reported_not_dropped(original):
    data, base = original

    def go(c):
        c.walls[0].end = (c.walls[0].end[0] + 0.5, c.walls[0].end[1])
        c.rooms[0].type = "other" if c.rooms[0].type != "other" else "bedroom"

    _, rep = patch_dxf(data, base, _edit(base, go))
    assert any("wall" in s for s in rep.skipped) and any("type" in s for s in rep.skipped), rep.skipped


def test_a_generated_plan_cannot_be_patched(original):
    data, base = original
    gen = base.model_copy(update={"source": "generated"})
    with pytest.raises(ValueError):
        patch_dxf(data, gen, gen)


# ---------------------------------------------------------------- direct edit ops and endpoints


def _apply(stub, model, ops):
    """Apply direct ops over gRPC; the model as the Engine returns it. A refusal raises grpc.RpcError."""
    import json

    from fga.demo.engine.v1 import engine_pb2 as e

    step = stub.ApplyPlanOps(e.ApplyPlanOpsRequest(model_json=model.model_dump_json().encode(), ops_json=json.dumps({"ops": ops}).encode()))
    return json.loads(step.model_json)


def _refused(stub, model, ops) -> str:
    import grpc

    with pytest.raises(grpc.RpcError) as ei:
        _apply(stub, model, ops)
    assert ei.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    return ei.value.details()


def _patch(stub, data, base_json: bytes, current_json: bytes):
    import json

    from fga.demo.common.v1 import common_pb2 as c
    from fga.demo.engine.v1 import engine_pb2 as e

    r = stub.PatchDXF(e.PatchDXFRequest(original=c.Upload(name="p.dxf", mime="image/vnd.dxf", data=data),
                                        base_model_json=base_json, current_model_json=current_json))
    return r.dxf, json.loads(r.report_json)


def test_direct_edit_moves_turns_and_slides_without_an_llm(original, stub):
    _, base = original
    fx = base.fixtures[0]
    room = base.room(fx.room_id)
    # a step toward the room's centre is always legal for a fixture that stands against a wall
    cx, cy = (room.net_rect[0] + room.net_rect[2]) / 2, (room.net_rect[1] + room.net_rect[3]) / 2
    fcx, fcy = (fx.rect[0] + fx.rect[2]) / 2, (fx.rect[1] + fx.rect[3]) / 2
    edited = _apply(stub, base, [{"op": "move_fixture", "fixture": fx.id, "dx": 0.02 if cx > fcx else -0.02, "dy": 0.0}])
    assert edited["fixtures"][0]["id"] == fx.id
    moved = next(f for f in edited["fixtures"] if f["id"] == fx.id)
    assert moved["src"] == fx.src and moved["rect"] != list(fx.rect)

    o = next(o for o in base.openings if o.kind == "window")
    _refused(stub, base, [{"op": "move_opening", "opening": o.id, "offset": 50}])  # beyond the field limit
    assert "clear of the wall ends" in _refused(stub, base, [{"op": "move_opening", "opening": o.id, "offset": 29}])
    assert "leave" in _refused(stub, base, [{"op": "move_fixture", "fixture": fx.id, "dx": 25, "dy": 0}])


def test_a_direct_edit_then_export_writes_the_change_into_the_file(original, stub):
    import grpc

    data, base = original
    o = next(o for o in base.openings if o.kind == "window")
    w = next(w for w in base.walls if w.id == o.wall_id)
    length = ((w.end[0] - w.start[0]) ** 2 + (w.end[1] - w.start[1]) ** 2) ** 0.5
    # slide towards the nearer wall end only as far as it allows: a small, legal step
    step = 0.05 if length > o.width + 1.0 else 0.0
    cur = _apply(stub, base, [{"op": "move_opening", "opening": o.id, "offset": step}, {"op": "rename_room", "room": base.rooms[0].name, "name": "Den"}])
    import json

    out, report = _patch(stub, data, base.model_dump_json().encode(), json.dumps(cur).encode())
    assert any("renamed to 'Den'" in s for s in report["applied"])
    assert import_dxf(out).plans[0].model.rooms[0].name == "Den"
    with pytest.raises(grpc.RpcError) as ei:
        _patch(stub, data, b"{}", b"{}")
    assert ei.value.code() == grpc.StatusCode.INVALID_ARGUMENT


def test_delete_fixture_by_id_and_the_chat_help_lists_the_new_ops(original, stub):
    from fga_engine.domain.plan_edits import ops_help

    _, base = original
    fx = base.fixtures[-1]
    edited = _apply(stub, base, [{"op": "delete_fixture", "fixture": fx.id}])
    assert fx.id not in {f["id"] for f in edited["fixtures"]}
    _refused(stub, base, [{"op": "delete_fixture", "fixture": "nope"}])
    help_ = ops_help()
    assert all(op in help_ for op in ("move_fixture", "rotate_fixture", "move_opening", "delete_fixture"))


# ---------------------------------------------------------------- a created plan behaves like an imported one


def test_a_created_plan_is_an_editable_cad_plan(stub):
    import json

    from fga.demo.engine.v1 import engine_pb2 as e

    brief = next(iter(FEASIBLE.values()))
    r = stub.CreatePlan(e.CreatePlanRequest(brief_json=brief.model_dump_json().encode()))
    model = BuildingModel.model_validate_json(r.model_json)
    assert model.source == "imported" and model.origin is not None and all(f.src for f in model.fixtures)
    assert r.ok and not r.issues
    # the file is the plan's original: a direct edit of the model is written back into it
    room = model.rooms[0]
    cur = _apply(stub, model, [{"op": "rename_room", "room": room.name, "name": "Studio nook"}])
    out, _ = _patch(stub, r.dxf, model.model_dump_json().encode(), json.dumps(cur).encode())
    assert import_dxf(out).plans[0].model.room(room.id).name == "Studio nook"


def test_a_brief_that_cannot_be_built_creates_nothing(stub):
    from fga.demo.engine.v1 import engine_pb2 as e

    brief = next(iter(FEASIBLE.values())).model_dump(mode="json")
    brief["lot"] = {**brief["lot"], "width": 5.0, "depth": 5.0}
    import json

    r = stub.CreatePlan(e.CreatePlanRequest(brief_json=json.dumps(brief).encode()))
    assert not r.dxf and not r.model_json and r.issues


def test_a_plan_made_from_a_brief_can_be_drawn_into_an_editable_cad_plan(stub):
    import grpc

    from fga.demo.engine.v1 import engine_pb2 as e

    brief = next(iter(FEASIBLE.values()))
    gen = generate(brief).model
    r = stub.DrawPlan(e.DrawPlanRequest(model_json=gen.model_dump_json().encode(), title="Old house"))
    model = BuildingModel.model_validate_json(r.model_json)
    assert model.source == "imported" and model.origin is not None and r.dxf.startswith(b"  0")
    assert len(model.rooms) == len(gen.rooms) and len(model.fixtures) == len(gen.fixtures)
    # a plan that is already a CAD plan is refused
    with pytest.raises(grpc.RpcError) as ei:
        stub.DrawPlan(e.DrawPlanRequest(model_json=model.model_dump_json().encode()))
    assert ei.value.code() == grpc.StatusCode.INVALID_ARGUMENT and "already" in ei.value.details()


def test_an_open_plan_reads_back_as_fewer_spaces_and_says_so():
    from fga_engine.wire import build_engine

    brief = next(b for b in FEASIBLE.values() if len(b.rooms) >= 4).model_copy(update={"open_plan": True})
    gen = generate(brief).model
    out = build_engine().plan_drawer.draw_plan(gen, "Open")
    assert len(out.result.model.rooms) < len(gen.rooms)
    assert any("open plan" in n for n in out.result.notes)
