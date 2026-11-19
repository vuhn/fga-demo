"""gRPC adapter of gateway.BlueprintEngine. Each method is exactly one Engine RPC, declared with @rpc so a test can
check that every `EngineService` method is either used here or deliberately left out (`NOT_USED`, `LATER`)."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

import grpc

from ...domain.model.brief import BriefBuild, BriefDraft
from ...domain.model.call_context import CallContext
from ...domain.model.design_limits import DesignLimits
from ...domain.model.operation_catalog import OperationCatalog
from ...domain.model.plan import PlanRevision
from ...domain.model.plan_check import PlanCheckReport
from ...gen.fga.demo.engine.v1 import engine_pb2_grpc as stubs
from .mapper import brief_mapper, design_limits_mapper, error_mapper, operation_catalog_mapper, plan_check_mapper, plan_mapper

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
            raise error_mapper.to_domain(name, err) from err

    @rpc("ListTools")
    def operations(self, ctx: CallContext) -> OperationCatalog:
        return operation_catalog_mapper.to_domain(self._call(ctx, "ListTools", operation_catalog_mapper.request()))

    @rpc("GetCatalog")
    def design_limits(self, ctx: CallContext) -> DesignLimits:
        return design_limits_mapper.to_domain(self._call(ctx, "GetCatalog", design_limits_mapper.request()))

    @rpc("NormalizeDraft")
    def normalize_draft(self, ctx: CallContext, draft: bytes) -> BriefDraft:
        return brief_mapper.draft_to_domain(self._call(ctx, "NormalizeDraft", brief_mapper.normalize_request(draft)))

    @rpc("Generate")
    def generate(self, ctx: CallContext, brief: bytes, previous_model: bytes = b"", repair: bool = False) -> BriefBuild:
        return brief_mapper.build_to_domain(
            self._call(ctx, "Generate", brief_mapper.generate_request(brief, previous_model, repair)))

    @rpc("ApplyBriefOps")
    def apply_brief_ops(self, ctx: CallContext, brief: bytes, ops: bytes, previous_model: bytes = b"") -> BriefBuild:
        return brief_mapper.build_to_domain(
            self._call(ctx, "ApplyBriefOps", brief_mapper.apply_ops_request(brief, ops, previous_model)))

    @rpc("ApplyPlanOps")
    def apply_plan_ops(self, ctx: CallContext, model: bytes, ops: bytes) -> PlanRevision:
        return plan_mapper.to_domain(self._call(ctx, "ApplyPlanOps", plan_mapper.apply_ops_request(model, ops)))

    @rpc("RunChecks")
    def run_checks(self, ctx: CallContext, model: bytes, brief: bytes = b"") -> PlanCheckReport:
        return plan_check_mapper.report_to_domain(
            self._call(ctx, "RunChecks", plan_check_mapper.run_checks_request(model, brief)))
