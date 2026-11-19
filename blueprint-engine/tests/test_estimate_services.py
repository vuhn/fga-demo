"""Unit tests for the estimate services (T5): takeoff, pricing and the plan summary take EstimateFacts directly."""

import pytest

import sample_dxf
from fga_engine.domain.estimate import EstimateFacts, Fact, RateResult, RoomScope
from fga_engine.domain.rate_catalog import RATE_CATALOG
from fga_engine.domain.schemas import PlacedFixture
from fga_engine.dxf_plan import import_dxf
from fga_engine.estimate_models import SessionState, facts_of
from fga_engine.wire import build_engine


@pytest.fixture(scope="module")
def plan():
    data, _ = sample_dxf.sheet()
    return import_dxf(data).plans[0].model


@pytest.fixture
def fxplan(plan):
    bath = next(r for r in plan.rooms if r.name == "Bath")
    liv = next(r for r in plan.rooms if r.type == "living")
    bed = next(r for r in plan.rooms if r.name == "Bedroom 1")
    fx = [PlacedFixture(id="f1", kind="toilet", room_id=bath.id, rect=(0, 0, 0.6, 0.4), facing="south"),
          PlacedFixture(id="f3", kind="kitchen_counter", room_id=liv.id, rect=(0, 0, 2.4, 0.6), facing="south"),
          PlacedFixture(id="f4", kind="bed", room_id=bed.id, rect=(0, 0, 2, 1.6), facing="south")]
    return plan.model_copy(update={"fixtures": fx})


def facts(plan, **over) -> EstimateFacts:
    f = {"kind": "apartment", "work_type": "fit_out", "floor_height": 3.0, "mep_in_scope": False}
    f.update(over)
    return EstimateFacts(facts={k: Fact(value=v, source="user") for k, v in f.items()},
                         scope={r.id: RoomScope(state="empty", source="user") for r in plan.rooms})


def test_summarize_lists_rooms_fixtures_and_openings(fxplan):
    s = build_engine().plan_summarizer.summarize_plan(fxplan, EstimateFacts())
    assert [r.id for r in s.rooms] == [r.id for r in fxplan.rooms] and all(r.area_m2 > 0 for r in s.rooms)
    assert [(f.id, f.kind) for f in s.fixtures] == [("f1", "toilet"), ("f3", "kitchen_counter"), ("f4", "bed")]
    assert len(s.openings) == len(fxplan.openings) and {o.kind for o in s.openings} <= {"door", "window", "open"}


def test_summarize_pending_fixtures_are_priced_unknown_ones_only(fxplan):
    svc = build_engine().plan_summarizer
    assert svc.summarize_plan(fxplan, EstimateFacts()).pending_fixtures == ["f1", "f3"]  # a bed is loose furniture
    said = EstimateFacts(fixtures_existing=["f1"])
    assert svc.summarize_plan(fxplan, said).pending_fixtures == ["f3"]
    bath = next(r for r in fxplan.rooms if r.name == "Bath")
    finished = EstimateFacts(scope={bath.id: RoomScope(state="existing", source="user")})
    assert svc.summarize_plan(fxplan, finished).pending_fixtures == ["f3"]


def test_summarize_structure_in_scope_follows_the_facts(plan):
    svc = build_engine().plan_summarizer
    assert svc.summarize_plan(plan, EstimateFacts()).structure_in_scope is False
    new_build = EstimateFacts(facts={"work_type": Fact(value="new_build", source="user")})
    assert svc.summarize_plan(plan, new_build).structure_in_scope is True
    reno = EstimateFacts(facts={"work_type": Fact(value="renovation", source="user"),
                                "structure_in_scope": Fact(value=True, source="assumed")})
    assert svc.summarize_plan(plan, reno).structure_in_scope is True
    reno_no = EstimateFacts(facts={"work_type": Fact(value="renovation", source="user"),
                                   "structure_in_scope": Fact(value=False, source="user")})
    assert svc.summarize_plan(plan, reno_no).structure_in_scope is False


def test_summarize_an_empty_plan_is_empty(plan):
    empty = plan.model_copy(update={"rooms": [], "fixtures": [], "openings": []})
    s = build_engine().plan_summarizer.summarize_plan(empty, EstimateFacts())
    assert s.rooms == [] and s.pending_fixtures == [] and s.openings == []


def test_takeoff_and_price_take_facts_and_match_the_legacy_adapter(plan):
    from fga_engine.estimate import price
    from fga_engine.takeoff import takeoff as legacy_takeoff

    eng = build_engine()
    f = facts(plan)
    tk = eng.takeoff.takeoff(plan, f)
    st = SessionState(facts=f.facts, scope=f.scope)
    assert facts_of(st) == f
    assert [ln.model_dump() for ln in legacy_takeoff(plan, st).lines] == [ln.model_dump() for ln in tk.lines]
    rates = {c: RateResult(code=c, low=900, base=1000, high=1200, unit=RATE_CATALOG[c].unit, confidence="ok") for c in tk.codes()}
    est = eng.estimate_pricer.price_estimate(tk, rates, f)
    assert est.model_dump() == price(tk, rates, st).model_dump() and est.total["base"] > 0


def test_takeoff_honours_excluded_codes_and_the_source_of_a_fact(plan):
    eng = build_engine()
    f = facts(plan)
    f.excluded = ["window"]
    f.facts["floor_height"] = Fact(value=3.0, source="assumed")
    tk = eng.takeoff.takeoff(plan, f)
    assert "window" not in tk.codes() and any("Left out" in w for w in tk.warnings)
    assert next(ln for ln in tk.lines if ln.code == "plaster").confidence == "assumed"


def test_price_uses_markup_facts_and_defaults(plan):
    eng = build_engine()
    f = facts(plan, vat_percent=10)
    tk = eng.takeoff.takeoff(plan, f)
    rates = {c: RateResult(code=c, base=1000, unit="x", confidence="ok") for c in tk.codes()}
    est = eng.estimate_pricer.price_estimate(tk, rates, f)
    vat = est.markups[-1]
    assert vat.name == "VAT" and vat.percent == 10
    assert next(m for m in est.markups if m.name == "Contingency").percent == 7.5  # default, no assumed lines
