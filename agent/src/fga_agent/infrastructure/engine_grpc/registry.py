"""The Agent's typed tool registry over the Engine's gRPC API.

One entry = a tool name, ONE Engine RPC, the request built from the call's arguments and the response mapped to a
domain type. Adding a tool needs a method in `engine.proto` and one entry here (the registry test fails until the
method is either registered or deliberately listed in `NOT_TOOLS` / `LATER`)."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import Any

from ...gen.fga.demo.engine.v1 import engine_pb2 as e
from . import mapping as m


@dataclass(frozen=True)
class Entry:
    name: str
    rpc: str  # the EngineService method
    request: Callable[[Mapping[str, Any]], Any]
    response: Callable[[Any], Any]


def _b(args: Mapping[str, Any], key: str) -> bytes:
    v = args.get(key) or b""
    return v.encode() if isinstance(v, str) else v


ENTRIES: tuple[Entry, ...] = (
    Entry("normalize_draft", "NormalizeDraft",
          lambda a: e.NormalizeDraftRequest(draft_json=_b(a, "draft")), m.draft_normalized_from_pb),
    Entry("generate", "Generate",
          lambda a: e.GenerateRequest(brief_json=_b(a, "brief"), previous_model_json=_b(a, "previous_model"),
                                      repair=bool(a.get("repair"))), m.brief_result_from_pb),
    Entry("apply_brief_ops", "ApplyBriefOps",
          lambda a: e.ApplyBriefOpsRequest(brief_json=_b(a, "brief"), ops_json=_b(a, "ops"),
                                           previous_model_json=_b(a, "previous_model")), m.brief_result_from_pb),
    Entry("apply_plan_ops", "ApplyPlanOps",
          lambda a: e.ApplyPlanOpsRequest(model_json=_b(a, "model"), ops_json=_b(a, "ops")), m.plan_step_from_pb),
    Entry("run_checks", "RunChecks",
          lambda a: e.RunChecksRequest(model_json=_b(a, "model"), brief_json=_b(a, "brief")), m.checks_result_from_pb),
)

# Engine RPCs the Agent reads through the catalog loader, not as tools.
CATALOG = frozenset({"ListTools", "GetCatalog"})
# Engine RPCs the Agent never calls: the API calls them (plans, files).
NOT_TOOLS = frozenset({"CreatePlan", "DrawPlan", "ConvertToDXF", "ImportDXF", "PatchDXF", "Render"})
# Tools the estimate port (T12) registers.
LATER = frozenset({"SummarizePlan", "Takeoff", "PriceEstimate"})
