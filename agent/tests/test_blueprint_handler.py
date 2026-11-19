"""BriefFromText and Revise over real gRPC: an Agent server on a scripted language model and the in-process fake Engine."""

import time

import grpc
import pytest
from fake_engine import Running
from fake_language_model import FakeLanguageModel
from scripted_engine import BRIEF

from fga_agent.domain.errors import LlmUnavailable
from fga_agent.gen.fga.demo.agent.v1 import agent_pb2 as a
from fga_agent.gen.fga.demo.agent.v1 import agent_pb2_grpc as rpc
from fga_agent.gen.fga.demo.common.v1 import common_pb2 as c
from fga_agent.handler.grpc.blueprint_handler import BlueprintHandler, BlueprintUsecases
from fga_agent.handler.grpc.common.error_helper import DETAIL_KEY
from fga_agent.handler.grpc.server import build_server
from fga_agent.handler.grpc.servicer import AgentServicer
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from fga_agent.service.brief_drafter import BriefDrafterService
from fga_agent.service.brief_reviser import BriefReviserService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.feedback_asker import FeedbackAskerService

DRAFT = b'{"title":"T","lot_width":20,"lot_depth":30,"rooms":[{"name":"Living","type":"living","target_area":20}]}'
OPS = {"ops": [{"op": "set_area", "room": "kitchen", "area": 12}]}


class Agent:
    def __init__(self, llm):
        self.eng = Running()
        engine = GrpcBlueprintEngine(open_channel(self.eng.addr), timeout=5)
        catalog, asker = DesignCatalogLoaderService(engine), FeedbackAskerService(llm)
        handler = BlueprintHandler(BlueprintUsecases(BriefDrafterService(engine, llm, catalog, asker),
                                                    BriefReviserService(engine, catalog, asker)))
        self.built = build_server(AgentServicer(handler), catalog.load)
        port = self.built.server.add_insecure_port("127.0.0.1:0")
        self.built.server.start()
        self.channel = grpc.insecure_channel(f"127.0.0.1:{port}")
        self.stub = rpc.AgentServiceStub(self.channel)

    def close(self):
        self.channel.close()
        self.built.stop(0)
        self.eng.stop()


@pytest.fixture
def agent_for():
    started = []

    def make(*answers):
        started.append(Agent(FakeLanguageModel(list(answers))))
        return started[-1]

    yield make
    for x in started:
        x.close()


def revise_request(message="make the kitchen bigger"):
    return a.ReviseRequest(brief_json=BRIEF, previous_model_json=b"{}", message=message, history=["earlier"])


def detail(err: grpc.RpcError) -> c.ErrorDetail:
    d = c.ErrorDetail()
    d.ParseFromString(dict(err.trailing_metadata())[DETAIL_KEY])
    return d


def kinds(events):
    return [e.WhichOneof("event") for e in events]


def test_brief_from_text_returns_the_engines_brief_and_notes(agent_for):
    ag = agent_for({"title": "T", "lot_width": 20, "lot_depth": 30, "rooms": [{"name": "Living", "type": "living", "target_area": 20}]})
    r = ag.stub.BriefFromText(a.BriefFromTextRequest(prompt="a small house"))
    assert b'"lot_width": 20' in r.brief_json and list(r.notes) == ["n"]
    assert ag.eng.engine.calls == ["ListTools", "GetCatalog", "NormalizeDraft"]


def test_an_empty_description_is_a_status_with_the_edit_error_detail(agent_for):
    ag = agent_for()
    with pytest.raises(grpc.RpcError) as e:
        ag.stub.BriefFromText(a.BriefFromTextRequest(prompt=" "))
    assert e.value.code() == grpc.StatusCode.INVALID_ARGUMENT and detail(e.value).code == c.ERROR_CODE_EDIT_ERROR


def test_an_unreachable_model_is_unavailable_with_the_llm_as_the_dependency(agent_for):
    ag = agent_for(LlmUnavailable("ollama down"))
    with pytest.raises(grpc.RpcError) as e:
        ag.stub.BriefFromText(a.BriefFromTextRequest(prompt="a house"))
    assert e.value.code() == grpc.StatusCode.UNAVAILABLE and detail(e.value).dependency == "llm"


def test_revise_streams_progress_then_one_result(agent_for):
    ag = agent_for(OPS)
    events = list(ag.stub.Revise(revise_request()))
    assert kinds(events) == ["progress"] * 4 + ["result"]
    assert [e.progress.stage for e in events[:-1]] == ["ask_model", "ops", "layout", "checks"]
    out = events[-1].result
    assert out.ok and out.attempts == 1 and list(out.history) == ["set area room=kitchen, area=12"]
    assert out.notes[0].startswith("applied: ")
    assert out.brief_json  # the Engine's normalized brief


def test_an_empty_message_is_a_plain_status_before_any_event(agent_for):
    ag = agent_for(OPS)
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.Revise(revise_request("  ")))
    assert e.value.code() == grpc.StatusCode.INVALID_ARGUMENT and detail(e.value).code == c.ERROR_CODE_EDIT_ERROR


def test_a_down_engine_before_the_stream_opens_is_a_plain_unavailable_status(agent_for):
    ag = agent_for(OPS)
    ag.eng.engine.down = True
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.Revise(revise_request()))
    assert e.value.code() == grpc.StatusCode.UNAVAILABLE and detail(e.value).dependency == "engine"


def test_operations_the_engine_refuses_three_times_end_the_stream_with_an_in_band_422(agent_for):
    ag = agent_for(OPS, OPS, OPS)
    ag.eng.engine.refuse, ag.eng.engine.refuse_only = (c.ERROR_CODE_EDIT_ERROR, "unknown room"), "ApplyBriefOps"
    events = list(ag.stub.Revise(revise_request()))
    assert kinds(events) == ["progress", "progress", "progress", "progress", "progress", "error"]
    assert [e.progress.stage for e in events[:-1]] == ["ask_model", "retry", "ask_model", "retry", "ask_model"]
    err = events[-1].error
    assert (err.status, err.error) == (422, "invalid_edit") and "after 3 attempts" in err.message


def test_a_model_that_dies_mid_stream_is_an_in_band_502(agent_for):
    ag = agent_for(LlmUnavailable("ollama died"))
    events = list(ag.stub.Revise(revise_request()))
    assert kinds(events) == ["progress", "error"]
    assert (events[-1].error.status, events[-1].error.error) == (502, "llm_unavailable")


def test_an_unexpected_failure_is_an_in_band_internal_error_that_leaks_nothing(agent_for):
    ag = agent_for(RuntimeError("secret path /etc/x"))
    events = list(ag.stub.Revise(revise_request()))
    assert (events[-1].error.status, events[-1].error.message) == (500, "internal error")


def test_a_client_that_leaves_stops_the_run_at_its_next_step(agent_for):
    ag = agent_for(OPS, OPS)
    ag.eng.engine.refuse, ag.eng.engine.refuse_only = (c.ERROR_CODE_EDIT_ERROR, "bad"), "ApplyBriefOps"
    call = ag.stub.Revise(revise_request())
    assert next(call).progress.stage == "ask_model"
    call.cancel()
    with pytest.raises(grpc.RpcError) as e:
        list(call)
    assert e.value.code() == grpc.StatusCode.CANCELLED
    time.sleep(0.3)
    assert ag.eng.engine.calls.count("ApplyBriefOps") <= 1  # the retry step raised Cancelled before a second ask
