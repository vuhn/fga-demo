"""In-process gRPC tests of the Engine's catalog, brief, plan and estimate RPCs."""

import json

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from briefs import STUDIO
from fga_engine.gen.fga.demo.common.v1 import common_pb2 as c
from fga_engine.gen.fga.demo.engine.v1 import engine_pb2 as e
from fga_engine.gen.fga.demo.engine.v1 import engine_pb2_grpc as rpc
from fga_engine.handler.grpc.errors import DETAIL_KEY
from fga_engine.handler.grpc.server import SERVICE, build_server
from fga_engine.wire import build_engine


@pytest.fixture(scope="module")
def channel():
    server = build_server(build_engine())
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    with grpc.insecure_channel(f"127.0.0.1:{port}") as ch:
        yield ch
    server.stop(0)


@pytest.fixture(scope="module")
def stub(channel):
    return rpc.EngineServiceStub(channel)


def brief_json() -> bytes:
    return STUDIO.model_dump_json().encode()


def refusal(call) -> tuple[grpc.StatusCode, c.ErrorDetail]:
    with pytest.raises(grpc.RpcError) as ei:
        call()
    detail = c.ErrorDetail()
    for k, v in ei.value.trailing_metadata():
        if k == DETAIL_KEY:
            detail.ParseFromString(v)
    return ei.value.code(), detail


def test_health(channel):
    h = health_pb2_grpc.HealthStub(channel)
    assert h.Check(health_pb2.HealthCheckRequest(service="")).status == health_pb2.HealthCheckResponse.SERVING
    assert h.Check(health_pb2.HealthCheckRequest(service=SERVICE)).status == health_pb2.HealthCheckResponse.SERVING


def test_list_tools_and_catalog(stub):
    tl = stub.ListTools(e.ListToolsRequest())
    assert {t.kind for t in tl.tools} == {"brief_op", "plan_op", "draft"}
    assert json.loads(tl.ops_json_schema) and json.loads(tl.draft_json_schema)
    cat = stub.GetCatalog(e.GetCatalogRequest())
    assert cat.min_area["bedroom"] > 0 and cat.max_nested == 2 and "toilet" in cat.priced_fixtures
    assert any(r.code == "fixture_toilet" for r in cat.rate_catalog)


def test_normalize_draft_and_error_detail(stub):
    draft = {"title": "t", "lot_width": 20, "lot_depth": 30, "entrance_side": "south",
             "rooms": [{"name": "Hall", "type": "entry", "target_area": 5},
                       {"name": "Bedroom", "type": "bedroom", "target_area": 2}]}
    r = stub.NormalizeDraft(e.NormalizeDraftRequest(draft_json=json.dumps(draft).encode()))
    assert [x["name"] for x in json.loads(r.brief_json)["rooms"]] == ["Bedroom"] and r.notes
    draft["rooms"] = draft["rooms"][:1]
    status, d = refusal(lambda: stub.NormalizeDraft(e.NormalizeDraftRequest(draft_json=json.dumps(draft).encode())))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_EDIT_ERROR


def test_generate_and_apply_brief_ops(stub):
    g = stub.Generate(e.GenerateRequest(brief_json=brief_json(), repair=True))
    assert g.ok and g.model_json and g.total_target_area > 0 and g.checks_line
    assert len(g.room_targets) == len(STUDIO.rooms)
    room = STUDIO.rooms[0]
    ops = {"ops": [{"op": "resize_room", "room": room.name, "target_area": room.target_area + 1}]}
    a = stub.ApplyBriefOps(e.ApplyBriefOpsRequest(brief_json=brief_json(), ops_json=json.dumps(ops).encode(),
                                                  previous_model_json=g.model_json))
    assert a.changed and a.issue_score >= 0


def test_edit_error_and_unsupported_codes(stub):
    bad = json.dumps({"ops": [{"op": "remove_room", "room": "nope"}]}).encode()
    status, d = refusal(lambda: stub.ApplyBriefOps(e.ApplyBriefOpsRequest(brief_json=brief_json(), ops_json=bad)))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_EDIT_ERROR and "nope" in d.message
    uns = json.dumps({"ops": [{"op": "unsupported", "reason": "x"}]}).encode()
    status, d = refusal(lambda: stub.ApplyBriefOps(e.ApplyBriefOpsRequest(brief_json=brief_json(), ops_json=uns)))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_UNSUPPORTED


def test_malformed_json_is_invalid_argument(stub):
    status, d = refusal(lambda: stub.Generate(e.GenerateRequest(brief_json=b"{not json")))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_EDIT_ERROR
    status, _ = refusal(lambda: stub.Generate(e.GenerateRequest()))
    assert status == grpc.StatusCode.INVALID_ARGUMENT


def test_create_plan_then_plan_ops_checks_summary_takeoff_price(stub):
    p = stub.CreatePlan(e.CreatePlanRequest(brief_json=brief_json()))
    assert p.ok and p.model_json and p.dxf
    model = json.loads(p.model_json)
    fx = model["fixtures"][0]
    ops = {"ops": [{"op": "delete_fixture", "fixture": fx["id"]}]}
    step = stub.ApplyPlanOps(e.ApplyPlanOpsRequest(model_json=p.model_json, ops_json=json.dumps(ops).encode()))
    assert step.changed and step.summary and len(step.descriptions) == 1
    rc = stub.RunChecks(e.RunChecksRequest(model_json=p.model_json))
    assert len(rc.checks) > 0 and rc.issue_score >= 0

    facts = c.EstimateFacts(facts=[c.Fact(name="quality", text="normal", source=c.FACT_SOURCE_USER),
                                   c.Fact(name="floor_height", number=3.0, source=c.FACT_SOURCE_USER)])
    s = stub.SummarizePlan(e.SummarizePlanRequest(model_json=p.model_json, facts=facts))
    assert len(s.rooms) == len(model["rooms"]) and len(s.fixtures) == len(model["fixtures"])
    tk = stub.Takeoff(e.TakeoffRequest(model_json=p.model_json, facts=facts))
    assert tk.status == "ok" and tk.lines
    rates = [c.RateResult(code=ln.code, low=1.0, base=2.0, high=3.0, unit=ln.unit, confidence="ok") for ln in tk.lines]
    pe = stub.PriceEstimate(e.PriceEstimateRequest(takeoff=tk, rates=rates, facts=facts))
    assert pe.total_base > 0 and json.loads(pe.estimate_json)["lines"]


def test_draw_plan_refuses_an_imported_plan(stub):
    p = stub.CreatePlan(e.CreatePlanRequest(brief_json=brief_json()))
    status, d = refusal(lambda: stub.DrawPlan(e.DrawPlanRequest(model_json=p.model_json, title="x")))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_EDIT_ERROR


def test_message_larger_than_default_is_accepted(stub):
    big = json.dumps({"title": "x" * (5 << 20)}).encode()  # > the 4 MB default; refused for content, not size
    status, _ = refusal(lambda: stub.Generate(e.GenerateRequest(brief_json=big)))
    assert status == grpc.StatusCode.INVALID_ARGUMENT
