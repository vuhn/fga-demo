"""BriefDrafter and BriefReviser: ported from the Engine-era llm_steps cases, against a scripted language model and a scripted Engine."""

import json

import pytest
from fake_language_model import FakeLanguageModel
from scripted_engine import BRIEF, ScriptedEngine, build

from fga_agent.domain.errors import Cancelled, EngineUnavailable, InvalidEdit, LlmUnavailable, UnsupportedEdit
from fga_agent.domain.model.brief import BriefDraft
from fga_agent.domain.model.call_context import CallContext
from fga_agent.service.brief_drafter import BriefDrafterService
from fga_agent.service.brief_reviewer import BriefReviewerService
from fga_agent.service.brief_reviser import BriefReviserService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.feedback_asker import FeedbackAskerService

CTX = CallContext(request_id="t")
DRAFT = {"title": "Cottage", "lot_width": 15, "lot_depth": 25, "rooms": [
    {"name": "Living", "type": "living", "target_area": 22, "side": "north"},
    {"name": "Kitchen", "type": "kitchen", "target_area": 9, "side": "east"}]}
OPS = {"ops": [{"op": "set_area", "room": "kitchen", "area": 12}]}


class Steps:
    def __init__(self):
        self.seen = []

    def __call__(self, p):
        self.seen.append((p.kind, p.stage))

    @property
    def stages(self):
        return [s for k, s in self.seen if k == "step"]


def drafter(llm, engine):
    return BriefDrafterService(engine, llm, DesignCatalogLoaderService(engine), FeedbackAskerService(llm))


def reviser(llm, engine):
    catalog, asker = DesignCatalogLoaderService(engine), FeedbackAskerService(llm)
    return BriefReviserService(engine, catalog, asker, BriefReviewerService(engine, catalog, asker))


def revise(svc, message="make the kitchen bigger", history=(), steps=None):
    return svc.revise(CTX, BRIEF, b"{}", message, list(history), steps or Steps())


# --- drafting -------------------------------------------------------------------------------------------------

def test_drafting_hands_the_model_output_to_the_engine_and_returns_its_brief_and_notes():
    engine = ScriptedEngine(drafts=[BriefDraft(b"BRIEF", ["dropped Garage"])])
    out = drafter(FakeLanguageModel([DRAFT]), engine).draft(CTX, "small cottage", False, Steps())
    assert (out.brief, out.notes) == (b"BRIEF", ["dropped Garage"])


def test_drafting_drops_sides_the_person_never_named():
    engine = ScriptedEngine(drafts=[BriefDraft(b"B", [])])
    drafter(FakeLanguageModel([DRAFT]), engine).draft(CTX, "a cottage with the living room on the north", False, Steps())
    rooms = json.loads(engine.normalized[0])["rooms"]
    assert [r["side"] for r in rooms] == ["north", None]


def test_the_enhancer_is_off_by_default():
    llm = FakeLanguageModel([DRAFT])
    drafter(llm, ScriptedEngine(drafts=[BriefDraft(b"B", [])])).draft(CTX, "cottage", False, Steps())
    assert len(llm.calls) == 1 and "Requirements from the architect" not in llm.calls[0]["user"]


def test_enhancer_requirements_feed_the_draft_and_assumptions_become_notes():
    llm = FakeLanguageModel([{"reasoning": "r", "requirements": ["Living 22 m²", "Lot 20 x 30"],
                              "assumptions": ["lot size not given"]}, DRAFT])
    out = drafter(llm, ScriptedEngine(drafts=[BriefDraft(b"B", ["n"])])).draft(CTX, "cottage", True, Steps())
    assert len(llm.calls) == 2
    assert "- Living 22 m²" in llm.calls[1]["user"] and "- Lot 20 x 30" in llm.calls[1]["user"]
    assert out.notes == ["assumed: lot size not given", "n"]


def test_a_draft_the_engine_refuses_is_sent_back_with_the_error_and_tried_again():
    llm = FakeLanguageModel([DRAFT, DRAFT])
    steps = Steps()
    out = drafter(llm, ScriptedEngine(drafts=[InvalidEdit("unknown adjacent room"), BriefDraft(b"B", [])])).draft(
        CTX, "cottage", False, steps)
    assert out.brief == b"B"
    assert "unknown adjacent room" in llm.calls[1]["user"] and "Your previous answer was" in llm.calls[1]["user"]
    assert steps.stages == ["ask_model", "retry", "ask_model"]


def test_drafting_gives_up_after_three_attempts():
    llm = FakeLanguageModel([DRAFT] * 3)
    with pytest.raises(InvalidEdit, match="after 3 attempts"):
        drafter(llm, ScriptedEngine(drafts=[InvalidEdit("bad")] * 3)).draft(CTX, "cottage", False, Steps())
    assert len(llm.calls) == 3


def test_an_empty_description_is_refused_before_the_model_is_asked():
    llm = FakeLanguageModel([DRAFT])
    with pytest.raises(InvalidEdit, match="describe the house"):
        drafter(llm, ScriptedEngine()).draft(CTX, "   ", False, Steps())
    assert llm.calls == []


def test_the_description_is_cut_at_4000_characters():
    llm = FakeLanguageModel([DRAFT])
    drafter(llm, ScriptedEngine(drafts=[BriefDraft(b"B", [])])).draft(CTX, "x" * 5000, False, Steps())
    assert llm.calls[0]["user"] == "Description: " + "x" * 4000


def test_the_prompts_are_built_from_the_engines_catalog_and_limits():
    llm = FakeLanguageModel([DRAFT])
    drafter(llm, ScriptedEngine(drafts=[BriefDraft(b"B", [])])).draft(CTX, "cottage", False, Steps())
    system = llm.calls[0]["system"]
    assert "Room types: bedroom, bathroom." in system and "Fixtures (sink, toilet)" in system
    assert "only bathroom\nrooms can be inside another room" in system


# --- revising -------------------------------------------------------------------------------------------------

def test_revising_applies_the_ops_through_the_engine_and_reports_what_was_done():
    engine = ScriptedEngine(builds=[build(notes=["fit: ok"], checks_line="1 warning")])
    steps = Steps()
    out = revise(reviser(FakeLanguageModel([OPS]), engine), steps=steps)
    assert engine.applied == [(BRIEF, json.dumps(OPS).encode(), b"{}")]
    assert (out.brief, out.model, out.ok, out.llm_attempts) == (BRIEF, b"{}", True, 1)
    assert out.history == ["set area room=kitchen, area=12"]
    assert out.notes == ["applied: set area room=kitchen, area=12", "fit: ok"]
    assert steps.stages == ["ask_model", "ops", "layout", "checks"]


def test_revising_retries_with_the_error_and_says_so():
    llm = FakeLanguageModel([{"ops": [{"op": "set_area", "room": "garage", "area": 5}]}, OPS])
    engine = ScriptedEngine(builds=[InvalidEdit("unknown room 'garage'"), build()])
    out = revise(reviser(llm, engine))
    assert out.llm_attempts == 2 and "the first 1 answer(s) could not be applied; corrected with the error" in out.notes
    assert "unknown room 'garage'" in llm.calls[1]["user"] and llm.calls[1]["user"].startswith(llm.calls[0]["user"])


def test_revising_gives_up_after_three_attempts():
    llm = FakeLanguageModel([OPS] * 3)
    with pytest.raises(InvalidEdit, match="after 3 attempts"):
        revise(reviser(llm, ScriptedEngine(builds=[InvalidEdit("nope")] * 3)))
    assert len(llm.calls) == 3


def test_an_unsupported_request_is_not_retried():
    llm = FakeLanguageModel([{"ops": [{"op": "unsupported", "reason": "two storeys"}]}] * 3)
    with pytest.raises(UnsupportedEdit):
        revise(reviser(llm, ScriptedEngine(builds=[UnsupportedEdit("two storeys")] * 3)))
    assert len(llm.calls) == 1


def test_the_history_and_the_current_brief_reach_the_prompt():
    llm = FakeLanguageModel([OPS])
    revise(reviser(llm, ScriptedEngine(builds=[build()])), "a bit more", ["make the kitchen bigger", "  ", "add a study"])
    user = llm.calls[0]["user"]
    assert "Earlier requests on this plan, oldest first" in user
    assert "- make the kitchen bigger\n- add a study\n" in user
    assert 'Request: a bit more' in user
    assert '"adjacent_to": ["Kitchen"]' in user and '"inside": "Kitchen"' in user  # rooms by name, not by id


def test_only_the_last_six_earlier_requests_are_shown():
    llm = FakeLanguageModel([OPS])
    revise(reviser(llm, ScriptedEngine(builds=[build()])), history=[f"request {i}" for i in range(10)])
    user = llm.calls[0]["user"]
    assert "request 3\n" not in user and "request 4\n" in user and "request 9\n" in user


def test_the_system_prompt_lists_the_briefs_operations_only():
    llm = FakeLanguageModel([OPS])
    revise(reviser(llm, ScriptedEngine(builds=[build()])))
    system = llm.calls[0]["system"]
    assert "- set_area {room, area}: set a room's target area" in system and "move_fixture" not in system


def test_an_empty_message_is_refused_before_anything_is_asked_or_reported():
    llm, steps = FakeLanguageModel([OPS]), Steps()
    with pytest.raises(InvalidEdit, match="say what to change"):
        revise(reviser(llm, ScriptedEngine()), "  ", steps=steps)
    assert llm.calls == [] and steps.seen == []


def test_a_down_engine_refuses_before_the_first_step():
    engine, steps = ScriptedEngine(), Steps()
    engine.down = True
    with pytest.raises(EngineUnavailable):
        revise(reviser(FakeLanguageModel([OPS]), engine), steps=steps)
    assert steps.seen == []


def test_an_unreachable_model_is_reported_as_it_is():
    with pytest.raises(LlmUnavailable):
        revise(reviser(FakeLanguageModel([LlmUnavailable("ollama down")]), ScriptedEngine()))


def test_a_cancelled_run_stops_at_its_next_step():
    llm, engine = FakeLanguageModel([OPS, OPS]), ScriptedEngine(builds=[InvalidEdit("bad"), build()])

    def cancel_on_retry(p):
        if p.stage == "retry":
            raise Cancelled()

    with pytest.raises(Cancelled):
        revise(reviser(llm, engine), steps=cancel_on_retry)
    assert len(llm.calls) == 1 and len(engine.applied) == 1
