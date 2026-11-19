"""AgentService. T10 is the scaffold: the RPCs answer UNIMPLEMENTED until their tasks (T11a, T11b, T13b) land. Each
RPC then delegates to a `<domain>_handler.py` built by `wire.py` with its own `<Domain>Usecases` (like the Go API's
handlers); this class only routes."""

from __future__ import annotations

from ...gen.fga.demo.agent.v1 import agent_pb2_grpc as rpc


class AgentServicer(rpc.AgentServiceServicer):
    pass
