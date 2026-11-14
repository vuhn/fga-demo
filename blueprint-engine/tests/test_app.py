"""HTTP engine with a scripted LLM (no Ollama needed)."""

from fastapi.testclient import TestClient

from blueprint_engine.app import create_app
from blueprint_engine.llm_steps import DraftBrief
from blueprint_engine.schemas import EditOps

from briefs import FAMILY_3BR, SHRINKABLE, TOO_BIG


class FakeLLM:
    """Scripted answers in order; the last one repeats once the script runs out."""

    def __init__(self, *answers):
        self.answers = list(answers)
        self.calls = []
        self.prompts = []

    def structured(self, system, user, schema, *, images=None, vision=False):
        self.calls.append((schema.__name__, vision))
        self.prompts.append(user)
        answer = self.answers.pop(0) if len(self.answers) > 1 else self.answers[0]
        return schema.model_validate(answer)


REQ = {"reasoning": "a small house", "requirements": ["Living about 22 m²"], "assumptions": ["lot size not given"]}


def client(*answers):
    llm = FakeLLM(*answers)
    return TestClient(create_app(llm)), llm


def test_from_text_cleans_draft():
    c, _ = client({
        "title": "Cottage", "lot_width": 15, "lot_depth": 25, "entrance_side": "south",
        "rooms": [
            {"name": "Living room", "type": "living", "target_area": 22, "adjacent_to": ["Kitchen", "Garage"]},
            {"name": "Kitchen", "type": "kitchen", "target_area": 9},
            {"name": "Hallway", "type": "entry", "target_area": 6},
            {"name": "Bedroom", "type": "bedroom", "target_area": 7},
        ],
    })
    r = c.post("/v1/brief/from-text", json={"prompt": "small cottage"})
    assert r.status_code == 200, r.text
    body = r.json()
    ids = [x["id"] for x in body["brief"]["rooms"]]
    assert ids == ["living_room", "kitchen", "bedroom"]
    assert body["brief"]["rooms"][0]["adjacent_to"] == ["kitchen"]
    assert body["brief"]["rooms"][2]["target_area"] == 9  # raised to bedroom minimum
    assert any("Garage" in n for n in body["notes"]) and any("Hallway" in n for n in body["notes"])


def test_revise_applies_ops_and_regenerates():
    c, _ = client({"ops": [{"op": "set_side", "room": "the kitchen", "side": "north"}]})
    base = c.post("/v1/generate", json={"brief": FAMILY_3BR.model_dump()}).json()["result"]
    r = c.post("/v1/revise", json={"brief": base["brief"], "previous": base["model"], "message": "kitchen north"})
    assert r.status_code == 200, r.text
    res = r.json()["result"]
    assert res["ok"]
    kitchen = next(x for x in res["model"]["rooms"] if x["id"] == "kitchen")
    assert "north" in kitchen["exterior_sides"]


def test_revise_unknown_room_is_422():
    c, _ = client({"ops": [{"op": "remove_room", "room": "garage"}]})
    r = c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "remove the garage"})
    assert r.status_code == 422
    assert "garage" in r.json()["message"]


def test_unsupported_request_is_422():
    c, _ = client({"ops": [{"op": "unsupported", "reason": "a second storey"}]})
    r = c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "add a floor"})
    assert r.status_code == 422


def test_repair_only_when_requested_and_never_uses_the_llm():
    c, llm = client()
    plain = c.post("/v1/generate", json={"brief": TOO_BIG.model_dump()}).json()
    assert plain["result"]["model"] is None
    fixed = c.post("/v1/generate", json={"brief": SHRINKABLE.model_dump(), "repair": True}).json()
    assert llm.calls == []
    assert fixed["result"]["model"] is not None
    assert any("shrank rooms" in n for n in fixed["notes"])
    impossible = c.post("/v1/generate", json={"brief": TOO_BIG.model_dump(), "repair": True}).json()
    assert impossible["result"]["model"] is None
    assert "minimum sizes" in impossible["result"]["issues"][-1]


def test_render_formats():
    c, _ = client()
    model = c.post("/v1/generate", json={"brief": FAMILY_3BR.model_dump()}).json()["result"]["model"]
    for fmt, prefix in (("svg", b"<?xml"), ("pdf", b"%PDF"), ("dxf", b"  0"), ("ifc", b"ISO-10303-21")):
        r = c.post("/v1/render", json={"model": model, "format": fmt})
        assert r.status_code == 200 and r.content.lstrip().startswith(prefix.strip()), fmt


def test_harness_page_served():
    c, _ = client()
    assert c.get("/").status_code == 200


def test_schemas_are_json_schema_ready():
    # Ollama's `format` receives these; they must serialise.
    assert DraftBrief.model_json_schema()["type"] == "object"
    assert "ops" in EditOps.model_json_schema()["properties"]


def test_from_text_drops_sides_the_user_never_named():
    c, _ = client({
        "title": "T", "lot_width": 16, "lot_depth": 24, "entrance_side": "north",
        "rooms": [
            {"name": "Kitchen", "type": "kitchen", "target_area": 9, "side": "west"},
            {"name": "Bedroom", "type": "bedroom", "target_area": 12, "side": "east"},
        ],
    })
    body = c.post("/v1/brief/from-text", json={"prompt": "street on the north, kitchen on the west wall"}).json()
    sides = {r["name"]: r["side"] for r in body["brief"]["rooms"]}
    assert sides == {"Kitchen": "west", "Bedroom": None}


def test_from_text_nests_rooms_and_merges_fixtures():
    c, _ = client({
        "title": "Two bed", "lot_width": 18, "lot_depth": 28, "entrance_side": "south",
        "rooms": [
            {"name": "Living", "type": "living", "target_area": 24},
            {"name": "Master bedroom", "type": "bedroom", "target_area": 15, "fixtures": ["desk", "bed"]},
            {"name": "Ensuite", "type": "bathroom", "target_area": 4.5, "inside": "master bedroom", "side": "north"},
            {"name": "Bedroom 2", "type": "bedroom", "target_area": 11, "inside": "Master bedroom"},
            {"name": "Bathroom", "type": "bathroom", "target_area": 5},
        ],
    })
    r = c.post("/v1/brief/from-text", json={"prompt": "two bed house, ensuite in the master, desk in the master"})
    assert r.status_code == 200, r.text
    body = r.json()
    rooms = {x["id"]: x for x in body["brief"]["rooms"]}
    assert rooms["ensuite"]["parent"] == "master_bedroom" and rooms["ensuite"]["side"] is None
    assert rooms["bedroom_2"]["parent"] is None  # a bedroom cannot be nested
    assert rooms["master_bedroom"]["fixtures"] == ["bed", "wardrobe", "desk"]
    assert any("Bedroom 2" in n for n in body["notes"])
    g = c.post("/v1/generate", json={"brief": body["brief"]}).json()["result"]
    assert g["ok"], [x["message"] for x in g["checks"] if not x["ok"]]


# ---- Text2BIM-style agent loop (enhancer, retry with feedback, memory, reviewer) ----

def _conflicting_sides() -> dict:
    """Four public rooms all asking for the east wall: three side checks fail."""
    d = FAMILY_3BR.model_dump()
    for r in d["rooms"]:
        if r["id"] in ("kitchen", "living", "dining", "entry"):
            r["side"] = "east"
    return d


FREE_SIDES = {"ops": [{"op": "set_side", "room": n, "side": None} for n in ("Kitchen", "Dining", "Living")]}


def test_enhancer_requirements_feed_the_draft_and_assumptions_become_notes():
    c, llm = client(REQ, {
        "title": "Cottage", "lot_width": 15, "lot_depth": 25, "entrance_side": "south",
        "rooms": [{"name": "Living", "type": "living", "target_area": 22}],
    })
    r = c.post("/v1/brief/from-text", json={"prompt": "small cottage", "enhance": True})
    assert r.status_code == 200, r.text
    assert [n for n, _ in llm.calls] == ["Requirements", "DraftBrief"]
    assert "Requirements from the architect" in llm.prompts[1] and "Living about 22" in llm.prompts[1]
    assert "assumed: lot size not given" in r.json()["notes"]


def test_enhancer_is_off_by_default():
    c, llm = client({"title": "C", "lot_width": 15, "lot_depth": 25, "entrance_side": "south",
                     "rooms": [{"name": "Living", "type": "living", "target_area": 22}]})
    assert c.post("/v1/brief/from-text", json={"prompt": "cottage"}).status_code == 200
    assert [n for n, _ in llm.calls] == ["DraftBrief"]


def test_revise_retries_with_the_error_and_succeeds():
    c, llm = client({"ops": [{"op": "resize_room", "room": "garage", "target_area": 20}]},
                    {"ops": [{"op": "resize_room", "room": "living", "target_area": 26}]})
    r = c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "bigger lounge"})
    assert r.status_code == 200, r.text
    assert r.json()["llm_attempts"] == 2
    assert "Applying it failed" in llm.prompts[1] and "garage" in llm.prompts[1]


def test_revise_gives_up_after_three_attempts():
    c, llm = client({"ops": [{"op": "remove_room", "room": "garage"}]})
    r = c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "remove the garage"})
    assert r.status_code == 422 and "after 3 attempts" in r.json()["message"]
    assert len(llm.calls) == 3


def test_unsupported_is_not_retried():
    c, llm = client({"ops": [{"op": "unsupported", "reason": "a second storey"}]})
    assert c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "add a floor"}).status_code == 422
    assert len(llm.calls) == 1


def test_history_reaches_the_revise_prompt():
    c, llm = client({"ops": [{"op": "resize_room", "room": "living", "target_area": 26}]})
    c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(), "message": "a bit more",
                               "history": ["Three bedroom family house", "make the living room bigger"]})
    assert "Earlier requests" in llm.prompts[0] and "make the living room bigger" in llm.prompts[0]


def test_reviewer_keeps_an_improving_round():
    c, _ = client(FREE_SIDES, {"ops": [{"op": "resize_room", "room": "Living", "target_area": 24}]})
    r = c.post("/v1/review", json={"brief": _conflicting_sides(), "max_rounds": 3})
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["review_rounds"][0]["accepted"] is True
    assert body["review_rounds"][0]["score_after"] < body["review_rounds"][0]["score_before"]
    assert not [x for x in body["result"]["checks"] if x["rule"] == "side" and not x["ok"]]
    assert {x["id"] for x in body["result"]["brief"]["rooms"]} == {r["id"] for r in FAMILY_3BR.model_dump()["rooms"]}


def test_reviewer_refuses_forbidden_ops_and_never_returns_a_worse_plan():
    c, llm = client({"ops": [{"op": "remove_room", "room": "Kitchen"}]})
    r = c.post("/v1/review", json={"brief": _conflicting_sides()})
    assert r.status_code == 200, r.text
    body = r.json()
    assert len(llm.calls) == 3 and "may not use remove_room" in llm.prompts[1]
    assert any("no usable fix" in n for n in body["notes"])
    assert len(body["result"]["brief"]["rooms"]) == len(FAMILY_3BR.rooms)  # unchanged plan
    # an op that does not help is discarded
    c, _ = client({"ops": [{"op": "add_fixture", "room": "Living", "fixture": "desk"}]})
    body = c.post("/v1/review", json={"brief": _conflicting_sides()}).json()
    assert body["review_rounds"][0]["accepted"] is False
    assert all(r.get("fixtures") != ["sofa", "tv_unit", "desk"] for r in body["result"]["brief"]["rooms"])


def test_reviewer_caps_total_area_change():
    c, llm = client({"ops": [{"op": "resize_room", "room": "Living", "target_area": 80}]})
    body = c.post("/v1/review", json={"brief": _conflicting_sides()}).json()
    assert "stay within 15%" in llm.prompts[1]
    assert any("no usable fix" in n for n in body["notes"])


def test_checks_carry_issue_ids_categories_and_hints():
    c, _ = client()
    res = c.post("/v1/generate", json={"brief": _conflicting_sides()}).json()["result"]
    side = [x for x in res["checks"] if x["rule"] == "side" and not x["ok"]]
    assert side and all(x["issue_id"].startswith("side:") and x["hint"].startswith("set_side") for x in side)
    ids = [x["issue_id"] for x in res["checks"]]
    assert len(ids) == len(set(ids))
    assert {x["category"] for x in res["checks"]} >= {"program", "geometry", "collision", "information", "design"}


def test_reviewer_retries_an_answer_that_changes_nothing():
    same = {"ops": [{"op": "set_side", "room": "Kitchen", "side": "east"}]}
    c, llm = client(same, FREE_SIDES)
    body = c.post("/v1/review", json={"brief": _conflicting_sides(), "max_rounds": 1}).json()
    assert "change nothing" in llm.prompts[1]
    assert body["review_rounds"][0]["accepted"] is True


def test_removing_the_hall_is_steered_to_open_plan():
    c, llm = client({"ops": [{"op": "remove_room", "room": "Hall"}]}, {"ops": [{"op": "set_open_plan", "open": True}]})
    r = c.post("/v1/revise", json={"brief": FAMILY_3BR.model_dump(),
                                   "message": "Remove Hall partitions to create an open-plan living/kitchen/dining"})
    assert r.status_code == 200, r.text
    assert r.json()["result"]["brief"]["open_plan"] is True
    assert "set_open_plan" in llm.prompts[1]
    assert any(o["kind"] == "open" for o in r.json()["result"]["model"]["openings"])


def test_draft_can_ask_for_open_plan():
    c, _ = client({"title": "Open house", "lot_width": 20, "lot_depth": 30, "entrance_side": "south", "open_plan": True,
                   "rooms": [{"name": "Living", "type": "living", "target_area": 22},
                             {"name": "Kitchen", "type": "kitchen", "target_area": 10},
                             {"name": "Bedroom", "type": "bedroom", "target_area": 12}]})
    r = c.post("/v1/brief/from-text", json={"prompt": "open-plan 1 bed"})
    assert r.status_code == 200, r.text
    assert r.json()["brief"]["open_plan"] is True


def test_from_sketch_reads_dxf_without_vision():
    c, llm = client({
        "title": "Plan", "lot_width": 15, "lot_depth": 25, "entrance_side": "south",
        "rooms": [{"name": "Living room", "type": "living", "target_area": 22},
                  {"name": "Kitchen", "type": "kitchen", "target_area": 9},
                  {"name": "Bedroom", "type": "bedroom", "target_area": 12}],
    })
    import io
    import ezdxf

    doc = ezdxf.new()
    msp = doc.modelspace()
    msp.add_lwpolyline([(0, 0), (5000, 0), (5000, 4000), (0, 4000)], close=True)
    msp.add_text("Kitchen 9.0 m2")
    buf = io.StringIO()
    doc.write(buf)
    r = c.post("/v1/brief/from-sketch", files={"file": ("plan.dxf", buf.getvalue().encode(), "application/octet-stream")})
    assert r.status_code == 200, r.text
    assert llm.calls == [("DraftBrief", False)]  # text model, not vision
    assert "Kitchen 9.0 m2" in llm.prompts[0] and "20.0" in llm.prompts[0]
    assert any("DXF" in n for n in r.json()["notes"])


def test_from_sketch_rejects_broken_dxf():
    c, _ = client({})
    r = c.post("/v1/brief/from-sketch", files={"file": ("x.dxf", b"not a dxf", "application/octet-stream")})
    assert r.status_code == 422, r.text


# ---- streaming revise ----

def _lines(r):
    import json
    return [json.loads(x) for x in r.text.splitlines() if x]


def test_revise_stream_emits_progress_then_result():
    c, _ = client({"ops": [{"op": "set_side", "room": "the kitchen", "side": "north"}]})
    base = c.post("/v1/generate", json={"brief": FAMILY_3BR.model_dump()}).json()["result"]
    r = c.post("/v1/revise/stream", json={"brief": base["brief"], "previous": base["model"], "message": "kitchen north"})
    assert r.status_code == 200 and r.headers["content-type"].startswith("application/x-ndjson")
    evs = _lines(r)
    stages = [e["stage"] for e in evs if e["event"] == "progress"]
    assert stages[0] == "ask_model" and "ops" in stages and stages[-1] == "checks"
    assert [e["event"] for e in evs].count("result") == 1 and evs[-1]["event"] == "result"
    assert evs[-1]["data"]["result"]["model"] is not None


def test_revise_stream_reports_retry_and_error():
    bad = {"ops": [{"op": "set_side", "room": "the swimming pool", "side": "north"}]}
    c, _ = client(bad)
    base = c.post("/v1/generate", json={"brief": FAMILY_3BR.model_dump()}).json()["result"]
    r = c.post("/v1/revise/stream", json={"brief": base["brief"], "previous": base["model"], "message": "pool north"})
    evs = _lines(r)
    assert [e["stage"] for e in evs if e["event"] == "progress"].count("retry") == 2
    assert evs[-1]["event"] == "error" and evs[-1]["status"] == 422 and evs[-1]["error"] == "invalid_edit"


def test_revise_stream_empty_message_is_plain_422():
    c, _ = client({"ops": []})
    base = c.post("/v1/generate", json={"brief": FAMILY_3BR.model_dump()}).json()["result"]
    r = c.post("/v1/revise/stream", json={"brief": base["brief"], "previous": None, "message": "  "})
    assert r.status_code == 422 and r.json()["error"] == "invalid_edit"


def test_thinking_is_streamed_only_while_a_sink_listens():
    import json as _json
    import httpx
    from blueprint_engine import progress
    from blueprint_engine.llm import OllamaClient

    def handler(request):
        body = _json.loads(request.content)
        if body["stream"]:
            assert body["think"] is True
            lines = [{"message": {"thinking": "Let me "}}, {"message": {"thinking": "think."}},
                     {"message": {"content": '{"ops": [{"op": "set_side", "room": "kitchen", "side": "north"}]}'}}, {"message": {}, "done": True}]
            return httpx.Response(200, content="\n".join(_json.dumps(x) for x in lines))
        assert body["think"] is False
        return httpx.Response(200, json={"message": {"content": '{"ops": [{"op": "set_side", "room": "kitchen", "side": "north"}]}'}})

    llm = OllamaClient()
    llm.http = httpx.Client(transport=httpx.MockTransport(handler))
    seen = []
    with progress.sink(seen.append):
        assert llm.structured("s", "u", EditOps).ops[0].op == "set_side"
    assert [e["kind"] for e in seen] == ["thinking"] and seen[0]["message"] == "Let me think."
    assert llm.structured("s", "u", EditOps).ops[0].op == "set_side"  # no sink: plain call, think off
