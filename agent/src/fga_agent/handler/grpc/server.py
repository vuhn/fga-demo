"""The Agent's gRPC server: AgentService plus grpc.health.v1. Messages up to 32 MB. Health is NOT_SERVING until the
Engine's tool list and catalog have loaded (a background thread retries until they do)."""

from __future__ import annotations

import logging
import threading
from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from ...domain.errors import EngineUnavailable
from fga.demo.agent.v1 import agent_pb2_grpc as rpc
from collections.abc import Callable

from ...domain.model.call_context import CallContext
from .servicer import AgentServicer

MAX_MESSAGE = 32 << 20
MAX_BACKOFF = 15  # the retry pause grows to this many times the first one
SERVICE = "fga.demo.agent.v1.AgentService"
log = logging.getLogger("fga_agent.server")


class AgentServer:
    """The gRPC server and its health: start it, then call watch_catalog() once; stop() ends the loader thread too.
    `ready_check` is what must succeed before the Agent reports SERVING (it raises EngineUnavailable until then)."""

    def __init__(self, server: grpc.Server, ready_check: Callable[[CallContext], None], hs: health.HealthServicer,
                 retry_s: float) -> None:
        self.server, self._ready_check, self._hs, self._retry = server, ready_check, hs, retry_s
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._load, daemon=True)

    def _set(self, status) -> None:
        self._hs.set("", status)
        self._hs.set(SERVICE, status)

    def _load(self) -> None:
        """Retries until the Engine answers. The pause doubles up to 15 times the first one (2 s becomes 30 s), and a
        failure is a warning only when it differs from the last one logged, so a down Engine is one line, not a flood."""
        pause, last = self._retry, ""
        while not self._stop.is_set():
            try:
                self._ready_check(CallContext(request_id="startup"))
                self._set(health_pb2.HealthCheckResponse.SERVING)
                log.info("engine catalog loaded; serving")
                return
            except EngineUnavailable as e:
                if str(e) != last:
                    log.warning("engine catalog not loaded yet, retrying (up to every %.0fs): %s", self._retry * MAX_BACKOFF, e)
                    last = str(e)
                else:
                    log.debug("engine catalog still not loaded: %s", e)
            self._stop.wait(pause)
            pause = min(pause * 2, self._retry * MAX_BACKOFF)

    def watch_catalog(self) -> None:
        self._thread.start()

    def stop(self, grace: float = 5) -> None:
        self._stop.set()
        self.server.stop(grace).wait()


def build_server(servicer: AgentServicer, ready_check: Callable[[CallContext], None], workers: int = 8,
                 retry_s: float = 2.0) -> AgentServer:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=workers), options=[
        ("grpc.max_receive_message_length", MAX_MESSAGE), ("grpc.max_send_message_length", MAX_MESSAGE)])
    rpc.add_AgentServiceServicer_to_server(servicer, server)
    hs = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(hs, server)
    built = AgentServer(server, ready_check, hs, retry_s)
    built._set(health_pb2.HealthCheckResponse.NOT_SERVING)
    return built
