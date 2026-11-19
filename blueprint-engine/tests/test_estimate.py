"""Cost-estimation agent: takeoff, pricing cascade, price checks, the interview and the stream.
A scripted LLM and a fake price source stand in for Ollama and the web."""

from __future__ import annotations

import json
import math
import threading

import pytest
from fastapi.testclient import TestClient
from shapely.geometry import Polygon

import sample_dxf
from fga_engine import estimate_agent as ag
from fga_engine import estimate_reading as rd
from fga_engine import price_research as pr
from fga_engine.app import create_app
from fga_engine.dxf_plan import import_dxf
from fga_engine.estimate import price
from fga_engine.estimate_models import Fact, RateRequest, RateResult, RoomScope, SessionState
from fga_engine.price_source import NoPriceSource, requests_for
from fga_engine.takeoff import RATE_CATALOG, takeoff


@pytest.fixture(scope="module")
def plan():
    data, _ = sample_dxf.sheet()
    return import_dxf(data).plans[0].model  # the first apartment


class FakePrices:
    """1,000 VND per unit for every code, plus a log of the requests it saw."""

    def __init__(self, rate=1000.0, missing=()):
        self.rate, self.missing, self.seen = rate, set(missing), []

    def rates(self, requests, on_result=None):
        out = []
        for r in requests:
            self.seen.append(r)
            res = (RateResult(code=r.code, unit=r.unit, confidence="none") if r.code in self.missing else
                   RateResult(code=r.code, low=self.rate * 0.8, base=self.rate, high=self.rate * 1.25, unit=r.unit,
                              confidence="ok", region=r.region, quality=r.quality))
            if on_result:
                on_result(res)
            out.append(res)
        return out


class FakeLLM:
    """structured(): the next scripted reading, turned into the claims the model would return for the two calls the agent makes per
    message. A script entry is one message's reading in the agent's field names; its quotes are the whole message unless "evidence"
    ({field: quote}) is given, and then a field without an entry gets no claim. stream_text(): scripted chunks."""

    def __init__(self, *extractions, chunks=("Hello ", "there.")):
        self.extractions = list(extractions)
        self.chunks = chunks
        self.streams: list[str] = []
        self.calls = 0
        self.lock = threading.Lock()

    @staticmethod
    def _claims(data, message, table, cls):
        ev = data.get("evidence")
        out = []
        for claim_field, (name, many) in table.items():
            v = data.get(name)
            if v is None or v == [] or (v is False and name in ("use_typical_markups", "fixtures_all_exist", "fixtures_none_exist")):
                continue
            quote = message if ev is None else ev.get(name)
            if quote is None:
                continue
            vals = v if many else [v]
            for x in vals:
                if name == "scope_rooms":
                    x = f"{x['room_id']}={x['state']}"
                out.append({"quote": quote, "field": claim_field, "value": str(x).lower() if isinstance(x, bool) else str(x)})
        return cls.model_validate({"claims": out})

    def structured(self, system, user, schema, max_tokens=None):
        with self.lock:
            n = self.calls
            self.calls += 1
            data = self.extractions[n // 4] if n // 4 < len(self.extractions) else {}
        message = user.split("<message>\n", 1)[1].rsplit("\n</message>", 1)[0]
        if schema in rd.CLAIM_TABLES:
            return self._claims(data, message, rd.CLAIM_TABLES[schema], schema)
        raise AssertionError(schema)

    def stream_text(self, system, user):
        self.streams.append(user)
        yield from self.chunks


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


def find(**kw):
    d = dict(price=250000, currency="VND", unit="m2", quote="Gạch lát nền 60x60: 250.000đ/m2", material_spec="", date="")
    d.update(kw)
    return pr.PriceFind(**d)


def test_quote_must_be_on_the_page_and_carry_the_number():
    page = "Bảng giá. Gạch lát nền 60x60: 250.000đ/m2 tại TP.HCM"
    assert pr.quote_ok(find(), page)
    assert not pr.quote_ok(find(quote="Gạch lát nền 60x60: 999.000đ/m2"), page)  # invented quote
    assert not pr.quote_ok(find(price=480000), page)  # quote is real but is not that number
    assert pr.quote_ok(find(price=150000, quote="sơn 150k/m2"), "giá sơn 150k/m2 hôm nay")


def test_units_and_currency_convert_or_are_rejected():
    assert pr.to_rate(find(unit="m²"), "m2") == 250000
    assert pr.to_rate(find(unit="đ/m2"), "m2") == 250000  # the currency prefix and slash are stripped
    assert pr.to_rate(find(unit="widgets"), "m2") is None
    assert pr.to_rate(find(price=18_000_000, unit="tấn"), "kg") == 18000
    assert pr.to_rate(find(price=10, currency="USD", unit="m2"), "m2") == 10 * pr.usd_vnd()
    assert pr.to_rate(find(unit="m3"), "m2") is None
    assert pr.to_rate(find(currency="EUR"), "m2") is None


def test_aggregation_drops_outliers_and_flags_thin_evidence():
    lo, base, hi, conf = pr.aggregate([200, 220, 240, 260, 9000])
    assert conf == "ok" and lo <= base <= hi and base == 230 and hi < 300
    lo, base, hi, conf = pr.aggregate([100])
    assert conf == "low_confidence" and lo < base < hi
    assert pr.aggregate([])[3] == "none"


class ResearchLLM:
    def __init__(self, finds):
        self.finds, self.user_prompts = finds, []

    def structured(self, system, user, schema, max_tokens=None):
        self.user_prompts.append(user)
        if schema is pr.Queries:
            return pr.Queries(queries=["đơn giá gạch lát nền"])
        return pr.PriceFinds(items=self.finds)


class OneSearch(pr.SearchProvider):
    def __init__(self):
        self.queries = []

    def search(self, query, n=5):
        self.queries.append(query)
        return [pr.Hit(f"https://example.test/{i}", "t", "s") for i in range(3)]


def test_web_source_accepts_real_quotes_rejects_fakes_and_caches():
    page = "Gạch lát nền 60x60: 250.000đ/m2"
    llm = ResearchLLM([find(), find(quote="made up 999.000đ/m2", price=999000)])
    src = pr.LLMWebPriceSource(llm, OneSearch(), fetch=lambda url: page, parallel=1, today=lambda: "2026-11-16")
    req = RateRequest(code="floor_tile", description="floor tile", unit="m2", region="Ho Chi Minh City", quality="normal")
    (res,) = src.rates([req])
    assert res.confidence == "ok" and res.base == 250000 and len(res.sources) == 3
    assert res.sources[0].retrieved_at == "2026-11-16" and "250.000" in res.sources[0].quote
    n = len(llm.user_prompts)
    src.rates([req])
    assert len(llm.user_prompts) == n  # served from the cache


def test_web_source_with_nothing_usable_is_unpriced_and_queries_hold_no_project_data():
    llm = ResearchLLM([])
    search = OneSearch()
    src = pr.LLMWebPriceSource(llm, search, fetch=lambda url: "no prices here", parallel=1)
    req = RateRequest(code="floor_tile", description="floor tile", unit="m2", region="Hanoi", quality="good")
    (res,) = src.rates([req])
    assert res.confidence == "none" and res.base is None
    assert search.queries == ["đơn giá gạch lát nền Hanoi"]  # the region is appended by code
    assert all("<message>" not in p for p in llm.user_prompts)
    assert pr.LLMWebPriceSource(llm, None, parallel=1).rates([req])[0].confidence == "none"


def test_research_requests_carry_only_material_region_and_quality():
    (r,) = requests_for(["floor_tile"], "Hanoi", "good")
    assert set(r.model_dump()) == {"code", "description", "unit", "region", "quality"}
    assert "good" in r.description and r.region == "Hanoi"
    (c,) = requests_for(["plaster"], "Hanoi", "good")
    assert c.quality == ""  # quality-insensitive codes share one cached rate


# ---------- the interview ----------


def turn(llm, plan, state, message, prices=None, **kw):
    return ag.run_turn(llm, prices or FakePrices(), plan, state, message, **kw)


def test_the_example_conversation_asks_before_it_assumes(plan):
    wc_ids = [r.id for r in plan.rooms if r.type == "bathroom"]
    llm = FakeLLM(
        {"kind": "apartment", "quality": "normal"},
        {"city": "Ho Chi Minh City"},
        {"scope_default": "empty", "scope_rooms": [{"room_id": i, "state": "existing"} for i in wc_ids], "work_type": "fit_out"},
    )
    prices = FakePrices()
    t1 = turn(llm, plan, SessionState(), "This is an apartment, the rooms already exist, normal quality", prices)
    assert t1.question.id == "city" and t1.state.status == "interviewing" and prices.seen == []
    assert t1.state.facts["quality"].source == "user"
    t2 = turn(llm, plan, t1.state, "Ho Chi Minh City", prices)
    assert t2.question.id == "scope" and any("except" in o for o in t2.question.options) and "city" not in t2.state.pending
    t3 = turn(llm, plan, t2.state, "Except the toilets, everything is empty", prices)
    assert t3.question.id == "floor_height"  # the next missing thing, not an estimate
    assert {t3.state.scope[i].state for i in wc_ids} == {"existing"} and prices.seen == []


def test_a_place_answer_is_split_by_code_and_an_empty_shell_is_a_fit_out(plan):
    # the model hands back the whole answer as the city; code knows the city and keeps the rest as the district
    st = SessionState(pending=["city"])
    out = turn(FakeLLM({"city": "District 7, Ho Chi Minh City", "district": "Ho Chi Minh City"}), plan, st, "District 7, Ho Chi Minh City")
    assert out.state.facts["city"].value == "Ho Chi Minh City" and out.state.facts["district"].value == "District 7"
    out = turn(FakeLLM({"city": "Thu Duc"}), plan, SessionState(pending=["city"]), "Thu Duc, Ho Chi Minh City")
    assert out.state.facts["city"].value == "Ho Chi Minh City" and out.state.facts["district"].value == "Thu Duc"
    # "empty shell" already says what kind of work it is, so it is not asked again
    out = turn(FakeLLM({"kind": "house", "scope_default": "empty"}), plan, SessionState(), "A house in Hanoi, only the empty shell")
    assert out.state.facts["work_type"].value == "fit_out" and "work_type" not in out.state.pending
    out = turn(FakeLLM({}), plan, SessionState(), "Renovation of an empty shell")
    assert out.state.facts["work_type"].value == "renovation"


def test_i_dont_know_is_recorded_as_assumed_and_shown_in_the_result(plan):
    st = full_state(plan)
    del st.facts["floor_height"]
    st.pending = ["floor_height"]
    out = turn(FakeLLM({}), plan, st, ag.UNKNOWN_OPTION)
    assert out.state.facts["floor_height"].source == "assumed"
    assert out.estimate is not None and any("floor_height" in a for a in out.estimate.assumptions)


def test_several_slots_in_one_message_and_typical_markups(plan):
    ex = {"city": "Hanoi", "kind": "apartment", "work_type": "fit_out", "quality": "good", "floor_height": 2.8,
          "mep_in_scope": True, "scope_default": "empty", "use_typical_markups": True}
    out = turn(FakeLLM(ex), plan, SessionState(), "Hanoi apartment fit-out, good quality, 2.8 m, with MEP, all empty, use typical rates")
    assert out.question is None and out.estimate is not None and out.state.status == "ready"
    assert out.state.facts["vat_percent"].source == "assumed"
    assert {ln.code for ln in out.estimate.lines} <= {r.code for r in out.estimate.rates}
    assert "electrical_point" in {ln.code for ln in out.estimate.lines}


def test_missing_slots_come_from_code_not_the_llm(plan):
    # an LLM that extracts nothing cannot make the agent skip a question or invent an answer
    out = turn(FakeLLM({}), plan, SessionState(), "hello")
    assert out.question.id == "city" and out.estimate is None
    assert [s.name for s in ag.missing_slots(SessionState(), plan)][:3] == ["city", "kind", "scope"]


def test_changing_an_answer_reprices_only_what_changed(plan):
    prices = FakePrices()
    st = full_state(plan)
    first = turn(FakeLLM({}), plan, st, "go", prices)
    searched = {r.code for r in prices.seen}
    assert first.estimate is not None and searched
    prices.seen.clear()
    markups_only = turn(FakeLLM({"vat_percent": 10}), plan, first.state, "VAT is 10%", prices)
    assert prices.seen == [] and markups_only.estimate.total["base"] > first.estimate.total["base"]
    quality = turn(FakeLLM({"quality": "luxury"}), plan, markups_only.state, "luxury", prices)
    again = {r.code for r in prices.seen}
    assert again == {c for c in searched if RATE_CATALOG[c].quality_sensitive}  # plaster/waterproofing keep their rates
    unchanged = turn(FakeLLM({}), plan, quality.state, "thanks", prices)
    assert unchanged.estimate is None  # nothing changed: no new estimate


def test_scope_changes_the_total(plan):
    st = full_state(plan)
    a = turn(FakeLLM({}), plan, st, "go")
    wc = [r.id for r in plan.rooms if r.type == "bathroom"]
    b = turn(FakeLLM({"scope_rooms": [{"room_id": i, "state": "existing"} for i in wc]}), plan, a.state, "toilets exist")
    assert b.estimate.total["base"] < a.estimate.total["base"]


def test_unpriced_codes_are_flagged_and_a_user_rate_fills_them(plan):
    prices = FakePrices(missing={"paint"})
    st = full_state(plan)
    a = turn(FakeLLM({}), plan, st, "go", prices)
    assert any("paint" in w.lower() for w in a.estimate.warnings)
    b = turn(FakeLLM({}), plan, a.state, "", prices, rate_overrides={"paint": 90000})
    paint = next(ln for ln in b.estimate.lines if ln.code == "paint")
    assert paint.priced and paint.rate_base == 90000 and paint.rate_confidence == "user"


def test_no_search_provider_means_unpriced_with_a_reason(plan):
    out = turn(FakeLLM({}), plan, full_state(plan), "go", NoPriceSource())
    assert out.estimate.total["base"] == 0 and all(not ln.priced for ln in out.estimate.lines)
    assert all("provider" in r.note for r in out.estimate.rates)


def test_explaining_a_line(plan):
    st = full_state(plan)
    a = turn(FakeLLM({}), plan, st, "go")
    llm = FakeLLM({"explain": "floor tile"})
    b = turn(llm, plan, a.state, "why is the floor tile so high?")
    assert b.estimate is None and "floor_tile" in llm.streams[0] and "net floor area" in llm.streams[0]


def test_drawing_without_rooms_fails_cleanly(plan):
    empty = plan.model_copy(update={"rooms": [], "walls": [], "openings": [], "fixtures": []})
    out = turn(FakeLLM({"city": "Hanoi", "kind": "apartment", "work_type": "fit_out", "quality": "normal", "floor_height": 3,
                        "mep_in_scope": False, "use_typical_markups": True}), empty, SessionState(), "use typical rates")
    assert out.state.status == "failed" and out.estimate is None


# ---------- the stream ----------


def ndjson(resp):
    return [json.loads(x) for x in resp.text.splitlines() if x]


def stream_client(llm, prices=None):
    return TestClient(create_app(llm, prices or FakePrices()))


def post(c, plan, state, message, **kw):
    return c.post("/v1/estimate/agent/turn", json={"model": plan.model_dump(mode="json"), "state": state.model_dump(mode="json"),
                                                   "message": message, **kw})


def test_a_question_turn_streams_progress_tokens_question_then_done(plan):
    c = stream_client(FakeLLM({"city": "Hanoi"}, chunks=("Which ", "city?")))
    evs = ndjson(post(c, plan, SessionState(), "Hanoi"))
    names = [e["event"] for e in evs]
    assert names[0] == "progress" and names[-1] == "done"
    assert names.index("fact") < names.index("token") < names.index("question") < names.index("done")
    assert "".join(e["data"]["text"] for e in evs if e["event"] == "token") == "Which city?"
    done = evs[-1]["data"]
    assert done["reply"] == "Which city?" and done["state"]["facts"]["city"]["value"] == "Hanoi" and done["question"]["id"] in ("kind",)


def test_an_estimate_turn_streams_rates_then_the_estimate_then_the_closing_reply(plan):
    st = full_state(plan)
    evs = ndjson(post(stream_client(FakeLLM({})), plan, st, "go"))
    names = [e["event"] for e in evs]
    assert names.count("rate") >= 5
    assert names.index("rate") < names.index("estimate") < names.index("token") < names.index("done")
    assert evs[-1]["data"]["estimate"]["total"]["base"] > 0
    assert evs[-1]["data"]["state"]["status"] == "ready"


def test_llm_failure_mid_stream_is_an_error_event(plan):
    class Broken(FakeLLM):
        def stream_text(self, system, user):
            yield "partial"
            from fga_engine.llm import LLMError
            raise LLMError("ollama went away")

    evs = ndjson(post(stream_client(Broken({"city": "Hanoi"})), plan, SessionState(), "Hanoi"))
    assert evs[-1]["event"] == "error" and evs[-1]["status"] == 502
    assert any(e["event"] == "token" for e in evs)


def test_a_dead_llm_before_any_text_falls_back_to_the_question_text(plan):
    class Silent(FakeLLM):
        def stream_text(self, system, user):
            from fga_engine.llm import LLMError
            raise LLMError("down")
            yield

    evs = ndjson(post(stream_client(Silent({})), plan, SessionState(), "hi"))
    assert evs[-1]["event"] == "done" and "located" in evs[-1]["data"]["reply"]


def test_chips_match_the_slot_asked_and_always_offer_you_decide(plan):
    out = turn(FakeLLM({}), plan, SessionState(), "hi")
    assert out.question.id == "city" and out.question.options[-1] == ag.UNKNOWN_OPTION
    for slot in ag.SLOTS:
        if slot.name == "scope":
            continue
        q = slot.ask(full_state(plan), plan)
        assert q.id == slot.name


# ---------- a small model must not make the agent assume ----------


def test_flags_the_model_sets_unprompted_do_not_assume_anything(plan):
    # the message states a kind and a quality; the LLM also (wrongly) claims "typical markups" and "you decide"
    ex = {"kind": "apartment", "quality": "normal", "use_typical_markups": True, "decide_for_me": ["markups", "city", "scope"]}
    out = turn(FakeLLM(ex), plan, SessionState(), "This is an apartment, the rooms already exist, normal quality")
    assert out.question.id == "city"
    assert set(out.state.facts) == {"kind", "quality"} and out.state.scope == {}
    assert all(f.source == "user" for f in out.state.facts.values())


def test_typical_and_give_up_phrases_still_work_when_said(plan):
    st = full_state(plan)
    for n in ("vat_percent", "overhead_percent", "profit_percent", "design_percent", "contingency_percent"):
        del st.facts[n]
    st.pending = ["markups"]
    out = turn(FakeLLM({}), plan, st, "use typical rates")
    assert out.state.facts["vat_percent"].source == "assumed" and out.estimate is not None
    st2 = full_state(plan)
    del st2.facts["quality"]
    st2.pending = ["quality"]
    out2 = turn(FakeLLM({"decide_for_me": ["quality"]}), plan, st2, "Không biết, bạn quyết")
    assert out2.state.facts["quality"].source == "assumed"


def test_the_same_scope_twice_is_one_fact_event(plan):
    ids = [r.id for r in plan.rooms]
    ex = {"scope_default": "empty", "scope_rooms": [{"room_id": i, "state": "empty"} for i in ids]}
    changes = ag.apply_extraction(SessionState(), plan, ag.Extracted.model_validate(ex), "everything is empty")
    assert len([c for c in changes if c[0].startswith("scope.")]) == len(ids)


def test_extraction_calls_ask_for_no_live_thinking(plan):
    seen = []

    class Spy(FakeLLM):
        def structured(self, system, user, schema, max_tokens=None):
            from fga_engine import progress
            seen.append(progress.thinking_wanted())
            return super().structured(system, user, schema, max_tokens)

    c = stream_client(Spy({}))
    post(c, plan, SessionState(), "hi")
    assert seen == [False] * 4  # every reading is quiet


def test_code_reads_what_the_llm_missed_and_drops_an_unprompted_scope(plan):
    # the model returns nothing useful for the first message and invents "existing" for the second
    out = turn(FakeLLM({}), plan, SessionState(), "This is an apartment, the rooms already exist, normal quality")
    assert out.state.facts["kind"].value == "apartment" and out.state.facts["quality"].value == "normal"
    out2 = turn(FakeLLM({"scope_default": "existing"}), plan, SessionState(), "the rooms already exist")
    assert out2.state.scope == {}


def test_the_agents_own_chips_are_understood_without_the_llm(plan):
    st = SessionState(pending=["city"])
    out = turn(FakeLLM({}), plan, st, "Ho Chi Minh City")
    assert out.state.facts["city"].value == "Ho Chi Minh City" and out.state.facts["city"].source == "user"
    st = SessionState(pending=["scope"])
    out = turn(FakeLLM({}), plan, st, "Everything is empty except the bathrooms")
    wc = {r.id for r in plan.rooms if r.type == "bathroom"}
    assert {i for i, s in out.state.scope.items() if s.state == "existing"} == wc
    assert all(s.state == "empty" for i, s in out.state.scope.items() if i not in wc)
    for chip, field, want in [("3.3 m", "floor_height", 3.3), ("2", "storeys", 2)]:
        o = turn(FakeLLM({}), plan, SessionState(pending=[field]), chip)
        assert o.state.facts[field].value == want
    o = turn(FakeLLM({}), plan, SessionState(pending=["work_type"]), "Fit-out of an empty shell")
    assert o.state.facts["work_type"].value == "fit_out"
    o = turn(FakeLLM({}), plan, SessionState(pending=["mep_in_scope"]), "No, exclude them")
    assert o.state.facts["mep_in_scope"].value is False


def test_exist_alone_never_sets_every_room(plan):
    every = [{"room_id": r.id, "state": "existing"} for r in plan.rooms]
    out = turn(FakeLLM({"scope_rooms": every}), plan, SessionState(), "the rooms already exist")
    assert out.state.scope == {}
    wc = next(r for r in plan.rooms if r.name == "Wc")
    out = turn(FakeLLM({"scope_rooms": [{"room_id": wc.id, "state": "existing"}]}), plan, SessionState(), "the Wc already exists")
    assert list(out.state.scope) == [wc.id]


def test_a_room_can_be_marked_only_when_the_message_refers_to_it(plan):
    by_type = {t: [r.id for r in plan.rooms if r.type == t] for t in {r.type for r in plan.rooms}}
    wrong = [{"room_id": i, "state": "existing"} for i in by_type["bathroom"] + by_type["balcony"]]  # the model adds the balcony
    out = turn(FakeLLM({"scope_default": "empty", "scope_rooms": wrong}), plan, SessionState(), "Except the toilets, everything is empty")
    assert {i for i, s in out.state.scope.items() if s.state == "existing"} == set(by_type["bathroom"])
    bed2 = next(r for r in plan.rooms if r.name == "Bedroom 2")
    out = turn(FakeLLM({"scope_rooms": [{"room_id": r.id, "state": "existing"} for r in plan.rooms if r.type == "bedroom"]}), plan,
               SessionState(), "Bedroom 2 is already finished")
    assert list(out.state.scope) == [bed2.id]  # a named room is not widened to every bedroom


def test_the_region_is_pinned_by_code_not_the_model():
    assert pr.region_query("đơn giá gạch lát nền Hà Nội", "Ho Chi Minh City") is None  # another city: dropped
    assert pr.region_query("giá gạch lát nền", "Ho Chi Minh City") == "giá gạch lát nền Ho Chi Minh City"
    assert pr.region_query("giá gạch lát nền TP.HCM", "Ho Chi Minh City") == "giá gạch lát nền TP.HCM"  # same city, other spelling
    assert pr.region_query("giá gạch lát nền", "Hanoi") == "giá gạch lát nền Hanoi"
    assert pr.region_query("giá gạch", "Bien Hoa") == "giá gạch Bien Hoa"


def test_a_query_naming_the_wrong_city_never_reaches_the_search_provider():
    class WrongCity(ResearchLLM):
        def structured(self, system, user, schema, max_tokens=None):
            if schema is pr.Queries:
                return pr.Queries(queries=["đơn giá gạch lát nền Hà Nội", "floor tile price"])
            return super().structured(system, user, schema, max_tokens)

    search = OneSearch()
    src = pr.LLMWebPriceSource(WrongCity([]), search, fetch=lambda u: "x", parallel=1)
    src.rates([RateRequest(code="floor_tile", description="floor tile", unit="m2", region="Ho Chi Minh City", quality="normal")])
    assert search.queries == ["floor tile price Ho Chi Minh City"]


def test_the_cited_sources_are_only_those_the_rate_rests_on():
    page = "Gạch lát nền 60x60: 250.000đ/m2"
    finds = [find(), find(price=250000), find(price=999_999_999, quote="Gạch lát nền 60x60: 250.000đ/m2 999999999")]
    src = pr.LLMWebPriceSource(ResearchLLM(finds), OneSearch(), fetch=lambda u: page, parallel=1)
    (res,) = src.rates([RateRequest(code="floor_tile", description="x", unit="m2", region="Hanoi", quality="normal")])
    assert res.base == 250000 and "999" not in " ".join(s.quote for s in res.sources)


# ---------- fixtures: a floor plan shows where they go, not what exists ----------

from fga_engine.domain.schemas import PlacedFixture


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


def test_the_agent_asks_which_fixtures_exist_before_it_prices(fxplan):
    st = full_state(fxplan)
    out = turn(FakeLLM({}), fxplan, st, "go")
    assert out.question.id == "fixtures" and out.estimate is None
    assert "assume none of them exist" in out.question.text and "toilet (Bath)" in out.question.text
    assert out.question.options[:2] == ["None of them exist", "All of them already exist"]
    none = turn(FakeLLM({}), fxplan, out.state, "None of them exist")
    assert none.estimate is not None and none.state.facts["fixtures"].source == "user"
    assert "fixture_toilet" in {ln.code for ln in none.estimate.lines}
    allx = turn(FakeLLM({}), fxplan, out.state, "All of them already exist")
    assert not [ln for ln in allx.estimate.lines if ln.package == "Fixtures"]


def test_some_fixtures_exist_only_those_the_message_refers_to(fxplan):
    st = full_state(fxplan)
    st.pending = ["fixtures"]
    # the model also claims the kitchen counter, which the person never mentioned
    out = turn(FakeLLM({"fixtures_existing": ["f1", "f3"]}), fxplan, st, "The toilet in the bath is already installed")
    assert out.state.fixtures_existing == ["f1"] and out.state.facts["fixtures"].value == "some exist"
    codes = {ln.code for ln in out.estimate.lines}
    assert "fixture_toilet" not in codes and {"fixture_basin", "fixture_kitchen_counter"} <= codes


def test_i_dont_know_assumes_no_fixture_exists_and_says_so(fxplan):
    st = full_state(fxplan)
    st.pending = ["fixtures"]
    out = turn(FakeLLM({}), fxplan, st, ag.UNKNOWN_OPTION)
    assert out.state.facts["fixtures"].source == "assumed" and out.state.fixtures_existing == []
    assert any("fixtures" in a for a in out.estimate.assumptions)


# ---------- the demo search provider ----------


def test_demo_search_prices_every_code_through_the_real_pipeline_and_labels_it():
    from fga_engine.demo_search import BASE, DemoSearch, demo_rate

    assert set(BASE) == set(RATE_CATALOG)  # nothing the takeoff can ask for is missing

    class Reader(ResearchLLM):  # a stand-in for the LLM that reads the page text it is given
        def structured(self, system, user, schema, max_tokens=None):
            if schema is pr.Queries:
                return pr.Queries(queries=["đơn giá"])
            import re
            m = re.search(r": ([\d.]+)đ/(\S+)", user)
            if not m:
                return pr.PriceFinds(items=[])
            quote = re.search(r"[^.]*: [\d.]+đ/\S+", user).group(0).strip()
            return pr.PriceFinds(items=[pr.PriceFind(price=float(m.group(1).replace(".", "")), currency="VND", unit=m.group(2), quote=quote)])

    src = pr.LLMWebPriceSource(Reader([]), DemoSearch(), parallel=1)
    reqs = requests_for(list(RATE_CATALOG), "Ho Chi Minh City", "normal")
    out = {r.code: r for r in src.rates(reqs)}
    for code, res in out.items():
        assert res.confidence == "ok", (code, res.note)
        assert res.base == pytest.approx(demo_rate(code, "Ho Chi Minh City", "normal"), rel=0.12)
        assert all("demo.invalid" in s.url for s in res.sources)
    assert out["floor_tile"].base < pr.LLMWebPriceSource(Reader([]), DemoSearch(), parallel=1).rates(
        requests_for(["floor_tile"], "Ho Chi Minh City", "luxury"))[0].base  # quality moves the rate


def test_an_estimate_on_demo_prices_says_it_is_demo_data(plan):
    r = RateResult(code="floor_tile", low=1, base=2, high=3, unit="m2", confidence="ok",
                   sources=[pr.RateSource(url="https://demo.invalid/floor_tile/0", quote="q", retrieved_at="")])
    st = full_state(plan)
    est = price(takeoff(plan, st), {"floor_tile": r}, st)
    assert est.warnings[0].startswith("DEMO DATA")


def test_a_named_city_and_an_explicit_floor_height_never_depend_on_the_llm(plan):
    out = turn(FakeLLM({}), plan, SessionState(), "Apartment in Ho Chi Minh City, floor height 3.2 m")
    assert out.state.facts["city"].value == "Ho Chi Minh City" and out.state.facts["city"].source == "user"
    assert out.state.facts["floor_height"].value == 3.2


def test_include_or_exclude_mep_is_read_by_code(plan):
    assert turn(FakeLLM({}), plan, SessionState(), "include MEP").state.facts["mep_in_scope"].value is True
    assert turn(FakeLLM({}), plan, SessionState(), "without plumbing please").state.facts["mep_in_scope"].value is False


def test_work_type_and_structure_keywords_are_read_by_code(plan):
    out = turn(FakeLLM({}), plan, SessionState(), "fit-out only, finishes only")
    assert out.state.facts["work_type"].value == "fit_out" and out.state.facts["structure_in_scope"].value is False


# ---------- changing a finished estimate ----------


def _finished(plan, prices=None):
    st = full_state(plan)
    first = turn(FakeLLM({}), plan, st, "go", prices=prices)
    assert first.estimate is not None
    return first


def test_removing_an_item_from_a_finished_estimate_really_removes_it(plan):
    first = _finished(plan)
    assert {"door_interior", "door_entrance"} & {ln.code for ln in first.estimate.lines}
    out = turn(FakeLLM({}), plan, first.state, "Remove the cost for the doors, they already exist")
    codes = {ln.code for ln in out.estimate.lines}
    assert not codes & {"door_interior", "door_entrance"} and "floor_tile" in codes
    assert out.state.excluded == ["door_entrance", "door_interior"]
    assert out.estimate.total["base"] < first.estimate.total["base"]
    assert any(w.startswith("Left out at your request") for w in out.estimate.warnings)


def test_the_model_cannot_remove_items_the_message_never_mentions(plan):
    first = _finished(plan)
    out = turn(FakeLLM({"exclude_items": ["floor_tile"]}), plan, first.state, "What is the biggest cost?")
    assert out.state.excluded == [] and out.estimate is None  # nothing changed, no re-price


def test_an_item_can_be_put_back(plan):
    first = _finished(plan)
    gone = turn(FakeLLM({}), plan, first.state, "skip the windows")
    back = turn(FakeLLM({}), plan, gone.state, "add the windows back")
    assert "window" not in {ln.code for ln in gone.estimate.lines}
    assert "window" in {ln.code for ln in back.estimate.lines} and back.state.excluded == []


def test_a_message_that_changes_nothing_never_claims_a_change(plan):
    first = _finished(plan)
    llm = FakeLLM({})
    turn(llm, plan, first.state, "make the kitchen bigger")
    assert any("Nothing in the estimate was changed" in s for s in llm.streams)
    assert any("Never say or imply that anything was updated" in s for s in llm.streams)


def test_the_reply_after_a_real_change_is_given_exactly_what_changed(plan):
    first = _finished(plan)
    llm = FakeLLM({})
    turn(llm, plan, first.state, "the doors already exist")
    assert any("changes_applied_this_turn" in s and "door_interior" in s for s in llm.streams)


def test_an_exception_in_the_first_message_takes_that_opening_out(plan):
    msg = ("This is an apartment in Da Nang, estimate the cost with normal quality.\n- Fixtures are almost empty, except entrance door.\n"
           "- Rooms are empty cells")
    out = turn(FakeLLM({}), plan, SessionState(), msg)
    assert out.state.excluded == ["door_entrance"]
    assert out.state.facts["fixtures"].value == "none exist"


def test_exception_and_only_phrases_name_openings_only():
    assert ag.exception_items("everything is empty except the entrance door") == ["door_entrance"]
    assert ag.exception_items("only the windows are installed") == ["window"]
    assert ag.exception_items("everything is empty except the toilets") == []  # a fixture: its own slot
    assert ag.exception_items("estimate the cost, normal quality") == []


# ---------- reading any language: the quote check, with the keyword rules off ----------


@pytest.fixture
def no_rules(monkeypatch):
    monkeypatch.setattr(ag, "RULES", False)


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
    assert rd.grounded(quote, message) is ok


def test_values_without_a_verified_quote_are_dropped(no_rules, plan):
    llm = FakeLLM({"use_typical_markups": True, "exclude_items": ["door_interior"], "scope_default": "existing",
                   "evidence": {"use_typical_markups": "typical percentages", "exclude_items": "doors exist"}})
    out = turn(llm, plan, SessionState(), "This is an apartment, the rooms already exist, normal quality")
    assert out.state.excluded == [] and out.state.scope == {}
    assert not any(k.endswith("_percent") for k in out.state.facts)


@pytest.mark.parametrize("message,quote,items", [
    ("Fixtures are almost empty, except entrance door.", "except entrance door", ["door_entrance"]),
    ("Trừ cửa chính, thiết bị chưa có cái nào", "Trừ cửa chính", ["door_entrance"]),
    ("入口のドアを除いて、設備は何もありません", "入口のドアを除いて", ["door_entrance"]),
])
def test_the_model_reads_an_exception_in_any_language(no_rules, plan, message, quote, items):
    llm = FakeLLM({"exclude_items": items, "fixtures_none_exist": True,
                   "evidence": {"exclude_items": quote, "fixtures_none_exist": message}})
    out = turn(llm, plan, SessionState(), message)
    assert out.state.excluded == items and out.state.facts["fixtures"].value == "none exist"


def test_the_two_readings_run_for_every_message_and_a_failure_is_not_hidden(plan):
    class Down(FakeLLM):
        def structured(self, system, user, schema, max_tokens=None):
            raise ag.LLMError("ollama down")
    with pytest.raises(ag.LLMError):
        turn(Down(), plan, SessionState(), "apartment in Da Nang")
