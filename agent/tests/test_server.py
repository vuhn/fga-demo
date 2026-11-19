"""Health follows the catalog; the streaming helper stops a run when the client leaves."""

import threading
import time

import grpc
import pytest
from fake_engine import Running
from grpc_health.v1 import health_pb2, health_pb2_grpc

from fga_agent.domain import progress
from fga_agent.domain.cancel import CancelToken
from fga_agent.handler.grpc.server import SERVICE, build_server
from fga_agent.handler.grpc.streaming import stream_run
from fga_agent.infrastructure.engine_grpc.catalog import CatalogLoader
from fga_agent.infrastructure.engine_grpc.client import EngineTools, open_channel
from fga_agent.wire import Agent


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
    tools = EngineTools(open_channel(eng.addr), timeout=5)
    built = build_server(Agent(llm=None, plan_tools=tools, registry=tools, catalog=CatalogLoader(tools)), retry_s=0.05)
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

    def add_callback(self, cb):
        self.callbacks.append(cb)
        return True

    def leave(self):
        for cb in self.callbacks:
            cb()


def test_stream_run_yields_progress_then_the_result():
    def run(token: CancelToken):
        progress.emit("layout", "laying out")
        progress.emit_thinking("hmm")
        return "RESULT"

    out = list(stream_run(Ctx(), run, lambda ev: (ev["kind"], ev["message"]), lambda e: ("error", str(e))))
    assert out == [("step", "laying out"), ("thinking", "hmm"), "RESULT"]


def test_an_error_after_the_stream_opened_is_an_in_band_event():
    def run(token):
        progress.emit("a", "first")
        raise RuntimeError("boom")

    out = list(stream_run(Ctx(), run, lambda ev: ev["stage"], lambda e: f"error:{e}"))
    assert out == ["a", "error:boom"]


def test_a_client_that_leaves_cancels_the_run_at_its_next_step():
    ctx = Ctx()
    started, finished = threading.Event(), threading.Event()
    reached_after_cancel = []

    def run(token):
        progress.emit("one", "x")
        started.set()
        assert _until(lambda: token.cancelled)
        try:
            progress.emit("two", "y")  # the sink raises Cancelled here
            reached_after_cancel.append(True)
        finally:
            finished.set()

    gen = stream_run(ctx, run, lambda ev: ev["stage"], lambda e: "error")
    assert next(gen) == "one"
    assert started.wait(2)
    ctx.leave()
    assert list(gen) == []  # no result, no error event
    assert finished.wait(2) and not reached_after_cancel


def test_cancel_token():
    t = CancelToken()
    t.check()
    t.cancel()
    with pytest.raises(progress.Cancelled):
        t.check()
