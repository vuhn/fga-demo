"""AgentService: each RPC delegates to a `<domain>_handler.py` built by `wire.py` with its own `<Domain>Usecases` (like the
Go API's handlers); this class only routes. EstimateTurn (T13b) answers UNIMPLEMENTED until its task lands."""

from __future__ import annotations

from fga.demo.agent.v1 import agent_pb2_grpc as rpc
from .blueprint_handler import BlueprintHandler


class AgentServicer(rpc.AgentServiceServicer):
    def __init__(self, blueprint: BlueprintHandler) -> None:
        self._blueprint = blueprint

    def BriefFromText(self, request, context):
        return self._blueprint.BriefFromText(request, context)

    def Revise(self, request, context):
        return self._blueprint.Revise(request, context)

    def RevisePlan(self, request, context):
        return self._blueprint.RevisePlan(request, context)

    def Review(self, request, context):
        return self._blueprint.Review(request, context)
