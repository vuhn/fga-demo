"""An in-process EngineService built from the Agent's own generated stubs (the Agent never imports Engine code)."""

from __future__ import annotations

from concurrent import futures

import grpc

from fga.demo.common.v1 import common_pb2 as c
from fga.demo.engine.v1 import engine_pb2 as e
from fga.demo.engine.v1 import engine_pb2_grpc as rpc


class FakeEngine(rpc.EngineServiceServicer):
    def __init__(self) -> None:
        self.calls: list[str] = []
        self.down = False  # answer UNAVAILABLE while True
        self.refuse: tuple[int, str] | None = None  # (ErrorCode, message): INVALID_ARGUMENT with a detail
        self.refuse_only: str | None = None  # limit `refuse` to this one RPC
        self.clean = False  # Generate answers a plan with no failed check
        self.request_ids: list[str] = []

    def _enter(self, name: str, context) -> None:
        self.calls.append(name)
        self.request_ids += [v for k, v in context.invocation_metadata() if k == "x-request-id"]
        if self.down:
            context.abort(grpc.StatusCode.UNAVAILABLE, "engine is down")
        if self.refuse and self.refuse_only in (None, name):
            code, msg = self.refuse
            context.set_trailing_metadata((("error-detail-bin", c.ErrorDetail(code=code, message=msg).SerializeToString()),))
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, msg)

    def ListTools(self, request, context):
        self._enter("ListTools", context)
        return e.ListToolsResponse(tools=[e.Tool(name="set_area", kind="brief_op", description="d", json_schema="{}",
                                                 signature="set_area {room}")], ops_json_schema="{}",
                                   plan_ops_json_schema="{}", draft_json_schema="{}")

    def GetCatalog(self, request, context):
        self._enter("GetCatalog", context)
        return e.GetCatalogResponse(min_area={"bedroom": 9.0}, max_nested=2, fixture_kinds=["sink"],
                                    rate_catalog=[e.RateSpec(code="R1", package="p", description="d", unit="m2", quality_sensitive=True)])

    def NormalizeDraft(self, request, context):
        self._enter("NormalizeDraft", context)
        return e.NormalizeDraftResponse(brief_json=request.draft_json, notes=["n"])

    def Generate(self, request, context):
        self._enter("Generate", context)
        failed = [] if self.clean else [c.Check(rule="x", severity="error", ok=False, message="m", room_id="r1")]
        return e.BriefResult(brief_json=request.brief_json, model_json=b"{}", ok=True, attempts=1, issue_score=2,
                             room_targets=[e.RoomTarget(room_id="r1", target_area=12.5)], changed=request.repair,
                             checks=failed)

    def ApplyBriefOps(self, request, context):
        self._enter("ApplyBriefOps", context)
        return e.BriefResult(brief_json=request.ops_json, model_json=b"{}", ok=True, changed=True,
                             room_targets=[e.RoomTarget(room_id="r1", target_area=12.5)])

    def ApplyPlanOps(self, request, context):
        self._enter("ApplyPlanOps", context)
        return e.PlanStep(model_json=request.model_json, ok=True, summary="s", descriptions=["moved"], changed=True)

    def SummarizePlan(self, request, context):
        self._enter("SummarizePlan", context)
        self.facts_seen = request.facts
        return e.PlanSummary(rooms=[e.PlanSummary.Room(id="r1", name="Living", type="living", area_m2=22.5)],
                             fixtures=[e.PlanSummary.Fixture(id="f1", kind="toilet", room_id="r1")],
                             openings=[e.PlanSummary.Opening(id="d1", kind="door", room_ids=["r1", "outside"], width=0.9)],
                             pending_fixtures=["f1"], structure_in_scope=True)

    def Takeoff(self, request, context):
        self._enter("Takeoff", context)
        self.takeoff_seen = request
        return e.TakeoffResponse(status="ok", warnings=["w"], lines=[
            e.TakeoffLine(code="floor_tile", package="Finishes", description="floor tile", quantity=40.5, unit="m2", basis="net area",
                          confidence="measured", deps=["floor_height"], room_id="r1")])

    def PriceEstimate(self, request, context):
        self._enter("PriceEstimate", context)
        self.price_seen = request
        doc = {"currency": "VND", "total": {"low": 8.0, "base": 10.0, "high": 12.0}, "packages": {"Finishes": {"low": 8, "base": 10, "high": 12}},
               "assumptions": ["floor_height = 3.0 (assumed)"], "warnings": ["No price for paint"],
               "lines": [{"code": "floor_tile", "package": "Finishes", "description": "floor tile", "quantity": 40.5, "unit": "m2",
                          "basis": "net area", "confidence": "measured", "priced": True, "rate_base": 250000.0, "rate_confidence": "ok",
                          "low": 8, "base": 10, "high": 12}],
               "rates": [{"code": "floor_tile", "low": 200000.0, "base": 250000.0, "high": 300000.0, "unit": "m2", "currency": "VND",
                          "confidence": "ok", "region": "Hanoi", "quality": "good", "note": "n",
                          "sources": [{"url": "https://x.test", "quote": "q", "retrieved_at": "2026-11-16"}]}]}
        import json
        return e.PriceEstimateResponse(estimate_json=json.dumps(doc).encode(), total_low=8, total_base=10, total_high=12)

    def RunChecks(self, request, context):
        self._enter("RunChecks", context)
        return e.RunChecksResponse(ok=True, issue_score=0)


class Running:
    def __init__(self) -> None:
        self.engine = FakeEngine()
        self.server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
        rpc.add_EngineServiceServicer_to_server(self.engine, self.server)
        self.addr = f"127.0.0.1:{self.server.add_insecure_port('127.0.0.1:0')}"
        self.server.start()

    def stop(self) -> None:
        self.server.stop(0)
