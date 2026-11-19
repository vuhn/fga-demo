"""Health follows the catalog; the streaming helper stops a run when the client leaves."""

import threading
import time

import grpc
import pytest
from fake_engine import Running
from grpc_health.v1 import health_pb2, health_pb2_grpc

from fga_agent.domain.errors import Cancelled
from fga_agent.handler.grpc.servicer import AgentServicer
from fga_agent.handler.grpc.server import SERVICE, build_server
from fga_agent.domain.model.progress import Progress
from fga_agent.handler.grpc.common.streaming_helper import CancelToken, stream_run
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService


def _status(ch, service=""):
    return health_pb2_grpc.HealthStub(ch).Check(health_pb2.HealthCheckRequest(service=service)).status


def _until(pred, timeout=5.0):
    end = time.time() + timeout
    while time.time() < end:
        if pred():
            return True
        time.sleep(0.02)
    return False


def test_health_is_not_serving_until_the_catalog_loads():
    eng = Running()
    eng.engine.down = True
    engine = GrpcBlueprintEngine(open_channel(eng.addr), timeout=5)
    built = build_server(AgentServicer(None), DesignCatalogLoaderService(engine).load, retry_s=0.05)
    port = built.server.add_insecure_port("127.0.0.1:0")
    built.server.start()
    built.watch_catalog()
    try:
        with grpc.insecure_channel(f"127.0.0.1:{port}") as ch:
            assert _status(ch) == health_pb2.HealthCheckResponse.NOT_SERVING
            assert _status(ch, SERVICE) == health_pb2.HealthCheckResponse.NOT_SERVING
            time.sleep(0.2)
            assert _status(ch) == health_pb2.HealthCheckResponse.NOT_SERVING  # still down
            eng.engine.down = False
            assert _until(lambda: _status(ch) == health_pb2.HealthCheckResponse.SERVING)
            assert _status(ch, SERVICE) == health_pb2.HealthCheckResponse.SERVING
    finally:
        built.stop(0)
        eng.stop()


class Ctx:
    """Just what stream_run needs of a ServicerContext."""

    def __init__(self):
        self.callbacks = []
        self.left = False

    def add_callback(self, cb):
        self.callbacks.append(cb)
        return True

    def leave(self):
        self.left = True
        for cb in self.callbacks:
            cb()


def test_stream_run_yields_progress_then_the_result():
    def run(on_progress):
        on_progress(Progress("step", "layout", "laying out"))
        on_progress(Progress("thinking", "thinking", "hmm"))
        return "RESULT"

    out = list(stream_run(Ctx(), run, lambda p: (p.kind, p.message), lambda e: ("error", str(e))))
    assert out == [("step", "laying out"), ("thinking", "hmm"), "RESULT"]


def test_an_error_after_the_stream_opened_is_an_in_band_event():
    def run(on_progress):
        on_progress(Progress("step", "a", "first"))
        raise RuntimeError("boom")

    out = list(stream_run(Ctx(), run, lambda p: p.stage, lambda e: f"error:{e}"))
    assert out == ["a", "error:boom"]


def test_a_client_that_leaves_cancels_the_run_at_its_next_report():
    ctx = Ctx()
    started, finished = threading.Event(), threading.Event()
    reached_after_cancel = []

    def run(on_progress):
        on_progress(Progress("step", "one", "x"))
        started.set()
        assert _until(lambda: ctx.left)
        try:
            on_progress(Progress("step", "two", "y"))  # the callback raises Cancelled here
            reached_after_cancel.append(True)
        finally:
            finished.set()

    gen = stream_run(ctx, run, lambda p: p.stage, lambda e: "error")
    assert next(gen) == "one"
    assert started.wait(2)
    ctx.leave()
    assert list(gen) == []  # no result, no error event
    assert finished.wait(2) and not reached_after_cancel


def test_cancel_token():
    t = CancelToken()
    t.check()
    t.cancel()
    with pytest.raises(Cancelled):
        t.check()
