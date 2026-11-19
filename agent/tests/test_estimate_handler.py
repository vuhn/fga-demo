"""EstimateTurn over real gRPC: an Agent server on the scripted Engine, researcher and model. Decode, one use case, encode; a refusal
before the first message is a status, after it an in-band `Error`; a client that leaves stops the turn."""

import json
import time

import grpc
import pytest
from estimate_engine import EstimateEngine, FakePrices
from estimate_fixtures import SessionState, TurnLanguageModel, full_state

from fga.demo.agent.v1 import agent_pb2 as a
from fga.demo.agent.v1 import agent_pb2_grpc as rpc
from fga.demo.common.v1 import common_pb2 as c
from fga_agent.handler.grpc.common.error_helper import DETAIL_KEY
from fga_agent.handler.grpc.estimate_handler import EstimateHandler, EstimateUsecases
from fga_agent.handler.grpc.mapper import session_state_mapper
from fga_agent.handler.grpc.server import build_server
from fga_agent.handler.grpc.servicer import AgentServicer
from fga_agent.service.estimate_interview import EstimateInterviewService
from fga_agent.service.estimate_pricer import EstimatePricerService
from fga_agent.service.estimate_reader import EstimateReaderService
from fga_agent.service.estimate_replier import EstimateReplierService
from fga_agent.service.estimate_turner import EstimateTurnerService
from fga_agent.service.helper.claim_checker import ClaimCheckerService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.keyword_reader import KeywordReaderService


class Agent:
    def __init__(self, llm, engine=None, prices=None, **turner):
        self.engine = engine or EstimateEngine(fixtures=[])
        self.prices, self.llm = prices or FakePrices(), llm
        catalog, keywords = DesignCatalogLoaderService(self.engine), KeywordReaderService()
        turner = EstimateTurnerService(
            self.engine, catalog, EstimateReaderService(llm, catalog, ClaimCheckerService(), keywords),
            EstimateInterviewService(catalog, keywords), EstimatePricerService(self.engine, catalog, self.prices),
            EstimateReplierService(llm), **turner)
        self.built = build_server(AgentServicer(None, EstimateHandler(EstimateUsecases(turner))), catalog.load)
        port = self.built.server.add_insecure_port("127.0.0.1:0")
        self.built.server.start()
        self.channel = grpc.insecure_channel(f"127.0.0.1:{port}")
        self.stub = rpc.AgentServiceStub(self.channel)

    def close(self):
        self.channel.close()
        self.built.stop(0)


@pytest.fixture
def agent_for():
    started = []

    def make(llm=None, **kw):
        started.append(Agent(llm or TurnLanguageModel(), **kw))
        return started[-1]

    yield make
    for x in started:
        x.close()


def request(state=None, message="", **kw):
    return a.EstimateTurnRequest(model_json=b"plan", state_json=session_state_mapper.to_json(state) if state else b"", message=message, **kw)


def kinds(events):
    return [e.WhichOneof("event") for e in events]


def detail(err: grpc.RpcError) -> c.ErrorDetail:
    d = c.ErrorDetail()
    d.ParseFromString(dict(err.trailing_metadata())[DETAIL_KEY])
    return d


def test_a_question_turn_streams_progress_fact_tokens_question_then_done(agent_for):
    ag = agent_for(TurnLanguageModel({"city": "Hanoi"}, chunks=("Which ", "kind?")))
    events = list(ag.stub.EstimateTurn(request(message="Hanoi")))
    names = kinds(events)
    assert names[0] == "progress" and names.index("fact") < names.index("token") < names.index("question") < names.index("done")
    assert "".join(e.token.text for e in events if e.HasField("token")) == "Which kind?"
    fact = next(e.fact for e in events if e.HasField("fact"))
    assert (fact.name, fact.value, fact.source) == ("city", "Hanoi", "user")
    question = next(e.question for e in events if e.HasField("question"))
    assert question.id == "kind" and list(question.options)[-1] == "I don't know, you decide" and question.allow_unknown
    done = events[-1].done
    state = json.loads(done.state_json)
    assert done.status == "interviewing" and done.reply == "Which kind?" and not done.estimate_json
    assert state["facts"]["city"] == {"value": "Hanoi", "source": "user"} and state["pending"][0] == "kind" and state["turns"] == 1


def test_an_estimate_turn_streams_rates_then_the_estimate_then_the_closing_reply(agent_for):
    ag = agent_for()
    events = list(ag.stub.EstimateTurn(request(full_state(), "")))
    names = kinds(events)
    assert names.count("rate") >= 5 and names.index("rate") < names.index("estimate_json") < names.index("token") < names.index("done")
    rate = next(e.rate for e in events if e.HasField("rate"))
    assert rate.confidence == "ok" and rate.HasField("base") and rate.base == 1000
    streamed = json.loads(next(e.estimate_json for e in events if e.WhichOneof("event") == "estimate_json"))
    done = events[-1].done
    assert done.status == "ready" and json.loads(done.estimate_json) == streamed and streamed["total"]["base"] > 0
    state = json.loads(done.state_json)
    assert state["status"] == "ready" and state["estimate"] == streamed and state["rates"] and state["fingerprint"]


def test_the_session_comes_back_and_an_unchanged_answer_is_not_priced_again(agent_for):
    ag = agent_for()
    first = list(ag.stub.EstimateTurn(request(full_state(), "")))[-1].done
    again = list(ag.stub.EstimateTurn(a.EstimateTurnRequest(model_json=b"plan", state_json=first.state_json, message="thanks")))
    assert "estimate_json" not in kinds(again) and "rate" not in kinds(again) and again[-1].done.status == "ready"
    assert not again[-1].done.estimate_json and json.loads(again[-1].done.state_json)["estimate"]["total"]["base"] > 0
    assert json.loads(again[-1].done.state_json)["turns"] == 2


def test_a_session_the_engine_era_agent_wrote_carries_on(agent_for):
    legacy = {"facts": {"kind": {"value": "apartment", "source": "user"}, "city": {"value": "Hanoi", "source": "user"}},
              "scope": {"r1": {"state": "empty", "source": "user"}}, "fixtures_existing": [], "excluded": [], "rate_overrides": {},
              "rates": [], "status": "interviewing", "pending": ["work_type"], "fingerprint": "", "estimate": None, "turns": 2}
    ag = agent_for()
    events = list(ag.stub.EstimateTurn(a.EstimateTurnRequest(model_json=b"plan", state_json=json.dumps(legacy).encode(), message="")))
    done = events[-1].done
    assert done.status == "interviewing" and json.loads(done.state_json)["turns"] == 3
    assert next(e.question for e in events if e.HasField("question")).id == "scope"  # the rest of the rooms


def test_rate_overrides_are_sent_in_the_request(agent_for):
    ag = agent_for()
    events = list(ag.stub.EstimateTurn(request(full_state(), "", rate_overrides={"paint": 90000.0})))
    estimate = json.loads(events[-1].done.estimate_json)
    paint = next(ln for ln in estimate["lines"] if ln["code"] == "paint")
    assert paint["rate_base"] == 90000 and paint["rate_confidence"] == "user"


def test_messages_are_the_chat_so_far(agent_for):
    ag = agent_for()
    list(ag.stub.EstimateTurn(request(SessionState(pending=["city"]), message="Hanoi", messages=[a.ChatMessage(role="assistant", content="Where is it?")])))
    assert any("Where is it?" in call["user"] for call in ag.llm.calls)


def test_an_invalid_session_is_a_plain_status_before_any_event(agent_for):
    ag = agent_for()
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.EstimateTurn(a.EstimateTurnRequest(model_json=b"plan", state_json=b"not json", message="hi")))
    assert e.value.code() == grpc.StatusCode.INVALID_ARGUMENT and detail(e.value).code == c.ERROR_CODE_EDIT_ERROR
    assert "state_json" in detail(e.value).message


def test_a_down_engine_before_the_stream_opens_is_a_plain_unavailable_status(agent_for):
    engine = EstimateEngine(fixtures=[])
    engine.down = True
    ag = agent_for(engine=engine)
    with pytest.raises(grpc.RpcError) as e:
        list(ag.stub.EstimateTurn(request(full_state(), "go")))
    assert e.value.code() == grpc.StatusCode.UNAVAILABLE and detail(e.value).dependency == "engine"


def test_an_engine_that_dies_mid_stream_is_an_in_band_502_and_nothing_is_priced(agent_for):
    ag = agent_for(TurnLanguageModel({}), engine=EstimateEngine(fixtures=[], fail_on="takeoff"))
    events = list(ag.stub.EstimateTurn(request(full_state(), "go")))
    assert kinds(events) == ["progress", "error"]
    assert (events[-1].error.status, events[-1].error.error) == (502, "engine_unavailable") and ag.prices.seen == []


def test_a_model_that_dies_mid_reply_is_an_in_band_502_after_its_first_text(agent_for):
    ag = agent_for(TurnLanguageModel(broken="mid", chunks=("part", "ial")))
    events = list(ag.stub.EstimateTurn(request()))
    assert kinds(events) == ["token", "error"]
    assert (events[-1].error.status, events[-1].error.error) == (502, "llm_unavailable")


def test_a_dead_model_before_any_text_falls_back_to_the_question_text(agent_for):
    events = list(agent_for(TurnLanguageModel(broken="dead")).stub.EstimateTurn(request()))
    assert kinds(events)[-1] == "done" and "located" in events[-1].done.reply and events[-1].done.status == "interviewing"


def test_a_turn_past_its_ceiling_ends_with_an_in_band_504(agent_for):
    events = list(agent_for(ceiling_seconds=-1).stub.EstimateTurn(request(full_state(), "")))
    assert kinds(events) == ["error"] and (events[0].error.status, events[0].error.error) == (504, "timeout")


def test_a_client_that_leaves_stops_the_turn_at_the_next_event(agent_for):
    chunks = tuple("x" for _ in range(400))
    llm = TurnLanguageModel(chunks=chunks, pause=0.01)
    ag = agent_for(llm)
    call = ag.stub.EstimateTurn(request())
    assert next(call).WhichOneof("event") == "token"
    call.cancel()
    time.sleep(0.3)
    sent = llm.sent
    time.sleep(0.3)
    assert sent < len(chunks) and llm.sent == sent  # the model is no longer being read from
