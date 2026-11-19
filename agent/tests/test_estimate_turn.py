"""One turn of the estimate conversation, end to end over scripted doubles: the interview asks before it assumes, the plan is priced
once its questions are answered, an unchanged answer is not priced again, and every reply is given only the figures it may say.
Ported from the Engine-era estimate agent."""

import pytest
from estimate_engine import EstimateEngine, FakePrices, SENSITIVE
from estimate_fixtures import ROOMS, SessionState, TurnLanguageModel, full_state

from fga_agent.domain.errors import Cancelled, EngineUnavailable, LlmUnavailable, TurnTimedOut
from fga_agent.domain.model.call_context import CallContext
from fga_agent.domain.model.estimate import Estimate
from fga_agent.domain.model.estimate_session import FactChange, Message, Question
from fga_agent.domain.model.estimate_turn import EstimateTurnRequest
from fga_agent.domain.model.progress import Progress
from fga_agent.domain.model.rate import RateResult
from fga_agent.domain.model.reply_token import ReplyToken
from fga_agent.service.estimate_interview import EstimateInterviewService
from fga_agent.service.estimate_pricer import EstimatePricerService
from fga_agent.service.estimate_reader import EstimateReaderService
from fga_agent.service.estimate_replier import EstimateReplierService
from fga_agent.service.estimate_turner import EstimateTurnerService
from fga_agent.service.helper.claim_checker import ClaimCheckerService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.keyword_reader import KeywordReaderService
from fga_agent.service.prompts import estimate_questions as ask

CTX = CallContext(request_id="t")
WC = ["r5", "r6"]  # the two bathrooms of the plan


class Turns:
    """A turner built on the scripted Engine, researcher and model; `say()` plays one turn and returns (result, events). Every message
    with text uses the next scripted reading, so a script starts with `{}` for a first message that states nothing."""

    def __init__(self, *readings, fixtures=(), prices=None, llm=None, engine=None, **turner) -> None:
        # `fixtures` is the plan's fixtures: none by default (so nothing is asked about them), None for the plan's own four
        self.engine = engine or (EstimateEngine() if fixtures is None else EstimateEngine(fixtures=list(fixtures)))
        self.prices = prices or FakePrices()
        self.llm = llm or TurnLanguageModel(*readings)
        catalog, keywords = DesignCatalogLoaderService(self.engine), KeywordReaderService()
        self.turner = EstimateTurnerService(
            self.engine, catalog, EstimateReaderService(self.llm, catalog, ClaimCheckerService(), keywords),
            EstimateInterviewService(catalog, keywords), EstimatePricerService(self.engine, catalog, self.prices),
            EstimateReplierService(self.llm), **turner)

    def say(self, state, message, overrides=None, history=(), on_event=None):
        events = []
        result = self.turner.turn(CTX, EstimateTurnRequest(b"plan", state, message, list(history), overrides or {}),
                                  on_event or events.append)
        return result, events


def kinds(events):
    return [type(e).__name__ for e in events]


# --- the interview ---------------------------------------------------------------------------------------------------

def test_the_example_conversation_asks_before_it_assumes():
    t = Turns({"kind": "apartment", "quality": "normal"}, {"city": "Ho Chi Minh City"},
              {"scope_default": "empty", "scope_rooms": [{"room_id": i, "state": "existing"} for i in WC], "work_type": "fit_out"})
    r1, _ = t.say(SessionState(), "This is an apartment, the rooms already exist, normal quality")
    assert r1.question.id == "city" and r1.state.status == "interviewing" and t.prices.seen == []
    assert r1.state.facts["quality"].source == "user"
    r2, _ = t.say(r1.state, "Ho Chi Minh City")
    assert r2.question.id == "scope" and any("except" in o for o in r2.question.options) and "city" not in r2.state.pending
    r3, _ = t.say(r2.state, "Except the toilets, everything is empty")
    assert r3.question.id == "floor_height"  # the next missing thing, not an estimate
    assert {r3.state.scope[i].state for i in WC} == {"existing"} and t.prices.seen == [] and r3.estimate is None


def test_missing_slots_come_from_code_not_the_model():
    # a model that reads nothing cannot make the agent skip a question or invent an answer
    r, _ = Turns().say(SessionState(), "hello")
    assert r.question.id == "city" and r.estimate is None and r.state.pending[:3] == ["city", "kind", "scope"]


def test_chips_match_the_slot_asked_and_always_offer_you_decide():
    r, _ = Turns().say(SessionState(), "hi")
    assert r.question.id == "city" and r.question.options[-1] == ask.UNKNOWN_OPTION
    t = Turns()
    state = SessionState()
    for want in ("city", "kind"):
        r, _ = t.say(state, "")
        assert r.question.id == want and r.question.allow_unknown and r.question.options[-1] == ask.UNKNOWN_OPTION
        state = r.state
        state.facts[want] = full_state().facts[want]


def test_a_turn_with_no_message_only_continues_the_interview():
    t = Turns()
    r, events = t.say(SessionState(), "  ")
    assert r.question.id == "city" and t.llm.calls == [] and not any(isinstance(e, (FactChange, Progress)) for e in events)


def test_the_agent_asks_which_fixtures_exist_before_it_prices():
    fx = Turns(fixtures=None)  # the plan's own fixtures
    state = full_state()
    r, _ = fx.say(state, "go")
    assert r.question.id == "fixtures" and r.estimate is None
    assert "assume none of them exist" in r.question.text and "toilet (Bath)" in r.question.text
    assert r.question.options[:2] == ["None of them exist", "All of them already exist"]
    none, _ = fx.say(r.state, "None of them exist")
    assert none.estimate is not None and none.state.facts["fixtures"].source == "user"
    assert "fixture_toilet" in {ln.code for ln in none.estimate.lines}
    every, _ = fx.say(r.state, "All of them already exist")
    assert not [ln for ln in every.estimate.lines if ln.package == "Fixtures"]


def test_some_fixtures_exist_only_those_the_message_refers_to():
    t = Turns({"fixtures_existing": ["f1", "f3"]}, fixtures=None)  # the first message is the one read
    state = full_state()
    state.pending = ["fixtures"]
    r, _ = t.say(state, "The toilet in the bath is already installed")  # the model also claims the kitchen counter
    assert r.state.fixtures_existing == ["f1"] and r.state.facts["fixtures"].value == "some exist"
    codes = {ln.code for ln in r.estimate.lines}
    assert "fixture_toilet" not in codes and {"fixture_basin", "fixture_kitchen_counter"} <= codes


def test_i_dont_know_assumes_no_fixture_exists_and_says_so():
    t = Turns(fixtures=None)
    state = full_state()
    state.pending = ["fixtures"]
    r, _ = t.say(state, ask.UNKNOWN_OPTION)
    assert r.state.facts["fixtures"].source == "assumed" and r.state.fixtures_existing == []
    assert any("fixtures" in a for a in r.estimate.assumptions)


def test_i_dont_know_is_recorded_as_assumed_and_shown_in_the_result():
    state = full_state()
    del state.facts["floor_height"]
    state.pending = ["floor_height"]
    r, _ = Turns().say(state, ask.UNKNOWN_OPTION)
    assert r.state.facts["floor_height"].source == "assumed"
    assert r.estimate is not None and any("floor_height" in a for a in r.estimate.assumptions)


def test_several_slots_in_one_message_and_typical_markups():
    reading = {"city": "Hanoi", "kind": "apartment", "work_type": "fit_out", "quality": "good", "floor_height": 2.8,
               "mep_in_scope": True, "scope_default": "empty", "use_typical_markups": True}
    r, _ = Turns(reading).say(SessionState(), "Hanoi apartment fit-out, good quality, 2.8 m, with MEP, all empty, use typical rates")
    assert r.question is None and r.estimate is not None and r.state.status == "ready"
    assert r.state.facts["vat_percent"].source == "assumed"
    assert {ln.code for ln in r.estimate.lines} <= {x.code for x in r.estimate.rates}
    assert "electrical_point" in {ln.code for ln in r.estimate.lines}


def test_a_drawing_without_rooms_fails_cleanly():
    t = Turns({"city": "Hanoi", "kind": "apartment", "work_type": "fit_out", "quality": "normal", "floor_height": 3,
               "mep_in_scope": False, "use_typical_markups": True}, engine=EstimateEngine(rooms=[], fixtures=[]))
    r, _ = t.say(SessionState(), "use typical rates")
    assert r.state.status == "failed" and r.estimate is None and r.question is None
    assert "no rooms" in t.llm.streams[-1]


# --- pricing ---------------------------------------------------------------------------------------------------------------

def test_changing_an_answer_prices_again_only_what_changed():
    t = Turns({}, {"vat_percent": 10}, {"quality": "luxury"}, {})
    first, _ = t.say(full_state(), "go")
    searched = {r.code for r in t.prices.seen}
    assert first.estimate is not None and searched and first.state.status == "ready" and first.state.rates
    t.prices.seen.clear()
    vat, _ = t.say(first.state, "VAT is 10%")
    assert t.prices.seen == [] and vat.estimate.total["base"] > first.estimate.total["base"]  # markups need no search
    quality, _ = t.say(vat.state, "luxury")
    assert {r.code for r in t.prices.seen} == {c for c in searched if SENSITIVE[c]}  # plaster keeps its rate
    unchanged, _ = t.say(quality.state, "thanks")
    assert unchanged.estimate is None and unchanged.state.estimate is not None  # nothing changed: no new estimate


def test_the_scope_changes_the_total():
    t = Turns({}, {"scope_rooms": [{"room_id": i, "state": "existing"} for i in WC]})
    a, _ = t.say(full_state(), "go")
    b, _ = t.say(a.state, "toilets exist")
    assert b.estimate.total["base"] < a.estimate.total["base"]


def test_unpriced_codes_are_flagged_and_a_user_rate_fills_them():
    prices = FakePrices(missing={"paint"})
    t = Turns(prices=prices)
    a, _ = t.say(full_state(), "go")
    assert any("paint" in w.lower() for w in a.estimate.warnings)
    b, _ = t.say(a.state, "", overrides={"paint": 90000, "no_such_code": 5, "plaster": -1})
    paint = next(ln for ln in b.estimate.lines if ln.code == "paint")
    assert paint.priced and paint.rate_base == 90000 and paint.rate_confidence == "user"
    assert b.state.rate_overrides == {"paint": 90000} and not any(ln.code == "plaster" and ln.rate_confidence == "user" for ln in b.estimate.lines)


def test_no_search_provider_means_unpriced_lines_with_a_reason_in_the_estimate():
    class Unconfigured(FakePrices):
        def research(self, ctx, requests, on_progress, on_rate=None):
            out = [RateResult(code=r.code, unit=r.unit, confidence="none", note="no search provider is configured") for r in requests]
            for x in out:
                on_rate(x)
            return out

    r, _ = Turns(prices=Unconfigured()).say(full_state(), "go")
    assert r.estimate.total["base"] == 0 and all(not ln.priced for ln in r.estimate.lines)
    assert all("provider" in x.note for x in r.estimate.rates)


def test_explaining_a_line():
    t = Turns({}, {"explain": "floor tile"})
    a, _ = t.say(full_state(), "go")
    b, _ = t.say(a.state, "why is the floor tile so high?")
    assert b.estimate is None and "floor_tile" in t.llm.streams[-1] and "net floor area" in t.llm.streams[-1]


def test_a_message_that_changes_nothing_never_claims_a_change():
    t = Turns()
    a, _ = t.say(full_state(), "go")
    t.say(a.state, "what is the biggest cost?")
    assert "Never say or imply that anything was updated" in t.llm.streams[-1]


def test_the_reply_after_a_real_change_is_given_exactly_what_changed():
    t = Turns()
    a, _ = t.say(full_state(), "go")
    t.say(a.state, "the doors already exist")
    assert "changes_applied_this_turn" in t.llm.streams[-1] and "door_interior" in t.llm.streams[-1]
    assert "excluded = door_entrance, door_interior" in t.llm.streams[-1]


def test_removing_an_item_from_a_finished_estimate_really_removes_it():
    t = Turns()
    a, _ = t.say(full_state(), "go")
    b, _ = t.say(a.state, "Remove the cost for the doors, they already exist")
    codes = {ln.code for ln in b.estimate.lines}
    assert b.state.excluded == ["door_entrance", "door_interior"] and not codes & {"door_interior", "door_entrance"}
    assert b.estimate.total["base"] < a.estimate.total["base"]
    c, _ = t.say(b.state, "add the doors back")
    assert c.state.excluded == [] and {"door_interior", "door_entrance"} <= {ln.code for ln in c.estimate.lines}


def test_the_session_in_the_request_is_never_modified():
    state = full_state()
    before = (dict(state.facts), dict(state.scope), state.turns, state.status)
    t = Turns({"vat_percent": 10})
    t.say(state, "VAT is 10%")
    assert (state.facts, state.scope, state.turns, state.status) == (before[0], before[1], before[2], before[3])


# --- what a turn reports while it runs --------------------------------------------------------------------------------------

def test_a_question_turn_streams_progress_facts_tokens_then_the_question():
    t = Turns({"city": "Hanoi"}, llm=TurnLanguageModel({"city": "Hanoi"}, chunks=("Which ", "kind?")))
    r, events = t.say(SessionState(), "Hanoi")
    names = kinds(events)
    assert names[0] == "Progress" and names.index("FactChange") < names.index("ReplyToken") < names.index("Question")
    assert "".join(e.text for e in events if isinstance(e, ReplyToken)) == "Which kind?" == r.reply
    assert isinstance(events[-1], Question) and events[-1].id == r.question.id == "kind" and not any(isinstance(e, Estimate) for e in events)
    assert next(e for e in events if isinstance(e, FactChange)) == FactChange("city", "Hanoi", "user")


def test_an_estimate_turn_streams_rates_then_the_estimate_then_the_closing_reply():
    r, events = Turns().say(full_state(), "go")
    names = kinds(events)
    assert names.count("RateResult") >= 5
    assert names.index("RateResult") < names.index("Estimate") < names.index("ReplyToken")
    assert r.estimate.total["base"] > 0 and r.state.status == "ready" and r.question is None
    stages = [e.stage for e in events if isinstance(e, Progress)]
    assert stages == ["understand", "research", "compute"]


def test_a_dead_model_before_any_text_falls_back_to_the_question_text():
    r, events = Turns(llm=TurnLanguageModel(broken="dead")).say(SessionState(), "")
    assert "located" in r.reply and r.reply == r.question.text
    assert [e.text for e in events if isinstance(e, ReplyToken)] == [r.reply]


def test_a_model_failure_mid_reply_is_not_hidden():
    t = Turns(llm=TurnLanguageModel(broken="mid", chunks=("part", "ial")))
    with pytest.raises(LlmUnavailable):
        t.say(SessionState(), "")


def test_a_model_that_cannot_read_the_message_is_not_hidden():
    class Down(TurnLanguageModel):
        def answer_json(self, *a, **k):
            raise LlmUnavailable("ollama down")

    with pytest.raises(LlmUnavailable):
        Turns(llm=Down()).say(SessionState(), "Hanoi")


def test_the_reading_is_given_the_chat_so_far():
    t = Turns()
    t.say(SessionState(pending=["city"]), "Hanoi", history=[Message("user", "hi"), Message("assistant", "Where is it?")])
    assert any("Where is it?" in c["user"] for c in t.llm.calls)


# --- the Engine and the caller -----------------------------------------------------------------------------------------------

def test_an_engine_that_is_down_is_not_hidden_and_the_turn_asks_nothing():
    engine = EstimateEngine(fixtures=[])
    engine.down = True
    with pytest.raises(EngineUnavailable):
        Turns(engine=engine).say(full_state(), "go")
    failing = EstimateEngine(fixtures=[], fail_on="takeoff")
    t = Turns(engine=failing)
    with pytest.raises(EngineUnavailable):
        t.say(full_state(), "go")
    assert t.prices.seen == [] and t.llm.streams == []  # nothing was researched or said


def test_the_engine_is_asked_with_the_sessions_facts_and_the_plan_as_sent():
    t = Turns({"quality": "luxury"})
    t.say(full_state(), "luxury")
    model, facts = t.engine.summarized[-1]
    assert model == b"plan" and facts.facts["quality"].value == "luxury" and facts.scope["r1"].state == "empty"
    assert t.engine.takeoffs[-1].facts["quality"].value == "luxury"


def test_a_caller_that_leaves_stops_the_turn_at_its_next_event():
    seen = []

    def leave(event):
        seen.append(event)
        if isinstance(event, Progress) and event.stage == "research":
            raise Cancelled()

    t = Turns()
    state = full_state()
    with pytest.raises(Cancelled):
        t.say(state, "go", on_event=leave)
    assert t.prices.seen == [] and t.llm.streams == [] and state.estimate is None and state.turns == 0  # nothing said, nothing kept


def test_a_turn_past_its_ceiling_stops_with_a_timeout():
    now = [0.0]

    def clock():
        now[0] += 100
        return now[0]

    with pytest.raises(TurnTimedOut):
        Turns(ceiling_seconds=250, clock=clock).say(full_state(), "go")
    assert Turns(ceiling_seconds=1e9).say(full_state(), "go")[0].estimate is not None  # a long ceiling does not stop it


def test_the_rooms_of_the_plan_are_the_ones_the_interview_lists():
    assert {r.id for r in ROOMS if r.type == "bathroom"} == set(WC)
