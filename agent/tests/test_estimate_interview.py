"""The estimate interview: which questions are open (decided by code), what each message changes, and what a session is.
Ported from the Engine-era estimate cases."""

from dataclasses import replace

from estimate_fixtures import MARKUPS, NO_FIXTURES, ROOMS, SUMMARY, full_state
from test_estimate_reading import CTX, hear, services

from fga_agent.domain.model.estimate_session import SessionState
from fga_agent.domain.model.extracted import Extracted, RoomStateClaim
from fga_agent.service.prompts import estimate_questions as ask


def interview():
    return services()[2]


def open_ids(state, summary=NO_FIXTURES):
    return [q.id for q in interview().missing(CTX, state, summary)]


# --- the questions come from code, in order ---------------------------------------------------------------------------------

def test_missing_slots_come_from_code_not_the_model():
    assert open_ids(SessionState())[:3] == ["city", "kind", "scope"]  # a model that reads nothing cannot skip a question
    s, _ = hear("hello")
    assert open_ids(s)[0] == "city" and s.facts == {}


def test_every_slot_has_a_question_and_the_slot_names_are_the_questions_ids():
    ids = [q.id for q in interview().missing(CTX, SessionState(), replace(SUMMARY, structure_in_scope=True))]
    assert set(ids) <= set(ask.SLOT_NAMES) and ids[0] == "city" and "markups" in ids


def test_a_complete_interview_has_no_open_question():
    assert open_ids(full_state()) == []


def test_the_fixtures_question_comes_only_when_the_plan_has_pending_priced_fixtures():
    state = full_state()
    assert "fixtures" not in open_ids(state, NO_FIXTURES)
    q = next(q for q in interview().missing(CTX, state, SUMMARY) if q.id == "fixtures")
    assert "assume none of them exist" in q.text and "toilet (Bath)" in q.text and "kitchen counter (Kitchen)" in q.text
    assert "bed" not in q.text and q.options[:2] == ["None of them exist", "All of them already exist"]


def test_structure_is_asked_for_a_renovation_and_storeys_only_when_the_engine_says_structure_is_in_scope():
    reno = full_state(work_type="renovation")
    assert open_ids(reno) == ["structure_in_scope"]
    assert "storeys" not in open_ids(reno) and "storeys" in open_ids(reno, replace(NO_FIXTURES, structure_in_scope=True))


def test_the_markups_question_names_only_the_percentages_still_missing():
    state = full_state()
    for n in ("vat_percent", "profit_percent"):
        del state.facts[n]
    q = next(q for q in interview().missing(CTX, state, NO_FIXTURES) if q.id == "markups")
    assert "VAT" in q.text and "contractor profit" in q.text and "contingency" not in q.text and q.options == ["Use typical rates"]


def test_the_scope_question_lists_the_rooms_not_yet_decided_and_offers_the_bathroom_chip_while_one_is_open():
    state = full_state()
    del state.scope["r1"]
    q = next(q for q in interview().missing(CTX, state, NO_FIXTURES) if q.id == "scope")
    assert "Living" in q.text and "Kitchen" not in q.text and "except the bathrooms" not in " ".join(q.options)
    q = interview().missing(CTX, SessionState(), NO_FIXTURES)[2]
    assert "Everything is empty except the bathrooms" in q.options


# --- what a message changes ---------------------------------------------------------------------------------------------------

def test_flags_the_model_sets_unprompted_do_not_assume_anything():
    reading = {"kind": "apartment", "quality": "normal", "use_typical_markups": True, "decide_for_me": ["markups", "city", "scope"]}
    s, _ = hear("This is an apartment, the rooms already exist, normal quality", reading)
    assert set(s.facts) == {"kind", "quality"} and s.scope == {}
    assert all(f.source == "user" for f in s.facts.values())
    assert open_ids(s)[0] == "city"


def test_typical_and_give_up_phrases_still_work_when_said():
    state = full_state()
    for n in MARKUPS:
        del state.facts[n]
    state.pending = ["markups"]
    s, _ = hear("use typical rates", state=state)
    assert all(s.facts[n].source == "assumed" and s.facts[n].value == v for n, v in MARKUPS.items())
    state = full_state()
    del state.facts["quality"]
    state.pending = ["quality"]
    s, _ = hear("Không biết, bạn quyết", {"decide_for_me": ["quality"]}, state=state)
    assert s.facts["quality"].source == "assumed" and s.facts["quality"].value == "normal"


def test_i_dont_know_is_recorded_as_assumed():
    state = full_state()
    del state.facts["floor_height"]
    state.pending = ["floor_height"]
    s, changes = hear(ask.UNKNOWN_OPTION, state=state)
    assert s.facts["floor_height"].source == "assumed" and s.facts["floor_height"].value == 3.0
    assert [(c.name, c.source) for c in changes] == [("floor_height", "assumed")]


def test_i_dont_know_about_the_scope_assumes_the_rooms_empty():
    s, _ = hear(ask.UNKNOWN_OPTION, state=SessionState(pending=["scope"]), summary=NO_FIXTURES)
    assert {sc.state for sc in s.scope.values()} == {"empty"} and {sc.source for sc in s.scope.values()} == {"assumed"}


def test_i_dont_know_assumes_no_fixture_exists_and_says_so():
    state = full_state()
    state.pending = ["fixtures"]
    s, _ = hear(ask.UNKNOWN_OPTION, state=state)
    assert s.facts["fixtures"].source == "assumed" and s.facts["fixtures"].value == "none exist" and s.fixtures_existing == []


def test_the_chips_for_the_fixtures_question_are_understood():
    state = full_state()
    state.pending = ["fixtures"]
    none, _ = hear("None of them exist", state=state)
    assert none.facts["fixtures"].value == "none exist" and none.facts["fixtures"].source == "user" and none.fixtures_existing == []
    everything, _ = hear("All of them already exist", state=state)
    assert everything.facts["fixtures"].value == "all exist" and everything.fixtures_existing == ["f1", "f2", "f3"]  # priced ones only


def test_some_fixtures_exist_only_those_the_message_refers_to():
    state = full_state()
    state.pending = ["fixtures"]
    # the model also claims the kitchen counter, which the person never mentioned
    s, _ = hear("The toilet in the bath is already installed", {"fixtures_existing": ["f1", "f3"]}, state=state)
    assert s.fixtures_existing == ["f1"] and s.facts["fixtures"].value == "some exist"


def test_a_fixture_the_plan_does_not_price_is_never_recorded_as_existing():
    state = full_state()
    state.pending = ["fixtures"]
    s, _ = hear("The bed is already installed", {"fixtures_existing": ["f4"]}, state=state)
    assert s.fixtures_existing == []


def test_a_new_build_always_includes_structure_so_it_is_not_kept_or_asked():
    s, _ = hear("A new build", {"work_type": "new_build"}, state=full_state(structure_in_scope=False))
    assert s.facts["work_type"].value == "new_build" and "structure_in_scope" not in s.facts


def test_the_same_scope_twice_is_one_change_per_room():
    ex = Extracted(scope_default="empty", scope_rooms=[RoomStateClaim(r.id, "empty") for r in ROOMS])
    _, changes = interview().record(CTX, SessionState(), SUMMARY, ex, "everything is empty")
    assert len([c for c in changes if c.name.startswith("scope.")]) == len(ROOMS)


def test_a_message_that_changes_nothing_reports_no_change_and_the_given_state_is_untouched():
    state = full_state()
    before = (dict(state.facts), dict(state.scope), list(state.excluded))
    s, changes = hear("make the kitchen bigger", state=state)
    assert changes == [] and (s.facts, s.scope, s.excluded) == before and (state.facts, state.scope, state.excluded) == before
    s2, changes2 = hear("Fit-out please", {"work_type": "fit_out"}, state=state)
    assert changes2 == [] and s2 is not state  # already known: not a change


def test_a_users_own_percentages_are_not_overwritten_by_the_typical_ones():
    state = full_state(vat_percent=10)
    state.pending = ["markups"]
    s, _ = hear("use typical rates", state=state)
    assert s.facts["vat_percent"].value == 10 and s.facts["vat_percent"].source == "user"


# --- what the Engine reads and when an estimate is stale -----------------------------------------------------------------------

def test_the_engine_reads_the_facts_scope_and_exclusions_of_the_session():
    state = full_state()
    state.excluded, state.fixtures_existing = ["window"], ["f1"]
    facts = interview().facts(state)
    assert facts.facts is state.facts and facts.scope is state.scope and facts.excluded == ["window"] and facts.fixtures_existing == ["f1"]


def test_the_fingerprint_changes_with_the_inputs_of_the_estimate_and_not_with_the_conversation():
    iv, state = interview(), full_state()
    same = full_state()
    same.pending, same.turns = ["quality"], 7
    assert iv.fingerprint(state) == iv.fingerprint(same)
    other = full_state(quality="luxury")
    excluded = full_state()
    excluded.excluded = ["window"]
    overridden = full_state()
    overridden.rate_overrides = {"paint": 1000.0}
    assert len({iv.fingerprint(x) for x in (state, other, excluded, overridden)}) == 4
