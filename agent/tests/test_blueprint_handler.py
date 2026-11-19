"""BriefFromText and Revise over real gRPC: an Agent server on a scripted language model and the in-process fake Engine."""

import time

import grpc
import pytest
from fake_engine import Running
from fake_language_model import FakeLanguageModel
from scripted_engine import BRIEF

from fga_agent.domain.errors import LlmUnavailable
from fga.demo.agent.v1 import agent_pb2 as a
from fga.demo.agent.v1 import agent_pb2_grpc as rpc
from fga.demo.common.v1 import common_pb2 as c
from fga_agent.handler.grpc.blueprint_handler import BlueprintHandler, BlueprintUsecases
from fga_agent.handler.grpc.common.error_helper import DETAIL_KEY
from fga_agent.handler.grpc.server import build_server
from fga_agent.handler.grpc.servicer import AgentServicer
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from fga_agent.service.brief_drafter import BriefDrafterService
from fga_agent.service.brief_reviewer import BriefReviewerService
from fga_agent.service.brief_reviser import BriefReviserService
from fga_agent.service.plan_reviser import PlanReviserService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.feedback_asker import FeedbackAskerService

DRAFT = b'{"title":"T","lot_width":20,"lot_depth":30,"rooms":[{"name":"Living","type":"living","target_area":20}]}'
OPS = {"ops": [{"op": "set_area", "room": "kitchen", "area": 12}]}


class Agent:
    def __init__(self, llm):
        self.eng = Running()
        engine = GrpcBlueprintEngine(open_channel(self.eng.addr), timeout=5)
        catalog, asker = DesignCatalogLoaderService(engine), FeedbackAskerService(llm)
        reviewer = BriefReviewerService(engine, catalog, asker)
        handler = BlueprintHandler(BlueprintUsecases(BriefDrafterService(engine, llm, catalog, asker),
                                                    BriefReviserService(engine, catalog, asker, reviewer),
                                                    PlanReviserService(engine, catalog, asker), reviewer))
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


# --- RevisePlan and Review --------------------------------------------------------------------------------------

PLAN_OPS = {"ops": [{"op": "move_fixture", "id": "f1", "x": 1, "y": 2}]}


def revise_plan_request(message="move the sink"):
    return a.RevisePlanRequest(model_json=b'{"rooms":[],"fixtures":[],"openings":[]}', message=message, history=["earlier"])


def test_revise_plan_streams_progress_then_one_result_without_a_brief(agent_for):
    ag = agent_for(PLAN_OPS)
    events = list(ag.stub.RevisePlan(revise_plan_request()))
    assert kinds(events) == ["progress"] * 3 + ["result"]
    assert [e.progress.stage for e in events[:-1]] == ["ask_model", "ops", "checks"]
    out = events[-1].result
    assert out.ok and not out.brief_json and out.model_json and list(out.history) == ["moved"]
    assert out.notes[0] == "applied: moved" and "ApplyPlanOps" in ag.eng.engine.calls


def test_revise_plan_with_an_empty_message_is_a_plain_status_before_any_event(agent_for):
    ag = agent_for(PLAN_OPS)
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.RevisePlan(revise_plan_request(" ")))
    assert e.value.code() == grpc.StatusCode.INVALID_ARGUMENT and detail(e.value).code == c.ERROR_CODE_EDIT_ERROR


def test_revise_plan_operations_the_engine_refuses_three_times_end_in_an_in_band_422(agent_for):
    ag = agent_for(PLAN_OPS, PLAN_OPS, PLAN_OPS)
    ag.eng.engine.refuse, ag.eng.engine.refuse_only = (c.ERROR_CODE_EDIT_ERROR, "no room called x"), "ApplyPlanOps"
    events = list(ag.stub.RevisePlan(revise_plan_request()))
    assert (events[-1].error.status, events[-1].error.error) == (422, "invalid_edit")
    assert "no room called x" in events[-1].error.message


def review_request(max_rounds=0):
    return a.ReviewRequest(brief_json=BRIEF, previous_model_json=b"{}", history=["earlier"], max_rounds=max_rounds)


def test_review_streams_each_round_then_one_result(agent_for):
    ag = agent_for({"ops": [{"op": "set_side", "room": "kitchen", "side": None}]})
    events = list(ag.stub.Review(review_request()))
    assert kinds(events)[-1] == "result" and {e.progress.stage for e in events[:-1]} == {"review", "ask_model"}
    out = events[-1].result
    assert out.brief_json and out.attempts == 1 and out.history[0].startswith("round 1: set side room=kitchen")
    assert ag.eng.engine.calls == ["ListTools", "GetCatalog", "Generate", "ApplyBriefOps"]


def test_review_asks_the_model_nothing_when_the_plan_has_no_issues(agent_for):
    ag = agent_for()
    ag.eng.engine.clean = True
    events = list(ag.stub.Review(review_request(2)))
    assert kinds(events) == ["result"] and not events[0].result.history


def test_review_with_the_model_down_is_an_in_band_502(agent_for):
    ag = agent_for(LlmUnavailable("ollama died"))
    events = list(ag.stub.Review(review_request()))
    assert (events[-1].error.status, events[-1].error.error) == (502, "llm_unavailable")


def test_review_with_the_engine_down_before_the_stream_opens_is_a_plain_unavailable_status(agent_for):
    ag = agent_for()
    ag.eng.engine.down = True
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.Review(review_request()))
    assert e.value.code() == grpc.StatusCode.UNAVAILABLE and detail(e.value).dependency == "engine"


def test_a_client_that_leaves_stops_a_review_at_its_next_step(agent_for):
    ag = agent_for({"ops": [{"op": "set_side", "room": "kitchen", "side": None}]})
    ag.eng.engine.refuse, ag.eng.engine.refuse_only = (c.ERROR_CODE_EDIT_ERROR, "bad"), "ApplyBriefOps"
    call = ag.stub.Review(review_request())
    assert next(call).progress.stage == "review"
    call.cancel()
    with pytest.raises(grpc.RpcError) as e:
        list(call)
    assert e.value.code() == grpc.StatusCode.CANCELLED
