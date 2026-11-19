"""The Agent's gRPC server: AgentService plus grpc.health.v1. Messages up to 32 MB. Health is NOT_SERVING until the
Engine's tool list and catalog have loaded (a background thread retries until they do)."""

from __future__ import annotations

import logging
import threading
from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from ...domain.errors import EngineUnavailable
from ...gen.fga.demo.agent.v1 import agent_pb2_grpc as rpc
from ...wire import Agent
from .servicer import AgentServicer

MAX_MESSAGE = 32 << 20
SERVICE = "fga.demo.agent.v1.AgentService"
log = logging.getLogger("fga_agent.server")


class Built:
    """A built server: start it, then call watch_catalog() once; stop() ends the loader thread too."""

    def __init__(self, server: grpc.Server, agent: Agent, hs: health.HealthServicer, retry_s: float) -> None:
        self.server, self._agent, self._hs, self._retry = server, agent, hs, retry_s
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._load, daemon=True)

    def _set(self, status) -> None:
        self._hs.set("", status)
        self._hs.set(SERVICE, status)

    def _load(self) -> None:
        while not self._stop.is_set():
            try:
                self._agent.catalog.tools()
                self._agent.catalog.catalog()
                self._set(health_pb2.HealthCheckResponse.SERVING)
                log.info("engine catalog loaded; serving")
                return
            except EngineUnavailable as e:
                log.warning("engine catalog not loaded yet: %s", e)
            self._stop.wait(self._retry)

    def watch_catalog(self) -> None:
        self._thread.start()

    def stop(self, grace: float = 5) -> None:
        self._stop.set()
        self.server.stop(grace).wait()


def build_server(agent: Agent, workers: int = 8, retry_s: float = 2.0) -> Built:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=workers), options=[
        ("grpc.max_receive_message_length", MAX_MESSAGE), ("grpc.max_send_message_length", MAX_MESSAGE)])
    rpc.add_AgentServiceServicer_to_server(AgentServicer(agent), server)
    hs = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(hs, server)
    built = Built(server, agent, hs, retry_s)
    built._set(health_pb2.HealthCheckResponse.NOT_SERVING)
    return built
