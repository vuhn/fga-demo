"""One in-process Engine gRPC server for the whole test run; tests talk to it through `stub`."""

import grpc
import pytest

from fga.demo.engine.v1 import engine_pb2_grpc as rpc
from fga_engine.handler.grpc.server import build_server
from fga_engine.wire import build_servicer


@pytest.fixture(scope="session")
def channel():
    server = build_server(build_servicer())
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    with grpc.insecure_channel(f"127.0.0.1:{port}") as ch:
        yield ch
    server.stop(0)


@pytest.fixture(scope="session")
def stub(channel):
    return rpc.EngineServiceStub(channel)
