"""In-process gRPC tests of the Engine's catalog, brief, plan and estimate RPCs."""

import json

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from briefs import STUDIO
from fga.demo.common.v1 import common_pb2 as c
from fga.demo.engine.v1 import engine_pb2 as e
from fga.demo.engine.v1 import engine_pb2_grpc as rpc
from fga_engine.handler.grpc.common.error_helper import DETAIL_KEY
from fga_engine.handler.grpc.server import SERVICE, build_server
from fga_engine.wire import build_servicer


@pytest.fixture(scope="module")
def channel():
    server = build_server(build_servicer())
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


# --- file RPCs (T7b) ---------------------------------------------------------------------------------------------
import base64

from fastapi.testclient import TestClient

import sample_dxf
from fga_engine.app import create_app
from fga_engine.domain.schemas import BuildingModel


@pytest.fixture(scope="module")
def sheet_bytes() -> bytes:
    return sample_dxf.sheet()[0]


def _upload(data: bytes, name="p.dxf") -> c.Upload:
    return c.Upload(name=name, mime="image/vnd.dxf", data=data)


def test_import_dxf_matches_the_http_endpoint(stub, sheet_bytes):
    got = stub.ImportDXF(e.ImportDXFRequest(file=_upload(sheet_bytes)))
    http = TestClient(create_app(llm=object())).post("/v1/import/dxf", files={"file": ("s.dxf", sheet_bytes, "image/vnd.dxf")})
    assert http.status_code == 200
    assert json.loads(got.plans_json) == http.json()["plans"]  # identical plans, checks and svg
    assert json.loads(got.report_json) == http.json()["report"]
    assert len(json.loads(got.plans_json)) == 3


def test_import_dxf_refuses_junk_and_oversize(stub):
    code, detail = refusal(lambda: stub.ImportDXF(e.ImportDXFRequest(file=_upload(b"junk"))))
    assert code == grpc.StatusCode.INVALID_ARGUMENT and detail.code == c.ERROR_CODE_EDIT_ERROR
    code, _ = refusal(lambda: stub.ImportDXF(e.ImportDXFRequest(file=_upload(b"0" * ((20 << 20) + 1)))))
    assert code == grpc.StatusCode.INVALID_ARGUMENT


def test_a_20_mb_dxf_goes_through(stub, sheet_bytes):
    """DXF group 999 is a comment, so padding keeps the file valid and exactly at the limit."""
    pad = b"999\ncomment-padding-padding-padding-padding-padding\n"
    head, sep, tail = sheet_bytes.partition(b"0\nSECTION\n")
    n = ((20 << 20) - len(sheet_bytes)) // len(pad)
    big = head + pad * n + sep + tail
    assert (19 << 20) < len(big) <= (20 << 20)
    got = stub.ImportDXF(e.ImportDXFRequest(file=_upload(big)))
    assert len(json.loads(got.plans_json)) == 3


def test_patch_dxf_matches_the_http_endpoint(stub):
    from briefs import FEASIBLE
    from fga_engine.infrastructure.draw import build_dxf, to_dxf_bytes
    from fga_engine.infrastructure.dxf_plan import import_dxf
    from fga_engine.pipeline import generate

    gen = next(g for g in (generate(b).model for b in FEASIBLE.values())
               if g is not None and len(g.fixtures) >= 4 and any(o.kind == "window" for o in g.openings))
    data = to_dxf_bytes(build_dxf(gen)[0])
    base = import_dxf(data).plans[0].model
    cur = base.model_copy(deep=True)
    cur.rooms[0].name = "Den"
    got = stub.PatchDXF(e.PatchDXFRequest(original=_upload(data), base_model_json=base.model_dump_json().encode(),
                                          current_model_json=cur.model_dump_json().encode()))
    http = TestClient(create_app(llm=object())).post(
        "/v1/export/dxf-patch", files={"file": ("p.dxf", data, "image/vnd.dxf")},
        data={"base": base.model_dump_json(), "current": cur.model_dump_json()})
    assert http.status_code == 200
    # bytes differ per write (ezdxf stamps a GUID and times), so compare what the files mean
    assert import_dxf(got.dxf).plans[0].model == import_dxf(base64.b64decode(http.json()["dxf"])).plans[0].model
    assert import_dxf(got.dxf).plans[0].model.rooms[0].name == "Den"
    assert json.loads(got.report_json) == http.json()["report"]
    assert any("renamed to 'Den'" in s for s in json.loads(got.report_json)["applied"])
    code, _ = refusal(lambda: stub.PatchDXF(e.PatchDXFRequest(original=_upload(data), base_model_json=b"{}", current_model_json=b"{}")))
    assert code == grpc.StatusCode.INVALID_ARGUMENT


@pytest.mark.parametrize("fmt,media", [(e.RENDER_FORMAT_SVG, "image/svg+xml"), (e.RENDER_FORMAT_DXF, "image/vnd.dxf"),
                                       (e.RENDER_FORMAT_PDF, "application/pdf"), (e.RENDER_FORMAT_IFC, "application/x-step")])
def test_render_each_format_matches_http(stub, fmt, media):
    plan = stub.CreatePlan(e.CreatePlanRequest(brief_json=brief_json()))
    model = BuildingModel.model_validate_json(plan.model_json)
    names = {e.RENDER_FORMAT_SVG: "svg", e.RENDER_FORMAT_DXF: "dxf", e.RENDER_FORMAT_PDF: "pdf", e.RENDER_FORMAT_IFC: "ifc"}
    got = stub.Render(e.RenderRequest(model_json=plan.model_json, format=fmt,
                                      title_block=e.TitleBlock(title="Flat", version=2, issued="2026-01-01", author="ada")))
    assert got.media_type == media and got.data
    http = TestClient(create_app(llm=object())).post("/v1/render", json={
        "model": model.model_dump(mode="json"), "format": names[fmt],
        "title_block": {"project": "Flat", "version": 2, "issued": "2026-01-01", "author": "ada"}})
    assert http.status_code == 200 and http.headers["content-type"].startswith(media)
    if fmt == e.RENDER_FORMAT_SVG:
        assert got.data == http.content


def test_render_unspecified_format_is_refused(stub):
    code, detail = refusal(lambda: stub.Render(e.RenderRequest(model_json=b"{}")))
    assert code == grpc.StatusCode.INVALID_ARGUMENT and detail.code == c.ERROR_CODE_EDIT_ERROR


def test_convert_rejects_a_non_plan_file(stub):
    code, _ = refusal(lambda: stub.ConvertToDXF(e.ConvertToDXFRequest(
        file=c.Upload(name="x.png", mime="image/png", data=b"not a picture"))))
    assert code == grpc.StatusCode.INVALID_ARGUMENT
