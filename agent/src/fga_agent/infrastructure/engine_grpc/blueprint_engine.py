"""gRPC adapter of gateway.BlueprintEngine. Each method is exactly one Engine RPC, declared with @rpc so a test can
check that every `EngineService` method is either used here or deliberately left out (`NOT_USED`, `LATER`)."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

import grpc

from ...domain.brief import BriefBuild, BriefDraft
from ...domain.call_context import CallContext
from ...domain.design_limits import DesignLimits
from ...domain.operation_catalog import OperationCatalog
from ...domain.plan import PlanRevision
from ...domain.plan_check import PlanCheckReport
from ...gen.fga.demo.engine.v1 import engine_pb2 as e
from ...gen.fga.demo.engine.v1 import engine_pb2_grpc as stubs
from . import mapping as m
from .errors import map_error

MAX_MESSAGE = 32 << 20
REQUEST_ID = "x-request-id"

# Engine RPCs the Agent never calls: the API calls them (plans, files).
NOT_USED = frozenset({"CreatePlan", "DrawPlan", "ConvertToDXF", "ImportDXF", "PatchDXF", "Render"})
# Engine RPCs the estimate port (T12) will use.
LATER = frozenset({"SummarizePlan", "Takeoff", "PriceEstimate"})


def open_channel(addr: str) -> grpc.Channel:
    return grpc.insecure_channel(addr, options=[("grpc.max_receive_message_length", MAX_MESSAGE),
                                                ("grpc.max_send_message_length", MAX_MESSAGE)])


def rpc(name: str) -> Callable:
    """Marks a method as the one Engine RPC `name`."""

    def mark(fn: Callable) -> Callable:
        fn.rpc = name  # type: ignore[attr-defined]
        return fn

    return mark


def used_rpcs() -> set[str]:
    return {getattr(f, "rpc") for f in vars(GrpcBlueprintEngine).values() if hasattr(f, "rpc")}


class GrpcBlueprintEngine:
    def __init__(self, channel: grpc.Channel, timeout: float = 120.0) -> None:
        self._stub = stubs.EngineServiceStub(channel)
        self._timeout = timeout

    def _call(self, ctx: CallContext, name: str, request: Any) -> Any:
        md = ((REQUEST_ID, ctx.request_id),) if ctx.request_id else None
        try:
            return getattr(self._stub, name)(request, timeout=self._timeout, metadata=md)
        except grpc.RpcError as err:
            raise map_error(name, err) from err

    @rpc("ListTools")
    def operations(self, ctx: CallContext) -> OperationCatalog:
        return m.operation_catalog(self._call(ctx, "ListTools", e.ListToolsRequest()))

    @rpc("GetCatalog")
    def design_limits(self, ctx: CallContext) -> DesignLimits:
        return m.design_limits(self._call(ctx, "GetCatalog", e.GetCatalogRequest()))

    @rpc("NormalizeDraft")
    def normalize_draft(self, ctx: CallContext, draft: bytes) -> BriefDraft:
        return m.brief_draft(self._call(ctx, "NormalizeDraft", e.NormalizeDraftRequest(draft_json=draft)))

    @rpc("Generate")
    def generate(self, ctx: CallContext, brief: bytes, previous_model: bytes = b"", repair: bool = False) -> BriefBuild:
        return m.brief_build(self._call(ctx, "Generate", e.GenerateRequest(
            brief_json=brief, previous_model_json=previous_model, repair=repair)))

    @rpc("ApplyBriefOps")
    def apply_brief_ops(self, ctx: CallContext, brief: bytes, ops: bytes, previous_model: bytes = b"") -> BriefBuild:
        return m.brief_build(self._call(ctx, "ApplyBriefOps", e.ApplyBriefOpsRequest(
            brief_json=brief, ops_json=ops, previous_model_json=previous_model)))

    @rpc("ApplyPlanOps")
    def apply_plan_ops(self, ctx: CallContext, model: bytes, ops: bytes) -> PlanRevision:
        return m.plan_revision(self._call(ctx, "ApplyPlanOps", e.ApplyPlanOpsRequest(model_json=model, ops_json=ops)))

    @rpc("RunChecks")
    def run_checks(self, ctx: CallContext, model: bytes, brief: bytes = b"") -> PlanCheckReport:
        return m.plan_check_report(self._call(ctx, "RunChecks", e.RunChecksRequest(model_json=model, brief_json=brief)))
