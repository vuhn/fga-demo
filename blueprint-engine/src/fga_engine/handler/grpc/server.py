"""The Engine's gRPC server: EngineService plus grpc.health.v1. Messages up to 32 MB."""

from __future__ import annotations

from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from fga.demo.engine.v1 import engine_pb2_grpc as rpc
from .servicer import EngineServicer

MAX_MESSAGE = 32 << 20
SERVICE = "fga.demo.engine.v1.EngineService"


def build_server(servicer: EngineServicer, workers: int = 8) -> grpc.Server:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=workers), options=[
        ("grpc.max_receive_message_length", MAX_MESSAGE), ("grpc.max_send_message_length", MAX_MESSAGE)])
    rpc.add_EngineServiceServicer_to_server(servicer, server)
    hs = health.HealthServicer()
    hs.set("", health_pb2.HealthCheckResponse.SERVING)
    hs.set(SERVICE, health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(hs, server)
    return server
