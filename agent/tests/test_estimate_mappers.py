"""The estimate DTO mappers: the session JSON both ways, and each part of the stream message."""

import json

import pytest
from estimate_engine import EstimateEngine
from estimate_fixtures import full_state

from fga_agent.domain.errors import InvalidEdit
from fga_agent.domain.model.estimate_session import EstimateFact, FactChange, Question, RoomScope, SessionState
from fga_agent.domain.model.estimate_turn import EstimateTurnResult
from fga_agent.domain.model.progress import Progress
from fga_agent.domain.model.rate import RateResult, RateSource
from fga_agent.domain.model.reply_token import ReplyToken
from fga_agent.handler.grpc.mapper import (error_mapper, estimate_event_mapper, estimate_turn_mapper, fact_change_mapper,
                                           question_mapper, rate_mapper, session_state_mapper)
from fga_agent.domain.errors import TurnTimedOut
from fga.demo.agent.v1 import agent_pb2 as a


def priced_state() -> SessionState:
    state = full_state()
    rates = [RateResult(code="floor_tile", unit="m2", low=1.0, base=2.0, high=3.0, confidence="ok", region="Hanoi", quality="good",
                        sources=[RateSource("https://x.test", "q", "2026-11-16")]), RateResult(code="paint", unit="m2", note="none")]
    state.rates, state.fingerprint, state.status, state.turns = rates, "abc", "ready", 3
    state.excluded, state.fixtures_existing, state.rate_overrides = ["window"], ["f1"], {"paint": 9.0}
    state.estimate = EstimateEngine(fixtures=[]).price_estimate(None, EstimateEngine(fixtures=[]).takeoff(None, b"", _facts(state)), rates, _facts(state))
    return state


def _facts(state):
    from fga_agent.service.estimate_interview import EstimateInterviewService
    from estimate_engine import EstimateEngine as E

    return EstimateInterviewService(E(), None).facts(state)


def test_a_session_survives_its_json_unchanged():
    state = priced_state()
    again = session_state_mapper.from_json(session_state_mapper.to_json(state))
    assert again == state and again.estimate.total["base"] > 0 and again.rates[0].sources[0].url == "https://x.test"


def test_the_session_json_has_the_shape_the_engine_era_agent_wrote():
    doc = json.loads(session_state_mapper.to_json(priced_state()))
    assert set(doc) == {"facts", "scope", "fixtures_existing", "excluded", "rate_overrides", "rates", "status", "pending", "fingerprint",
                        "estimate", "turns"}
    assert doc["facts"]["quality"] == {"value": "normal", "source": "user"} and doc["scope"]["r1"] == {"state": "empty", "source": "user"}
    assert set(doc["rates"][0]) == {"code", "low", "base", "high", "unit", "currency", "confidence", "region", "quality", "note", "sources"}


def test_no_session_is_a_fresh_one_and_a_broken_session_is_refused():
    assert session_state_mapper.from_json(b"") == SessionState()
    for bad in (b"not json", b'{"facts": {"city": 5}}', b'{"scope": {"r1": {"state": "empty"}}}', b"[]"):
        with pytest.raises(InvalidEdit):
            session_state_mapper.from_json(bad)


def test_a_fact_travels_as_text():
    assert fact_change_mapper.to_pb(FactChange("mep_in_scope", True, "user")).value == "true"
    assert fact_change_mapper.to_pb(FactChange("floor_height", 3.0, "assumed")).value == "3"
    assert fact_change_mapper.to_pb(FactChange("floor_height", 2.85, "user")).value == "2.85"
    assert fact_change_mapper.to_pb(FactChange("city", "Hanoi", "user")).value == "Hanoi"
    assert fact_change_mapper.to_domain(fact_change_mapper.to_pb(FactChange("city", "Hanoi", "user"))) == FactChange("city", "Hanoi", "user")


def test_every_part_of_a_turn_has_its_slot_in_the_stream_message():
    estimate = priced_state().estimate
    q = Question("city", "Where?", ["Hanoi"])
    seen = {estimate_event_mapper.to_event(x).WhichOneof("event"): x for x in (
        Progress("step", "research", "m"), ReplyToken("hi"), FactChange("city", "Hanoi", "user"), q,
        RateResult(code="paint", unit="m2", base=1.0), estimate)}
    assert set(seen) == {"progress", "token", "fact", "question", "rate", "estimate_json"}
    assert json.loads(estimate_event_mapper.to_event(estimate).estimate_json) == estimate.document
    assert question_mapper.to_domain(question_mapper.to_pb(q)) == q
    assert rate_mapper.to_domain(rate_mapper.to_pb(RateResult(code="paint", unit="m2", base=1.0))) == RateResult(code="paint", unit="m2", base=1.0)
    with pytest.raises(TypeError):
        estimate_event_mapper.to_event(object())


def test_done_carries_the_estimate_only_when_the_turn_priced_the_plan():
    state = priced_state()
    done = estimate_turn_mapper.to_done_event(EstimateTurnResult(state, "reply")).done
    assert done.status == "ready" and done.reply == "reply" and not done.estimate_json and json.loads(done.state_json)["turns"] == 3
    priced = estimate_turn_mapper.to_done_event(EstimateTurnResult(state, "reply", estimate=state.estimate)).done
    assert json.loads(priced.estimate_json) == state.estimate.document


def test_the_request_is_decoded_with_the_chat_and_the_rates():
    request = a.EstimateTurnRequest(model_json=b"plan", state_json=b"", message="hi", rate_overrides={"paint": 5.0},
                                    messages=[a.ChatMessage(role="user", content="a"), a.ChatMessage(role="assistant", content="b")])
    turn = estimate_turn_mapper.to_domain(request)
    assert turn.model == b"plan" and turn.state == SessionState() and turn.message == "hi" and turn.rate_overrides == {"paint": 5.0}
    assert [(m.role, m.content) for m in turn.messages] == [("user", "a"), ("assistant", "b")]


def test_a_turn_that_ran_too_long_is_a_504_in_band():
    err = error_mapper.to_estimate_error_event(TurnTimedOut("30 minutes")).error
    assert (err.status, err.error) == (504, "timeout") and error_mapper.to_status(TurnTimedOut("x")) is None
