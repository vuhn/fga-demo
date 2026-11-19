"""The Engine's takeoff and pricing through the legacy session adapters (`estimate.price`, `takeoff.takeoff`).
The interview, the price research and the chat live in the Agent (agent/tests)."""

from __future__ import annotations

import math

import pytest
from shapely.geometry import Polygon

import sample_dxf
from fga_engine.domain.schemas import PlacedFixture
from fga_engine.estimate import price
from fga_engine.estimate_models import Fact, RateResult, RoomScope, SessionState
from fga_engine.infrastructure.dxf_plan import import_dxf
from fga_engine.takeoff import RATE_CATALOG, takeoff


@pytest.fixture(scope="module")
def plan():
    data, _ = sample_dxf.sheet()
    return import_dxf(data).plans[0].model  # the first apartment




def facts(**kw):
    return {k: Fact(value=v, source="user") for k, v in kw.items()}


def full_state(plan, **over):
    f = facts(kind="apartment", work_type="fit_out", city="Ho Chi Minh City", quality="normal", floor_height=3.0,
              mep_in_scope=False, vat_percent=8, overhead_percent=6.5, profit_percent=7.5, design_percent=5.5, contingency_percent=7.5)
    f.update({k: Fact(value=v, source="user") for k, v in over.items()})
    return SessionState(facts=f, scope={r.id: RoomScope(state="empty", source="user") for r in plan.rooms})


# ---------- takeoff ----------


def test_takeoff_areas_and_walls_are_hand_checkable(plan):
    st = full_state(plan)
    tk = takeoff(plan, st)
    by = {ln.code: ln for ln in tk.lines}
    area = sum(r.net_area for r in plan.rooms)
    assert by["floor_tile"].quantity == pytest.approx(area, abs=0.01)
    assert by["ceiling"].quantity == pytest.approx(area, abs=0.01)
    perim = sum(Polygon(r.net_polygon()).length for r in plan.rooms)
    opened = sum(o.width * (1.5 if o.kind == "window" else 2.1) * sum(1 for x in o.rooms if x != "outside") for o in plan.openings)
    assert by["plaster"].quantity == pytest.approx(perim * 3.0 - opened, rel=0.01)
    assert "brick_wall_100" not in by  # empty rooms: walls exist, only finished
    assert "concrete" not in by and "electrical_point" not in by  # structure and MEP are out of scope
    assert by["floor_tile"].confidence == "measured"
    assert by["plaster"].confidence == "derived"
    assert sum(1 for o in plan.openings if o.kind == "window") > 0 and by["window"].unit == "m2"


def test_existing_rooms_are_not_built_and_replaced_rooms_get_demolition(plan):
    wc = next(r for r in plan.rooms if r.type == "bathroom")
    st = full_state(plan)
    st.scope[wc.id] = RoomScope(state="existing", source="user")
    with_all = {ln.code: ln.quantity for ln in takeoff(plan, full_state(plan)).lines}
    without = {ln.code: ln.quantity for ln in takeoff(plan, st).lines}
    assert without["floor_tile"] == pytest.approx(with_all["floor_tile"] - wc.net_area, abs=0.01)
    assert without.get("waterproofing", 0) < with_all["waterproofing"]
    st.scope[wc.id] = RoomScope(state="to_replace", source="user")
    assert "demolition_finish" in {ln.code for ln in takeoff(plan, st).lines}


def test_assumed_height_makes_dependent_lines_assumed(plan):
    st = full_state(plan)
    st.facts["floor_height"] = Fact(value=3.0, source="assumed")
    by = {ln.code: ln for ln in takeoff(plan, st).lines}
    assert by["plaster"].confidence == "assumed" and by["floor_tile"].confidence == "measured"


def test_new_build_adds_structure_and_mep_points(plan):
    st = full_state(plan, work_type="new_build", storeys=2, mep_in_scope=True, kind="house")
    by = {ln.code: ln for ln in takeoff(plan, st).lines}
    for code in ("concrete", "rebar", "formwork", "excavation", "roof_tile", "electrical_point"):
        assert code in by, code
    assert by["rebar"].quantity == pytest.approx(by["concrete"].quantity * 120, rel=0.001)
    assert by["formwork"].quantity == pytest.approx(by["concrete"].quantity * 7, rel=0.001)
    assert by["concrete"].confidence == "assumed"


def test_a_drawing_with_no_rooms_is_unsupported_not_zero(plan):
    empty = plan.model_copy(update={"rooms": [], "walls": [], "openings": [], "fixtures": []})
    tk = takeoff(empty, full_state(plan))
    assert tk.status == "unsupported" and tk.lines == [] and tk.warnings


# ---------- pricing ----------


def test_cascade_to_the_dong(plan):
    st = full_state(plan)
    tk = takeoff(plan, st)
    rates = {c: RateResult(code=c, low=900, base=1000, high=1200, unit=RATE_CATALOG[c].unit, confidence="ok") for c in tk.codes()}
    est = price(tk, rates, st)
    direct = sum(ln.quantity * 1000 for ln in tk.lines)
    assert est.direct["base"] == pytest.approx(direct, abs=1)
    ov = direct * 0.065
    pf = (direct + ov) * 0.075
    ds = (direct + ov + pf) * 0.055
    ct = (direct + ov + pf + ds) * 0.075  # no assumed lines: contingency stays at 7.5%
    vat = (direct + ov + pf + ds + ct) * 0.08
    assert est.total["base"] == pytest.approx(direct + ov + pf + ds + ct + vat, abs=3)
    assert est.total["low"] < est.total["base"] < est.total["high"]
    assert [m.name for m in est.markups][-1] == "VAT"


def test_contingency_rises_with_assumed_share_and_unpriced_lines_stay_out(plan):
    st = full_state(plan)
    st.facts["floor_height"] = Fact(value=3.0, source="assumed")
    tk = takeoff(plan, st)
    rates = {c: RateResult(code=c, low=1, base=1000, high=1, unit="x", confidence="ok") for c in tk.codes() if c != "paint"}
    est = price(tk, rates, st)
    cont = next(m for m in est.markups if m.name == "Contingency")
    assert cont.percent > 7.5 and est.assumed_share > 0
    paint = next(ln for ln in est.lines if ln.code == "paint")
    assert not paint.priced and paint.base == 0 and any("paint" in w.lower() for w in est.warnings)
    assert est.direct["base"] == pytest.approx(sum(ln.base for ln in est.lines))


# ---------- price research checks ----------



@pytest.fixture
def fxplan(plan):
    bath = next(r for r in plan.rooms if r.name == "Bath")
    liv = next(r for r in plan.rooms if r.type == "living")
    bed = next(r for r in plan.rooms if r.name == "Bedroom 1")
    rc = (0, 0, 0.6, 0.4)
    fx = [PlacedFixture(id="f1", kind="toilet", room_id=bath.id, rect=rc, facing="south"),
          PlacedFixture(id="f2", kind="basin", room_id=bath.id, rect=rc, facing="south"),
          PlacedFixture(id="f3", kind="kitchen_counter", room_id=liv.id, rect=(0, 0, 2.4, 0.6), facing="south"),
          PlacedFixture(id="f4", kind="bed", room_id=bed.id, rect=(0, 0, 2, 1.6), facing="south")]
    return plan.model_copy(update={"fixtures": fx})


def test_fixtures_are_assumed_absent_and_priced_unless_they_exist(fxplan):
    st = full_state(fxplan)
    by = {ln.code: ln for ln in takeoff(fxplan, st).lines}
    assert by["fixture_toilet"].quantity == 1 and by["fixture_basin"].quantity == 1
    assert by["fixture_kitchen_counter"].quantity == pytest.approx(2.4) and by["fixture_kitchen_counter"].unit == "m"
    assert "fixture_bed" not in by and not any("bed" in c for c in by)  # loose furniture is not priced
    assert any("bed" in w for w in takeoff(fxplan, st).warnings)
    st.fixtures_existing = ["f1"]
    assert "fixture_toilet" not in {ln.code for ln in takeoff(fxplan, st).lines}
    wc = next(r for r in fxplan.rooms if r.name == "Bath")
    st.scope[wc.id] = RoomScope(state="existing", source="user")
    assert not {"fixture_toilet", "fixture_basin"} & {ln.code for ln in takeoff(fxplan, st).lines}


