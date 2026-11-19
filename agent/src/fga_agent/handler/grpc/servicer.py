"""AgentService. T10 is the scaffold: the RPCs answer UNIMPLEMENTED until their tasks (T11a, T11b, T13b) land."""

from __future__ import annotations

from ...gen.fga.demo.agent.v1 import agent_pb2_grpc as rpc
from ...wire import Usecases


class AgentServicer(rpc.AgentServiceServicer):
    def __init__(self, usecases: Usecases):
        self._uc = usecases
