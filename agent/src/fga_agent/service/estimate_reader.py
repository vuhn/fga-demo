from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from dataclasses import fields

from ..domain.gateway import LanguageModel
from ..domain.model.call_context import CallContext
from ..domain.model.estimate_session import Message, SessionState
from ..domain.model.extracted import Extracted, RoomStateClaim
from ..domain.model.plan_summary import PlanSummary
from ..usecase.helper.claim_checker import ClaimChecker
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.keyword_reader import KeywordReader
from .prompts import estimate_questions, estimate_reading as prompt

MAX_TOKENS = 300  # a claims list is short; the cap stops a runaway generation instead of waiting minutes for it
# claim field -> (Extracted field, takes several values). A claim names a field; list fields take one claim per value.
CORE = {n: (n, False) for n in ("kind", "work_type", "city", "district", "quality", "floor_height", "storeys")}
SETTINGS = {**{n: (n, False) for n in ("structure_in_scope", "mep_in_scope", "vat_percent", "overhead_percent", "profit_percent",
                                         "design_percent", "contingency_percent", "use_typical_markups", "explain")},
            "decide_for_me": ("decide_for_me", True)}
SCOPE = {"scope_default": ("scope_default", False), "scope_room": ("scope_rooms", True)}
ITEMS = {"fixtures_all_exist": ("fixtures_all_exist", False), "fixtures_none_exist": ("fixtures_none_exist", False),
         "fixture_exists": ("fixtures_existing", True), "exclude_item": ("exclude_items", True), "include_item": ("include_items", True)}
# a certain reading of these replaces the model's even when the model set them (it splits a place badly)
CERTAIN_PLACE = ("city", "district")


class EstimateReaderService:
    """`rules=False` drops the keyword layer, leaving the model's verified reading alone (the eval measures what each rule buys)."""

    def __init__(self, llm: LanguageModel, catalog: DesignCatalogLoader, checker: ClaimChecker, keywords: KeywordReader,
                 rules: bool = True) -> None:
        self._llm, self._catalog, self._checker, self._keywords, self._rules = llm, catalog, checker, keywords, rules

    def read(self, ctx: CallContext, summary: PlanSummary, state: SessionState, message: str, history: list[Message]) -> Extracted:
        limits = self._catalog.limits(ctx)
        codes = [r.code for r in limits.rate_catalog]
        asked = prompt.question_asked(state, next((m.content for m in reversed(history) if m.role == "assistant"), ""))
        facts, msg, rooms = prompt.facts_text(state), prompt.message_block(message), prompt.room_list(summary)
        fixtures = prompt.fixture_list(summary, limits.priced_fixtures) or "(none)"
        jobs = [(prompt.core_system(), CORE, 10, f"Facts so far:\n{facts}{asked}{msg}"),
                (prompt.settings_system(), SETTINGS, 12, f"Facts so far:\n{facts}{asked}{msg}"),
                (prompt.scope_system(), SCOPE, 30, f"Rooms in the plan:\n{rooms}{asked}{msg}"),
                (prompt.items_system(codes), ITEMS, 40,
                 f"Rooms in the plan:\n{rooms}\n\nFixtures in the plan:\n{fixtures}{asked}{msg}")]
        with ThreadPoolExecutor(max_workers=len(jobs)) as pool:  # the four readings run at once
            claims = [f.result() for f in [pool.submit(self._ask, s, u, table, n) for s, table, n, u in jobs]]
        data = {}
        for (_, table, _, _), got in zip(jobs, claims):
            data.update(self._checker.verified(table, got, message, codes))
        ex = self._extracted(data)
        ex = self._decided(ex, state)
        return self._merged(ex, self._keywords.read(state, summary, message), message, summary) if self._rules else ex

    def _ask(self, system: str, user: str, table: dict, limit: int) -> list[dict]:
        schema = prompt.claims_schema(list(table), limit)
        return self._llm.answer_json(system, user, schema, max_tokens=MAX_TOKENS, think=False).get("claims", [])

    def _extracted(self, data: dict) -> Extracted:
        rooms = [RoomStateClaim(room_id, st) for room_id, st in data.pop("scope_rooms", [])]
        return Extracted(**data, scope_rooms=rooms)

    def _decided(self, ex: Extracted, state: SessionState) -> Extracted:
        """The model need not know slot names: giving up always means the question just asked."""
        if not ex.decide_for_me:
            return ex
        names = set(estimate_questions.SLOT_NAMES)
        if state.pending:
            given = [x if x in names else state.pending[0] for x in ex.decide_for_me][:1]
        else:
            given = [x for x in ex.decide_for_me if x in names]
        return Extracted(**{**self._values(ex), "decide_for_me": given})

    def _values(self, ex: Extracted) -> dict:
        return {f.name: getattr(ex, f.name) for f in fields(Extracted)}

    def _merged(self, ex: Extracted, certain: Extracted, message: str, summary: PlanSummary) -> Extracted:
        """The model's reading, with code's certain readings filling the gaps and the model's unprompted claims dropped."""
        blank = self._values(Extracted())
        data, sure = self._values(ex), {k: v for k, v in self._values(certain).items() if v != blank[k]}  # False is a reading
        for name, value in sure.items():
            if data[name] in (None, [], False) or name in CERTAIN_PLACE:
                data[name] = value
        if not self._keywords.names_removal(message):  # a small model must not take work out unprompted
            data["exclude_items"] = []
        if not self._keywords.names_addition(message):
            data["include_items"] = []
        if "scope_rooms" not in sure:  # the model's per-room list may only name rooms the message refers to
            ok = self._keywords.referred_rooms(summary, message)
            data["scope_rooms"] = [r for r in data["scope_rooms"] if r.room_id in ok]
        if "scope_default" not in sure and not self._keywords.names_condition(message):
            # only the weak word "exist(s)": it may name some rooms ("the toilets exist"), never all of them or a default
            rooms = data["scope_rooms"] if self._keywords.mentions_existing(message) else []
            every = {r.room_id for r in rooms} >= {r.id for r in summary.rooms}
            data["scope_default"], data["scope_rooms"] = None, ([] if every else rooms)
        return Extracted(**data)
