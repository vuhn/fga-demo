from __future__ import annotations

import copy
import hashlib
import json
from collections.abc import Callable

from ..domain.model.call_context import CallContext
from ..domain.model.design_limits import DesignLimits
from ..domain.model.estimate_facts import EstimateFacts
from ..domain.model.estimate_session import EstimateFact, FactChange, Question, RoomScope, SessionState
from ..domain.model.extracted import Extracted
from ..domain.model.plan_summary import PlanSummary
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.keyword_reader import KeywordReader
from .prompts import estimate_questions as ask

FACT_FIELDS = ("kind", "work_type", "city", "district", "quality", "floor_height", "storeys", "structure_in_scope", "mep_in_scope",
               "vat_percent", "overhead_percent", "profit_percent", "design_percent", "contingency_percent")
Slot = tuple[str, Callable[[SessionState, PlanSummary], bool], Callable[[SessionState, PlanSummary], bool],
             Callable[[SessionState, PlanSummary], Question]]


def _has(*names: str) -> Callable[[SessionState, PlanSummary], bool]:
    return lambda s, p: all(n in s.facts for n in names)


def _always(s: SessionState, p: PlanSummary) -> bool:
    return True


def _is_renovation(s: SessionState, p: PlanSummary) -> bool:
    return "work_type" in s.facts and s.facts["work_type"].value == "renovation"


class EstimateInterviewService:
    """`rules=False` drops the keyword layer on top of the model's reading (the eval measures what each rule still buys)."""

    def __init__(self, catalog: DesignCatalogLoader, keywords: KeywordReader, rules: bool = True) -> None:
        self._catalog, self._keywords, self._rules = catalog, keywords, rules

    def facts(self, state: SessionState) -> EstimateFacts:
        return EstimateFacts(facts=state.facts, scope=state.scope, fixtures_existing=state.fixtures_existing, excluded=state.excluded)

    # ---------- which questions are open ----------

    def _slots(self, limits: DesignLimits) -> list[Slot]:
        markups = list(limits.default_markups)
        return [
            ("city", _always, _has("city"), lambda s, p: ask.city()),
            ("kind", _always, _has("kind"), lambda s, p: ask.kind()),
            ("scope", _always, lambda s, p: all(r.id in s.scope for r in p.rooms), lambda s, p: ask.scope(p, s)),
            ("work_type", _always, _has("work_type"), lambda s, p: ask.work_type()),
            ("fixtures", lambda s, p: bool(p.pending_fixtures) or "fixtures" in s.facts, _has("fixtures"), lambda s, p: ask.fixtures(p)),
            ("quality", _always, _has("quality"), lambda s, p: ask.quality()),
            ("structure_in_scope", _is_renovation, _has("structure_in_scope"), lambda s, p: ask.structure_in_scope()),
            ("storeys", lambda s, p: p.structure_in_scope, _has("storeys"), lambda s, p: ask.storeys()),
            ("floor_height", _always, _has("floor_height"), lambda s, p: ask.floor_height()),
            ("mep_in_scope", _always, _has("mep_in_scope"), lambda s, p: ask.mep_in_scope()),
            ("markups", _always, lambda s, p: all(n in s.facts for n in markups),
             lambda s, p: ask.markups([n for n in markups if n not in s.facts])),
        ]

    def missing(self, ctx: CallContext, state: SessionState, summary: PlanSummary) -> list[Question]:
        slots = self._slots(self._catalog.limits(ctx))
        return [question(state, summary) for _, applies, done, question in slots if applies(state, summary) and not done(state, summary)]

    # ---------- what the person said ----------

    def record(self, ctx: CallContext, state: SessionState, summary: PlanSummary, extracted: Extracted,
               message: str) -> tuple[SessionState, list[FactChange]]:
        limits = self._catalog.limits(ctx)
        s, changed = copy.deepcopy(state), []
        for name in FACT_FIELDS:
            if (v := getattr(extracted, name)) is not None:
                self._set_fact(s, name, v, "user", changed)
        self._record_items(s, extracted, [r.code for r in limits.rate_catalog], changed)
        self._record_markups(s, extracted, message, limits, changed)
        self._record_scope(s, summary, extracted, changed)
        self._record_fixtures(s, summary, limits, extracted, message, changed)
        self._record_giving_up(s, summary, extracted, message, limits, changed)
        if "work_type" in s.facts and s.facts["work_type"].value == "new_build":  # a new build always includes structure
            s.facts.pop("structure_in_scope", None)
        return s, changed

    def _set_fact(self, s: SessionState, name: str, value, source: str, changed: list[FactChange]) -> None:
        cur = s.facts.get(name)
        if cur is not None and cur.value == value and cur.source == source:
            return
        s.facts[name] = EstimateFact(value, source)
        changed.append(FactChange(name, value, source))

    def _set_scope(self, s: SessionState, room_id: str, value: str, source: str, changed: list[FactChange]) -> None:
        cur = s.scope.get(room_id)
        if cur is not None and cur.state == value and cur.source == source:
            return
        s.scope[room_id] = RoomScope(value, source)
        changed.append(FactChange(f"scope.{room_id}", value, source))

    def _record_items(self, s: SessionState, ex: Extracted, codes: list[str], changed: list[FactChange]) -> None:
        drop, back = [c for c in ex.exclude_items if c in codes], [c for c in ex.include_items if c in codes]
        if not (drop or back):
            return
        before = set(s.excluded)
        s.excluded = sorted((before | set(drop)) - set(back))
        if set(s.excluded) != before:
            changed.append(FactChange("excluded", ", ".join(s.excluded) or "nothing", "user"))

    def _record_markups(self, s: SessionState, ex: Extracted, message: str, limits: DesignLimits, changed: list[FactChange]) -> None:
        """A small model sometimes sets this flag unprompted. The words must really be in the message, or nothing is assumed."""
        asks_typical = (not self._rules or self._keywords.says_typical(message)) and (
            ex.use_typical_markups or (self._rules and s.pending[:1] == ["markups"]))
        if not asks_typical:
            return
        for name, value in limits.default_markups.items():
            if name not in s.facts or s.facts[name].source != "user":
                self._set_fact(s, name, value, "assumed", changed)

    def _record_scope(self, s: SessionState, summary: PlanSummary, ex: Extracted, changed: list[FactChange]) -> None:
        ids = {r.id for r in summary.rooms}
        if ex.scope_default is not None:
            for r in summary.rooms:
                cur = s.scope.get(r.id)
                if cur is None or cur.source != "user":
                    self._set_scope(s, r.id, ex.scope_default, "user", changed)
        for claim in ex.scope_rooms:
            if claim.room_id in ids:
                self._set_scope(s, claim.room_id, claim.state, "user", changed)

    def _record_fixtures(self, s: SessionState, summary: PlanSummary, limits: DesignLimits, ex: Extracted, message: str,
                         changed: list[FactChange]) -> None:
        talks = s.pending[:1] == ["fixtures"] or "fixtures" in s.facts or self._keywords.mentions_fixtures(message)
        if self._rules and not talks:
            return
        priced = {f.id for f in summary.fixtures if f.kind in limits.priced_fixtures}
        allowed = self._keywords.referred_fixtures(summary, limits.priced_fixtures, message) if self._rules else priced
        listed = [i for i in ex.fixtures_existing if i in priced and i in allowed]
        if ex.fixtures_all_exist:
            listed = sorted(priced)
        if listed or ex.fixtures_all_exist or ex.fixtures_none_exist:
            s.fixtures_existing = sorted(set(listed))
            value = "all exist" if ex.fixtures_all_exist else ("none exist" if not listed else "some exist")
            self._set_fact(s, "fixtures", value, "user", changed)

    def _record_giving_up(self, s: SessionState, summary: PlanSummary, ex: Extracted, message: str, limits: DesignLimits,
                          changed: list[FactChange]) -> None:
        names = {n for n, *_ in self._slots(limits)} | set(limits.default_markups) | {"markups"}
        gave_up: list[str] = []
        if self._keywords.says_unknown(message) if self._rules else ex.decide_for_me:
            gave_up = [x for x in ex.decide_for_me if x in names] or s.pending[:1]
        for slot in gave_up:
            if slot == "scope":
                for r in summary.rooms:
                    if r.id not in s.scope:
                        self._set_scope(s, r.id, "empty", "assumed", changed)
                continue
            for name, value in self._default_for(slot, limits):
                if name not in s.facts:
                    self._set_fact(s, name, value, "assumed", changed)

    def _default_for(self, slot: str, limits: DesignLimits) -> list[tuple[str, object]]:
        """What a slot is assumed to be when the person says "you decide"."""
        fixed = {"city": [("city", "Ho Chi Minh City")], "kind": [("kind", "apartment")], "work_type": [("work_type", "fit_out")],
                 "quality": [("quality", "normal")], "floor_height": [("floor_height", 3.0)], "storeys": [("storeys", 1)],
                 "structure_in_scope": [("structure_in_scope", False)], "mep_in_scope": [("mep_in_scope", True)],
                 "fixtures": [("fixtures", "none exist")], "markups": list(limits.default_markups.items())}
        return fixed.get(slot, [])

    def fingerprint(self, state: SessionState) -> str:
        blob = json.dumps({"f": {k: v.value for k, v in sorted(state.facts.items())},
                           "s": {k: v.state for k, v in sorted(state.scope.items())},
                           "o": dict(sorted(state.rate_overrides.items())), "x": sorted(state.excluded)}, sort_keys=True, default=str)
        return hashlib.sha1(blob.encode()).hexdigest()[:16]
