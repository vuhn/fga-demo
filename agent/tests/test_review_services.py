"""PlanReviser, BriefReviewer and the automatic review round after a revise: ported from the Engine-era reviewer and
plan-chat cases, against a scripted language model and a scripted Engine."""

import json

import pytest
from fake_language_model import FakeLanguageModel
from scripted_engine import BRIEF, PLAN, ScriptedEngine, build, check, revision

from fga_agent.domain.errors import EngineUnavailable, InvalidEdit, UnsupportedEdit
from fga_agent.domain.model.call_context import CallContext
from fga_agent.service.brief_reviewer import BriefReviewerService
from fga_agent.service.brief_reviser import BriefReviserService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.feedback_asker import FeedbackAskerService
from fga_agent.service.plan_reviser import PlanReviserService

CTX = CallContext(request_id="t")
RENAME = {"ops": [{"op": "rename_room", "room": "Kitchen", "name": "Library"}]}
FIX = {"ops": [{"op": "set_side", "room": "Kitchen", "side": None}]}


class Steps:
    def __init__(self):
        self.seen = []

    def __call__(self, p):
        self.seen.append((p.kind, p.stage, p.message))

    @property
    def stages(self):
        return [s for k, s, _ in self.seen if k == "step"]


def parts(llm, engine):
    catalog, asker = DesignCatalogLoaderService(engine), FeedbackAskerService(llm)
    return catalog, asker, BriefReviewerService(engine, catalog, asker)


def plan_reviser(llm, engine):
    catalog, asker, _ = parts(llm, engine)
    return PlanReviserService(engine, catalog, asker)


def reviewer(llm, engine):
    return parts(llm, engine)[2]


def reviser(llm, engine):
    catalog, asker, rev = parts(llm, engine)
    return BriefReviserService(engine, catalog, asker, rev)


def review(svc, rounds=3, steps=None):
    return svc.review(CTX, BRIEF, b"{}", ["earlier"], rounds, steps or Steps())


BAD = dict(model=b"{}", ok=False, issue_score=20, error_rules=["side"], checks=[check("side")])
BETTER = dict(issue_score=0, error_rules=[], checks=[], ok=True)


# --- plan chat ------------------------------------------------------------------------------------------------

def test_a_plan_chat_applies_the_ops_through_the_engine_and_reports_what_was_done():
    llm, engine = FakeLanguageModel([RENAME]), ScriptedEngine(plans=[revision(descriptions=["rename room Kitchen to Library"])])
    steps = Steps()
    out = plan_reviser(llm, engine).revise(CTX, PLAN, "make it a library", ["earlier"], steps)
    assert json.loads(engine.plan_ops[0][1]) == RENAME and engine.plan_ops[0][0] == PLAN
    assert out.brief == b"" and out.model == PLAN and out.ok and out.llm_attempts == 1
    assert out.notes == ["applied: rename room Kitchen to Library"] and out.history == ["rename room Kitchen to Library"]
    assert steps.stages == ["ask_model", "ops", "checks"]
    user, system = llm.calls[0]["user"], llm.calls[0]["system"]
    assert "Kitchen" in user and "sink" in user and "Earlier requests" in user and "make it a library" in user
    assert "move_fixture" in system and "set_area" not in system  # only the plan operations of the catalog


def test_a_plan_chat_asks_for_geometry_changes_to_be_refused_and_does_not_retry():
    llm, engine = FakeLanguageModel([{"ops": [{"op": "unsupported", "reason": "moving walls"}]}]), ScriptedEngine(
        plans=[UnsupportedEdit("not supported for an imported plan: moving walls")])
    with pytest.raises(UnsupportedEdit, match="moving walls"):
        plan_reviser(llm, engine).revise(CTX, PLAN, "move the wall", [], Steps())
    assert len(llm.calls) == 1


def test_a_plan_chat_retries_an_unknown_name_with_the_error_and_says_so():
    llm = FakeLanguageModel([{"ops": [{"op": "rename_room", "room": "Nonexistent", "name": "x"}]}, RENAME])
    engine = ScriptedEngine(plans=[InvalidEdit("no room called Nonexistent"), revision()])
    out = plan_reviser(llm, engine).revise(CTX, PLAN, "rename", [], Steps())
    assert out.llm_attempts == 2 and "no room called Nonexistent" in llm.calls[1]["user"]
    assert out.notes[1] == "the first 1 answer(s) could not be applied; corrected with the error"


def test_a_plan_chat_with_an_empty_message_or_a_down_engine_is_refused_before_any_step():
    llm, steps = FakeLanguageModel([RENAME]), Steps()
    with pytest.raises(InvalidEdit, match="say what to change"):
        plan_reviser(llm, ScriptedEngine()).revise(CTX, PLAN, "  ", [], steps)
    engine = ScriptedEngine()
    engine.down = True
    with pytest.raises(EngineUnavailable):
        plan_reviser(llm, engine).revise(CTX, PLAN, "rename", [], steps)
    assert steps.seen == [] and llm.calls == []


# --- review ---------------------------------------------------------------------------------------------------

def test_the_reviewer_keeps_an_improving_round():
    llm = FakeLanguageModel([FIX])
    engine = ScriptedEngine(generates=[build(**BAD)], builds=[build(**BETTER)])
    steps = Steps()
    out = review(reviewer(llm, engine), 3, steps)
    assert out.ok and out.checks == [] and out.llm_attempts == 1
    assert out.history == ["round 1: set side room=Kitchen (score 20 -> 0, kept)"]
    assert out.notes[0].startswith("review round 1: set side") and "issues 1 -> 0" in out.notes[0]
    assert steps.stages == ["review", "ask_model", "review"]
    assert "[error] side:kitchen: side problem (hint: set_side {room, side})" in llm.calls[0]["user"]
    assert len(engine.applied) == 1 and engine.applied[0][2] == b"{}"


def test_the_reviewer_stops_when_the_plan_has_no_issues_and_does_not_call_the_model():
    llm = FakeLanguageModel()
    out = review(reviewer(llm, ScriptedEngine(generates=[build()])))
    assert llm.calls == [] and out.notes == [] and out.history == [] and out.ok


def test_the_reviewer_refuses_forbidden_operations_and_never_returns_a_worse_plan():
    forbidden = {"ops": [{"op": "remove_room", "room": "Kitchen"}]}
    llm, engine = FakeLanguageModel([forbidden] * 3), ScriptedEngine(generates=[build(**BAD)])
    out = review(reviewer(llm, engine))
    assert len(llm.calls) == 3 and "may not use remove_room" in llm.calls[1]["user"]
    assert any("no usable fix" in n for n in out.notes) and "1 error(s) remain for you to decide" in out.notes
    assert engine.applied == [] and out.llm_attempts == 3 and not out.ok  # the plan given, unchanged
    assert "remove_room" not in llm.calls[0]["system"] and "set_area" in llm.calls[0]["system"]


def test_the_reviewer_discards_a_round_that_does_not_help_and_stops():
    llm = FakeLanguageModel([{"ops": [{"op": "add_fixture", "room": "Living", "fixture": "desk"}]}])
    engine = ScriptedEngine(generates=[build(**BAD)], builds=[build(**{**BAD, "issue_score": 20})])
    out = review(reviewer(llm, engine))
    assert len(llm.calls) == 1 and out.history[0].endswith("(score 20 -> 20, discarded)")
    assert "no improvement" in out.notes[0] and not out.ok


def test_the_reviewer_discards_a_round_that_adds_a_new_kind_of_error():
    worse_kind = build(**{**BETTER, "issue_score": 10, "error_rules": ["side", "door"], "ok": False})
    llm, engine = FakeLanguageModel([FIX]), ScriptedEngine(generates=[build(**BAD)], builds=[worse_kind])
    out = review(reviewer(llm, engine))
    assert "new errors ['door']" in out.notes[0] and "discarded" in out.history[0]


def test_the_reviewer_caps_the_total_area_change():
    llm = FakeLanguageModel([{"ops": [{"op": "set_area", "room": "Living", "area": 80}]}] * 3)
    engine = ScriptedEngine(generates=[build(**BAD)], builds=[build(total_target_area=80.0)] * 3)
    out = review(reviewer(llm, engine))
    assert "stay within 15%" in llm.calls[1]["user"] and any("no usable fix" in n for n in out.notes)


def test_the_reviewer_retries_an_answer_that_changes_nothing():
    llm = FakeLanguageModel([FIX, FIX])
    engine = ScriptedEngine(generates=[build(**BAD)], builds=[build(changed=False), build(**BETTER)])
    out = review(reviewer(llm, engine), 1)
    assert "change nothing" in llm.calls[1]["user"] and out.llm_attempts == 2 and out.ok


def test_the_reviewer_runs_at_most_three_rounds_and_remembers_what_earlier_rounds_did():
    still_bad = lambda score: build(**{**BAD, "issue_score": score, "checks": [check("side")]})  # noqa: E731
    llm = FakeLanguageModel([FIX] * 5)
    engine = ScriptedEngine(generates=[build(**{**BAD, "issue_score": 40})], builds=[still_bad(30), still_bad(20), still_bad(10)])
    out = review(reviewer(llm, engine), 99)
    assert len(llm.calls) == 3 and len(out.history) == 3
    assert "round 1: set side room=Kitchen -> accepted" in llm.calls[1]["user"]


def test_an_unbuildable_brief_is_reviewed_by_its_issues():
    broken = build(model=b"", ok=False, issues=["rooms do not fit the lot"], issue_score=1010, checks=[])
    llm = FakeLanguageModel([FIX])
    out = review(reviewer(llm, ScriptedEngine(generates=[broken], builds=[build(**BETTER)])))
    assert "rooms do not fit the lot" in llm.calls[0]["user"] and out.ok


# --- the automatic review round after a revise ---------------------------------------------------------------------

OPS = {"ops": [{"op": "set_area", "room": "kitchen", "area": 12}]}


def test_an_edit_that_introduces_errors_gets_one_review_round():
    llm = FakeLanguageModel([OPS, FIX])
    broken = build(**BAD)
    engine = ScriptedEngine(builds=[broken, build(**BETTER)], generates=[build(), build(**BAD)])
    steps = Steps()
    out = reviser(llm, engine).revise(CTX, BRIEF, b"{}", "make the kitchen bigger", ["earlier"], steps)
    assert out.ok and out.llm_attempts == 2
    assert steps.stages == ["ask_model", "ops", "layout", "checks", "review", "review", "ask_model", "review"]
    assert out.notes[0].startswith("applied: ") and any(n.startswith("review round 1") for n in out.notes)
    assert out.history[0] == "set area room=kitchen, area=12" and out.history[1].startswith("round 1:")
    assert "make the kitchen bigger" in llm.calls[1]["user"]  # the request joins the history the reviewer sees


def test_an_edit_that_fails_only_rules_the_plan_already_failed_is_not_reviewed():
    llm = FakeLanguageModel([OPS])
    engine = ScriptedEngine(builds=[build(**BAD)], generates=[build(**BAD)])
    out = reviser(llm, engine).revise(CTX, BRIEF, b"{}", "bigger", [], Steps())
    assert len(llm.calls) == 1 and out.llm_attempts == 1 and not out.ok


def test_a_clean_edit_does_not_ask_the_engine_for_the_plan_before():
    engine = ScriptedEngine(builds=[build()])
    reviser(FakeLanguageModel([OPS]), engine).revise(CTX, BRIEF, b"{}", "bigger", [], Steps())
    assert engine.generated == []
