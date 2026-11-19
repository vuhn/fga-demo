"""AgentService: each RPC delegates to a `<domain>_handler.py` built by `wire.py` with its own `<Domain>Usecases` (like the
Go API's handlers); this class only routes."""

from __future__ import annotations

from fga.demo.agent.v1 import agent_pb2_grpc as rpc
from .blueprint_handler import BlueprintHandler
from .estimate_handler import EstimateHandler


class AgentServicer(rpc.AgentServiceServicer):
    def __init__(self, blueprint: BlueprintHandler, estimate: EstimateHandler) -> None:
        self._blueprint, self._estimate = blueprint, estimate

    def BriefFromText(self, request, context):
        return self._blueprint.BriefFromText(request, context)

    def Revise(self, request, context):
        return self._blueprint.Revise(request, context)

    def RevisePlan(self, request, context):
        return self._blueprint.RevisePlan(request, context)

    def Review(self, request, context):
        return self._blueprint.Review(request, context)

    def EstimateTurn(self, request, context):
        return self._estimate.EstimateTurn(request, context)
