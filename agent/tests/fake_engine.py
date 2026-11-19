"""An in-process EngineService built from the Agent's own generated stubs (the Agent never imports Engine code)."""

from __future__ import annotations

from concurrent import futures

import grpc

from fga_agent.gen.fga.demo.common.v1 import common_pb2 as c
from fga_agent.gen.fga.demo.engine.v1 import engine_pb2 as e
from fga_agent.gen.fga.demo.engine.v1 import engine_pb2_grpc as rpc


class FakeEngine(rpc.EngineServiceServicer):
    def __init__(self) -> None:
        self.calls: list[str] = []
        self.down = False  # answer UNAVAILABLE while True
        self.refuse: tuple[int, str] | None = None  # (ErrorCode, message): INVALID_ARGUMENT with a detail
        self.request_ids: list[str] = []

    def _enter(self, name: str, context) -> None:
        self.calls.append(name)
        self.request_ids += [v for k, v in context.invocation_metadata() if k == "x-request-id"]
        if self.down:
            context.abort(grpc.StatusCode.UNAVAILABLE, "engine is down")
        if self.refuse:
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
                                    rate_catalog=[e.RateSpec(code="R1", package="p", description="d", unit="m2")])

    def NormalizeDraft(self, request, context):
        self._enter("NormalizeDraft", context)
        return e.NormalizeDraftResponse(brief_json=request.draft_json, notes=["n"])

    def Generate(self, request, context):
        self._enter("Generate", context)
        return e.BriefResult(brief_json=request.brief_json, model_json=b"{}", ok=True, attempts=1, issue_score=2,
                             room_targets=[e.RoomTarget(room_id="r1", target_area=12.5)], changed=request.repair,
                             checks=[c.Check(rule="x", severity="error", ok=False, message="m", room_id="r1")])

    def ApplyBriefOps(self, request, context):
        self._enter("ApplyBriefOps", context)
        return e.BriefResult(brief_json=request.ops_json, ok=True, changed=True)

    def ApplyPlanOps(self, request, context):
        self._enter("ApplyPlanOps", context)
        return e.PlanStep(model_json=request.model_json, ok=True, summary="s", descriptions=["moved"], changed=True)

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
