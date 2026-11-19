"""An Engine that measures and prices the small plan of `estimate_fixtures` (seven rooms, four fixtures) by simple rules, so the turn's
tests do not depend on the real takeoff. A fake researcher stands for the web: 1,000 VND per unit unless told a rate is missing."""

from __future__ import annotations

import json

from estimate_fixtures import FIXTURES, ROOMS
from scripted_engine import ScriptedEngine

from estimate_fixtures import ESTIMATE_LIMITS
from fga_agent.domain.errors import EngineUnavailable
from fga_agent.domain.model.estimate import Estimate, EstimateLine
from fga_agent.domain.model.plan_summary import PlanSummary
from fga_agent.domain.model.rate import RateResult
from fga_agent.domain.model.takeoff import Takeoff, TakeoffLine

PRICED = ("basin", "bathtub", "kitchen_counter", "shower", "sink", "toilet", "wardrobe")
SENSITIVE = {s.code: s.quality_sensitive for s in ESTIMATE_LIMITS.rate_catalog}
MARKUPS = ("overhead_percent", "profit_percent", "design_percent", "contingency_percent")


class EstimateEngine(ScriptedEngine):
    def __init__(self, rooms=ROOMS, fixtures=FIXTURES, fail_on: str | None = None) -> None:
        super().__init__(limits=ESTIMATE_LIMITS)
        self.rooms, self.fixtures, self.fail_on = rooms, fixtures, fail_on
        self.takeoffs: list = []
        self.priced: list = []

    def _existing(self, facts) -> set[str]:
        return {room for room, s in facts.scope.items() if s.state == "existing"}

    def _pending(self, facts) -> list[str]:
        gone = self._existing(facts)
        return [f.id for f in self.fixtures if f.kind in PRICED and f.id not in facts.fixtures_existing and f.room_id not in gone]

    def summarize_plan(self, ctx, model, facts):
        self._guard(None)
        self.summarized.append((model, facts))
        work = facts.facts.get("work_type")
        structure = (work is not None and work.value == "new_build") or (
            "structure_in_scope" in facts.facts and facts.facts["structure_in_scope"].value is True)
        return PlanSummary(rooms=self.rooms, fixtures=self.fixtures, pending_fixtures=self._pending(facts), structure_in_scope=structure)

    def takeoff(self, ctx, model, facts):
        self._guard(None)
        if self.fail_on == "takeoff":
            raise EngineUnavailable("the engine went away")
        self.takeoffs.append(facts)
        if not self.rooms:
            return Takeoff("unsupported", [], ["this drawing has no rooms, so there is nothing to estimate"])
        gone = self._existing(facts)
        area = sum(r.area_m2 for r in self.rooms if r.id not in gone)
        height = "assumed" if facts.facts["floor_height"].source == "assumed" else "measured"
        lines = [TakeoffLine("floor_tile", "Finishes", "floor tile", area, "m2", "net floor area", "measured"),
                 TakeoffLine("paint", "Finishes", "wall paint", area * 2.5, "m2", "wall area from the floor height", height),
                 TakeoffLine("plaster", "Plaster", "plaster", area * 2.5, "m2", "wall area", "derived"),
                 TakeoffLine("door_interior", "Openings", "interior doors", 4, "each", "4 doors", "measured"),
                 TakeoffLine("door_entrance", "Openings", "entrance door", 1, "each", "1 door", "measured")]
        by_id = {f.id: f for f in self.fixtures}
        lines += [TakeoffLine(f"fixture_{by_id[i].kind}", "Fixtures", by_id[i].kind, 1, "each", "1 in the plan", "measured")
                  for i in self._pending(facts)]
        if "mep_in_scope" in facts.facts and facts.facts["mep_in_scope"].value is True:
            lines.append(TakeoffLine("electrical_point", "MEP", "electrical points", 10, "each", "10 points", "assumed"))
        return Takeoff("ok", [ln for ln in lines if ln.code not in facts.excluded], [])

    def price_estimate(self, ctx, takeoff, rates, facts):
        self._guard(None)
        self.priced.append(rates)
        by = {r.code: r for r in rates}
        lines, warnings, packages = [], [], {}
        for ln in takeoff.lines:
            rate = by.get(ln.code)
            priced = rate is not None and rate.base is not None
            base = ln.quantity * rate.base if priced else 0.0
            lines.append(EstimateLine(ln.code, ln.package, ln.description, ln.quantity, ln.unit, ln.basis, ln.confidence, priced,
                                      rate.base if priced else None, rate.confidence if rate else "none", base * 0.8, base, base * 1.25))
            if not priced:
                warnings.append(f"No price found for {ln.code}; it is left out of the total.")
            pkg = packages.setdefault(ln.package, {"low": 0.0, "base": 0.0, "high": 0.0})
            pkg["low"] += base * 0.8
            pkg["base"] += base
            pkg["high"] += base * 1.25
        direct = sum(p["base"] for p in packages.values())
        percent = sum(float(facts.facts[n].value) for n in MARKUPS if n in facts.facts)
        vat = float(facts.facts["vat_percent"].value) if "vat_percent" in facts.facts else 0.0
        base = direct * (1 + percent / 100) * (1 + vat / 100)
        if any("demo.invalid" in s.url for r in rates for s in r.sources):
            warnings.insert(0, "DEMO DATA: these prices are invented.")
        assumptions = [f"{k} = {f.value} (assumed)" for k, f in facts.facts.items() if f.source == "assumed"]
        total = {"low": base * 0.8, "base": base, "high": base * 1.25}
        doc = {"currency": "VND", "lines": [vars(ln) for ln in lines], "packages": packages, "total": total,
               "assumptions": assumptions, "warnings": warnings, "rates": [_rate_json(r) for r in rates]}
        return Estimate(total=total, packages=packages, assumptions=assumptions, warnings=warnings, lines=lines, rates=list(rates),
                        document=json.loads(json.dumps(doc)))


def _rate_json(r: RateResult) -> dict:
    return {"code": r.code, "low": r.low, "base": r.base, "high": r.high, "unit": r.unit, "currency": r.currency,
            "confidence": r.confidence, "region": r.region, "quality": r.quality, "note": r.note,
            "sources": [{"url": s.url, "quote": s.quote, "retrieved_at": s.retrieved_at} for s in r.sources]}


class FakePrices:
    """The price researcher: 1,000 VND per unit for every code but the missing ones, and a log of what was asked."""

    def __init__(self, rate: float = 1000.0, missing=(), gate=None) -> None:
        self.rate, self.missing, self.seen, self.gate = rate, set(missing), [], gate

    def requests_for(self, ctx, codes, region, quality):
        from fga_agent.domain.model.rate import RateRequest
        return [RateRequest(c, c, "m2", region, quality if SENSITIVE[c] else "") for c in codes]

    def research(self, ctx, requests, on_progress, on_rate=None):
        out = []
        for r in requests:
            self.seen.append(r)
            if self.gate is not None:
                self.gate()
            res = (RateResult(code=r.code, unit=r.unit, confidence="none", region=r.region, quality=r.quality, note="no price") if r.code in self.missing
                   else RateResult(code=r.code, unit=r.unit, low=self.rate * 0.8, base=self.rate, high=self.rate * 1.25, confidence="ok",
                                   region=r.region, quality=r.quality))
            if on_rate:
                on_rate(res)
            out.append(res)
        return out
