"""The estimate interview's reading of a message: the model's claims verified against the person's words, and code's certain
readings. Ported from the Engine-era estimate cases, against a scripted model and a scripted Engine catalog."""

import pytest
from estimate_fixtures import ESTIMATE_LIMITS, NO_FIXTURES, ROOMS, SUMMARY, ClaimsLanguageModel, full_state
from scripted_engine import ScriptedEngine

from fga_agent.domain.errors import LlmUnavailable
from fga_agent.domain.model.call_context import CallContext
from fga_agent.domain.model.estimate_session import EstimateFact, Message, SessionState
from fga_agent.service.estimate_interview import EstimateInterviewService
from fga_agent.service.estimate_reader import EstimateReaderService
from fga_agent.service.helper.claim_checker import ClaimCheckerService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.keyword_reader import KeywordReaderService

CTX = CallContext(request_id="t")


def services(*readings, rules=True, llm=None):
    llm = llm or ClaimsLanguageModel(*readings)
    catalog, keywords = DesignCatalogLoaderService(ScriptedEngine(limits=ESTIMATE_LIMITS)), KeywordReaderService()
    return llm, EstimateReaderService(llm, catalog, ClaimCheckerService(), keywords, rules), EstimateInterviewService(catalog, keywords, rules)


def hear(message, *readings, state=None, summary=SUMMARY, rules=True, history=()):
    """One turn's listening: read the message, record what it states. Returns the session after and the changes."""
    _, reader, interview = services(*readings, rules=rules)
    state = state or SessionState()
    extracted = reader.read(CTX, summary, state, message, list(history))
    return interview.record(CTX, state, summary, extracted, message)


def bathrooms():
    return {r.id for r in ROOMS if r.type == "bathroom"}


# --- facts the code reads, whatever the model says ------------------------------------------------------------

def test_a_place_answer_is_split_by_code_and_an_empty_shell_is_a_fit_out():
    # the model hands back the whole answer as the city; code knows the city and keeps the rest as the district
    s, _ = hear("District 7, Ho Chi Minh City", {"city": "District 7, Ho Chi Minh City", "district": "Ho Chi Minh City"},
                state=SessionState(pending=["city"]))
    assert s.facts["city"].value == "Ho Chi Minh City" and s.facts["district"].value == "District 7"
    s, _ = hear("Thu Duc, Ho Chi Minh City", {"city": "Thu Duc"}, state=SessionState(pending=["city"]))
    assert s.facts["city"].value == "Ho Chi Minh City" and s.facts["district"].value == "Thu Duc"
    # "empty shell" already says what kind of work it is
    s, _ = hear("A house in Hanoi, only the empty shell", {"kind": "house", "scope_default": "empty"})
    assert s.facts["work_type"].value == "fit_out"
    s, _ = hear("Renovation of an empty shell")
    assert s.facts["work_type"].value == "renovation"


def test_a_named_city_and_an_explicit_floor_height_never_depend_on_the_model():
    s, _ = hear("Apartment in Ho Chi Minh City, floor height 3.2 m")
    assert s.facts["city"].value == "Ho Chi Minh City" and s.facts["city"].source == "user"
    assert s.facts["floor_height"].value == 3.2


def test_include_or_exclude_mep_is_read_by_code():
    assert hear("include MEP")[0].facts["mep_in_scope"].value is True
    assert hear("without plumbing please")[0].facts["mep_in_scope"].value is False


def test_work_type_and_structure_keywords_are_read_by_code():
    s, _ = hear("fit-out only, finishes only")
    assert s.facts["work_type"].value == "fit_out" and s.facts["structure_in_scope"].value is False


def test_code_reads_what_the_model_missed_and_drops_an_unprompted_scope():
    s, _ = hear("This is an apartment, the rooms already exist, normal quality")  # the model returns nothing
    assert s.facts["kind"].value == "apartment" and s.facts["quality"].value == "normal"
    s, _ = hear("the rooms already exist", {"scope_default": "existing"})  # the model invents "existing"
    assert s.scope == {}


def test_the_agents_own_chips_are_understood_without_the_model():
    s, _ = hear("Ho Chi Minh City", state=SessionState(pending=["city"]))
    assert s.facts["city"] == EstimateFact("Ho Chi Minh City", "user")
    s, _ = hear("Everything is empty except the bathrooms", state=SessionState(pending=["scope"]))
    assert {i for i, sc in s.scope.items() if sc.state == "existing"} == bathrooms()
    assert all(sc.state == "empty" for i, sc in s.scope.items() if i not in bathrooms())
    for chip, field, want in [("3.3 m", "floor_height", 3.3), ("2", "storeys", 2)]:
        assert hear(chip, state=SessionState(pending=[field]))[0].facts[field].value == want
    assert hear("Fit-out of an empty shell", state=SessionState(pending=["work_type"]))[0].facts["work_type"].value == "fit_out"
    assert hear("No, exclude them", state=SessionState(pending=["mep_in_scope"]))[0].facts["mep_in_scope"].value is False


# --- rooms: only what the message refers to ----------------------------------------------------------------------

def test_exist_alone_never_sets_every_room():
    every = [{"room_id": r.id, "state": "existing"} for r in ROOMS]
    assert hear("the rooms already exist", {"scope_rooms": every})[0].scope == {}
    s, _ = hear("the Wc already exists", {"scope_rooms": [{"room_id": "r6", "state": "existing"}]})
    assert list(s.scope) == ["r6"]


def test_a_room_can_be_marked_only_when_the_message_refers_to_it():
    wrong = [{"room_id": i, "state": "existing"} for i in sorted(bathrooms()) + ["r7"]]  # the model adds the balcony
    s, _ = hear("Except the toilets, everything is empty", {"scope_default": "empty", "scope_rooms": wrong})
    assert {i for i, sc in s.scope.items() if sc.state == "existing"} == bathrooms()
    every_bedroom = [{"room_id": "r3", "state": "existing"}, {"room_id": "r4", "state": "existing"}]
    s, _ = hear("Bedroom 2 is already finished", {"scope_rooms": every_bedroom})
    assert list(s.scope) == ["r4"]  # a named room is not widened to every bedroom


# --- work taken out of the estimate and put back ---------------------------------------------------------------------

def test_the_model_cannot_remove_items_the_message_never_mentions():
    st = full_state()
    st.estimate = {"total": 1}
    s, changes = hear("What is the biggest cost?", {"exclude_items": ["floor_tile"]}, state=st)
    assert s.excluded == [] and changes == []


def test_removing_and_putting_back_an_item_is_read_by_code():
    st = full_state()
    st.estimate = {"total": 1}
    gone, changes = hear("Remove the cost for the doors, they already exist", state=st)
    assert gone.excluded == ["door_entrance", "door_interior"] and changes[-1].name == "excluded"
    gone, _ = hear("skip the windows", state=st)
    back, _ = hear("add the windows back", state=gone)
    assert gone.excluded == ["window"] and back.excluded == []


def test_an_exception_in_the_first_message_takes_that_opening_out():
    msg = ("This is an apartment in Da Nang, estimate the cost with normal quality.\n- Fixtures are almost empty, except entrance door.\n"
           "- Rooms are empty cells")
    s, _ = hear(msg)
    assert s.excluded == ["door_entrance"] and s.facts["fixtures"].value == "none exist"


def test_exception_and_only_phrases_name_openings_only():
    kw = KeywordReaderService()
    assert kw.exception_items("everything is empty except the entrance door") == ["door_entrance"]
    assert kw.exception_items("only the windows are installed") == ["window"]
    assert kw.exception_items("everything is empty except the toilets") == []  # a fixture: its own slot
    assert kw.exception_items("estimate the cost, normal quality") == []


# --- reading any language: the quote check, with the keyword rules off ----------------------------------------------------

@pytest.mark.parametrize("quote,message,ok", [
    ("except entrance door", "Fixtures are almost empty, except entrance door.", True),
    ("EXCEPT   the Entrance Door", "fixtures are empty, except the entrance door", True),
    ("trừ cửa chính", "Trừ cửa chính, thiết bị chưa có cái nào", True),
    ("tru cua chinh", "Trừ cửa chính, thiết bị chưa có", True),                 # accents dropped by the model
    ("入口のドアを除いて", "入口のドアを除いて、設備は何もありません", True),
    ("typical percentages", "This is an apartment in Da Nang", False),           # invented
    ("the doors exist", "Fixtures are empty, except the entrance door", False),  # not said
    ("a", "a b c", False),                                                        # too short to prove anything
])
def test_a_quote_counts_only_when_it_is_in_the_message(quote, message, ok):
    assert ClaimCheckerService().grounded(quote, message) is ok


def test_values_without_a_verified_quote_are_dropped():
    reading = {"use_typical_markups": True, "exclude_items": ["door_interior"], "scope_default": "existing",
               "evidence": {"use_typical_markups": "typical percentages", "exclude_items": "doors exist"}}
    s, _ = hear("This is an apartment, the rooms already exist, normal quality", reading, rules=False)
    assert s.excluded == [] and s.scope == {} and not any(k.endswith("_percent") for k in s.facts)


@pytest.mark.parametrize("message,quote,items", [
    ("Fixtures are almost empty, except entrance door.", "except entrance door", ["door_entrance"]),
    ("Trừ cửa chính, thiết bị chưa có cái nào", "Trừ cửa chính", ["door_entrance"]),
    ("入口のドアを除いて、設備は何もありません", "入口のドアを除いて", ["door_entrance"]),
])
def test_the_model_reads_an_exception_in_any_language(message, quote, items):
    reading = {"exclude_items": items, "fixtures_none_exist": True, "evidence": {"exclude_items": quote, "fixtures_none_exist": message}}
    s, _ = hear(message, reading, rules=False)
    assert s.excluded == items and s.facts["fixtures"].value == "none exist"


def test_a_claim_with_a_bad_value_is_dropped_and_the_others_kept():
    reading = {"floor_height": 9.0, "storeys": 3, "kind": "castle", "quality": "good", "exclude_items": ["no_such_code", "window"]}
    s, _ = hear("Ceilings 9 m, 3 storeys, a castle, good quality, skip the windows", reading, rules=False)
    assert "floor_height" not in s.facts and "kind" not in s.facts and s.facts["storeys"].value == 3
    assert s.facts["quality"].value == "good" and s.excluded == ["window"]


# --- how the readings are asked ------------------------------------------------------------------------------------------

def test_four_readings_run_for_every_message_with_no_live_thinking_and_a_short_budget():
    llm, reader, _ = services()
    reader.read(CTX, SUMMARY, SessionState(), "hi", [])
    assert len(llm.calls) == 4 and [c["think"] for c in llm.calls] == [False] * 4 and {c["max_tokens"] for c in llm.calls} == {300}


def test_the_readings_are_built_from_the_plan_the_catalog_and_the_question_just_asked():
    llm, reader, _ = services()
    state = SessionState(facts={"city": EstimateFact("Hanoi", "user")}, pending=["fixtures"])
    reader.read(CTX, SUMMARY, state, "none", [Message("user", "hi"), Message("assistant", "Which fixtures already exist?")])
    users = "\n".join(c["user"] for c in llm.calls)
    assert "- r5: Bath (bathroom, 4.5 m²)" in users and "- f1: toilet in r5" in users and "- f4: bed" not in users  # priced fixtures only
    assert "- city = Hanoi (user)" in users and "Question just asked (slot fixtures): Which fixtures already exist?" in users
    assert any("fixture_toilet" in c["system"] and "door_entrance" in c["system"] for c in llm.calls)  # the catalog's rate codes
    assert len({c["system"] for c in llm.calls}) == 4  # four distinct readings


def test_a_dead_model_is_not_hidden():
    class Down:
        def answer_json(self, *a, **k):
            raise LlmUnavailable("ollama down")

    _, reader, _ = services(llm=Down())
    with pytest.raises(LlmUnavailable):
        reader.read(CTX, NO_FIXTURES, SessionState(), "apartment in Da Nang", [])
