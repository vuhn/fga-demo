"""The cost-estimation agent. A turn is a fixed state machine in code (interview -> research ->
compute -> present -> follow-up); the LLM does three narrow jobs: read the user's message into facts
(structured), write the reply (streamed text) and, inside price_research, find prices. Which slots are
still missing is decided by code, so the interview is stable and testable. Nothing here does maths."""

from __future__ import annotations

import hashlib
import json
import os
import re
from dataclasses import dataclass
from typing import Callable, Literal

from pydantic import BaseModel, ConfigDict, Field

from . import progress
from .estimate import DEFAULT_MARKUPS, price
from .estimate_reading import ExistingReading, FactsReading, RoomStateIn, read_message
from .estimate_models import (
    Estimate, Fact, Message, Question, Quality, RateResult, RateSource, RoomScope, RoomState, SessionState, Takeoff, WorkType,
)
from .llm import LLMClient, LLMError
from .price_source import PriceSource, requests_for
from .domain.schemas import BuildingModel
from .takeoff import PRICED_FIXTURES, RATE_CATALOG, pending_fixtures, structure_in_scope, takeoff

MAX_RATES = 40
# Language-specific rules (English keyword guards) on top of the model's reading. The model's reading is always verified against the
# message; ESTIMATE_RULES=off drops the keyword layer, which is how eval/estimate_run.py measures what each rule still buys.
RULES = os.getenv("ESTIMATE_RULES", "on") != "off"
UNKNOWN_OPTION = "I don't know, you decide"
UNKNOWN_RE = re.compile(r"don'?t know|do not know|you decide|up to you|not sure|không biết|tùy|tuỳ|bạn quyết|bạn chọn", re.I)
TYPICAL_RE = re.compile(r"\btypical (rates|percent|markup)|\busual (rates|percent)|\bstandard rates\b|thông thường", re.I)
# a scope is accepted only when the message talks about the condition of rooms ("exist" alone does not)
SCOPE_WORDS_RE = re.compile(r"\b(empty|bare|blank|shell|raw|unfinished|finished|furnished|renovated|replace|redo|strip|demolish|brand new|new)\b|trống|chưa hoàn thiện|đã hoàn thiện", re.I)
KIND_RE = {"apartment": re.compile(r"\b(apartment|flat|condo|condominium)\b|căn hộ|chung cư", re.I),
           "house": re.compile(r"\b(house|villa|townhouse)\b|nhà phố|biệt thự", re.I)}
QUALITY_RE = {"typical": re.compile(r"\b(typical|economy|basic)\b", re.I), "normal": re.compile(r"\b(normal|standard|medium|mid-range)\b", re.I),
              "good": re.compile(r"\b(good|high quality)\b", re.I), "luxury": re.compile(r"\b(luxury|luxurious|premium)\b|cao cấp", re.I)}
# an explicit word for the work beats the one that is only implied, so renovation is tried first and "empty shell" (= fit-out) last
WORK_RE = {"renovation": re.compile(r"\brenovat|\bcải tạo", re.I), "new_build": re.compile(r"\bnew build\b|xây mới", re.I),
           "fit_out": re.compile(r"\bfit-?out\b|hoàn thiện nội thất|\b(empty|bare|raw|rough) shell\b", re.I)}
TYPE_WORDS: dict[str, re.Pattern] = {
    "bathroom": re.compile(r"\b(toilets?|wc|wcs|bath(room)?s?|restrooms?|lavator(y|ies))\b|vệ sinh|nhà tắm", re.I),
    "bedroom": re.compile(r"\bbed ?rooms?\b|phòng ngủ", re.I),
    "kitchen": re.compile(r"\bkitchens?\b|bếp", re.I),
    "living": re.compile(r"\bliving( rooms?)?\b|phòng khách", re.I),
    "dining": re.compile(r"\bdining\b|phòng ăn", re.I),
    "balcony": re.compile(r"\b(balcon(y|ies)|loggias?|terrace)\b|ban công", re.I),
    "laundry": re.compile(r"\blaundry\b", re.I),
    "storage": re.compile(r"\b(storage|store ?room|closet)\b", re.I),
    "study": re.compile(r"\b(study|office)\b", re.I),
    "entry": re.compile(r"\b(entry|foyer|hall)\b", re.I),
}
NUM_RE = re.compile(r"(\d+(?:[.,]\d+)?)")


class _Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Extracted(_Strict):
    """What the user's latest message states. Leave a field null unless the user said it."""

    kind: Literal["apartment", "house"] | None = None
    work_type: WorkType | None = None
    city: str | None = Field(default=None, max_length=60)
    district: str | None = Field(default=None, max_length=60)
    quality: Quality | None = None
    floor_height: float | None = Field(default=None, ge=2.2, le=6)
    storeys: int | None = Field(default=None, ge=1, le=12)
    structure_in_scope: bool | None = None
    mep_in_scope: bool | None = None
    scope_default: RoomState | None = None
    scope_rooms: list[RoomStateIn] = Field(default_factory=list, max_length=40)
    vat_percent: float | None = Field(default=None, ge=0, le=30)
    overhead_percent: float | None = Field(default=None, ge=0, le=30)
    profit_percent: float | None = Field(default=None, ge=0, le=30)
    design_percent: float | None = Field(default=None, ge=0, le=30)
    contingency_percent: float | None = Field(default=None, ge=0, le=30)
    fixtures_existing: list[str] = Field(default_factory=list, max_length=60)  # fixture ids the person says are already installed
    fixtures_all_exist: bool = False
    fixtures_none_exist: bool = False
    exclude_items: list[str] = Field(default_factory=list, max_length=30)  # rate codes to take out of the estimate
    include_items: list[str] = Field(default_factory=list, max_length=30)  # rate codes to put back
    use_typical_markups: bool = False
    decide_for_me: list[str] = Field(default_factory=list, max_length=12)
    explain: str | None = Field(default=None, max_length=80)


def _slot_default(name: str, model: BuildingModel) -> list[tuple[str, object, str]]:
    return {
        "city": [("city", "Ho Chi Minh City", "assumed")],
        "kind": [("kind", "apartment", "assumed")],
        "work_type": [("work_type", "fit_out", "assumed")],
        "quality": [("quality", "normal", "assumed")],
        "floor_height": [("floor_height", 3.0, "assumed")],
        "storeys": [("storeys", 1, "assumed")],
        "structure_in_scope": [("structure_in_scope", False, "assumed")],
        "mep_in_scope": [("mep_in_scope", True, "assumed")],
        "fixtures": [("fixtures", "none exist", "assumed")],
        "markups": [(k, v, "assumed") for k, v in DEFAULT_MARKUPS.items()],
    }.get(name, [])


@dataclass
class Slot:
    name: str
    applies: Callable[[SessionState, BuildingModel], bool]
    done: Callable[[SessionState, BuildingModel], bool]
    ask: Callable[[SessionState, BuildingModel], Question]


def _has(*names: str) -> Callable[[SessionState, BuildingModel], bool]:
    return lambda s, m: all(n in s.facts for n in names)


def _always(s, m):
    return True


def _scope_done(s: SessionState, m: BuildingModel) -> bool:
    return all(r.id in s.scope for r in m.rooms)


def _scope_question(s: SessionState, m: BuildingModel) -> Question:
    unset = [r for r in m.rooms if r.id not in s.scope]
    names = ", ".join(r.name for r in unset[:12])
    opts = ["Everything is empty"]
    if any(r.type == "bathroom" for r in unset):
        opts.append("Everything is empty except the bathrooms")
    opts.append("Everything already exists")
    return Question(id="scope", text=f"Is the space empty, or do some rooms or items already exist? I see: {names}. Tell me which rooms are already finished (if any).",
                    options=opts)


def _fixtures_question(s: SessionState, m: BuildingModel) -> Question:
    rooms = {r.id: r.name for r in m.rooms}
    items = [f"{f.kind.replace('_', ' ')} ({rooms.get(f.room_id, f.room_id)})" for f in pending_fixtures(m, s)]
    return Question(id="fixtures", text="The plan shows these fixtures: " + ", ".join(items[:14]) + (" …" if len(items) > 14 else "")
                    + ". A floor plan does not say what is installed, so I assume none of them exist yet and will price supplying and fitting each. "
                    "Which of them already exist?", options=["None of them exist", "All of them already exist"])


def _markups_question(s: SessionState, m: BuildingModel) -> Question:
    missing = [n for n in DEFAULT_MARKUPS if n not in s.facts]
    labels = {"overhead_percent": "site overhead and temporary works (typically 5-8%)", "profit_percent": "contractor profit (5-10%)",
              "design_percent": "design, supervision and permits (3-8%)", "contingency_percent": "contingency (5-10%)",
              "vat_percent": "VAT (the rate in force)"}
    return Question(id="markups", text="Which percentages should I add on top of the direct cost: " + "; ".join(labels[n] for n in missing) + "? I can use the typical mid-range values if you agree.",
                    options=["Use typical rates"])


SLOTS: list[Slot] = [
    Slot("city", _always, _has("city"), lambda s, m: Question(id="city", text="Where is the project located (city and district)? Prices differ by region.",
                                                                options=["Ho Chi Minh City", "Hanoi", "Da Nang"])),
    Slot("kind", _always, _has("kind"), lambda s, m: Question(id="kind", text="Is this an apartment or a house?", options=["Apartment", "House"])),
    Slot("scope", _always, _scope_done, _scope_question),
    Slot("work_type", _always, _has("work_type"), lambda s, m: Question(
        id="work_type", text="What kind of work is this: a new build, fitting out an empty shell, or renovating an existing home?",
        options=["New build", "Fit-out of an empty shell", "Renovation"])),
    Slot("fixtures", lambda s, m: bool(pending_fixtures(m, s)) or "fixtures" in s.facts, _has("fixtures"), _fixtures_question),
    Slot("quality", _always, _has("quality"), lambda s, m: Question(
        id="quality", text="What quality level do you expect: typical (economy), normal, good or luxury?", options=["Typical", "Normal", "Good", "Luxury"])),
    Slot("structure_in_scope", lambda s, m: s.facts.get("work_type") is not None and s.facts["work_type"].value == "renovation",
         _has("structure_in_scope"), lambda s, m: Question(
             id="structure_in_scope", text="Does the work include structure (foundation, columns, slabs), or finishes only?",
             options=["Finishes only", "Include structure"])),
    Slot("storeys", lambda s, m: structure_in_scope(s), _has("storeys"), lambda s, m: Question(
        id="storeys", text="How many storeys does the structure have?", options=["1", "2", "3"])),
    Slot("floor_height", _always, _has("floor_height"), lambda s, m: Question(
        id="floor_height", text="What is the floor-to-ceiling height? The plan does not show it and it drives wall and finish areas.", options=["2.7 m", "3.0 m", "3.3 m"])),
    Slot("mep_in_scope", _always, _has("mep_in_scope"), lambda s, m: Question(
        id="mep_in_scope", text="Should electrical and plumbing work be included?", options=["Yes, include electrical and plumbing", "No, exclude them"])),
    Slot("markups", _always, lambda s, m: all(n in s.facts for n in DEFAULT_MARKUPS), _markups_question),
]


SLOT_NAMES = {sl.name for sl in SLOTS} | set(DEFAULT_MARKUPS) | {"markups"}


def missing_slots(state: SessionState, model: BuildingModel) -> list[Slot]:
    return [sl for sl in SLOTS if sl.applies(state, model) and not sl.done(state, model)]


# ---------- applying what the user said ----------


def _set_fact(state: SessionState, name: str, value, source: str, changed: list[tuple[str, object, str]]) -> None:
    cur = state.facts.get(name)
    if cur is not None and cur.value == value and cur.source == source:
        return
    state.facts[name] = Fact(value=value, source=source)  # type: ignore[arg-type]
    changed.append((name, value, source))


def _set_scope(state: SessionState, room_id: str, value: str, source: str, changed: list) -> None:
    cur = state.scope.get(room_id)
    if cur is not None and cur.state == value and cur.source == source:
        return
    state.scope[room_id] = RoomScope(state=value, source=source)  # type: ignore[arg-type]
    changed.append((f"scope.{room_id}", value, source))


def apply_extraction(state: SessionState, model: BuildingModel, ex: Extracted, message: str) -> list[tuple[str, object, str]]:
    """Merge the extraction into the state; returns (name, value, source) for every change."""
    changed: list[tuple[str, object, str]] = []
    for name in ("kind", "work_type", "city", "district", "quality", "floor_height", "storeys", "structure_in_scope", "mep_in_scope",
                 "vat_percent", "overhead_percent", "profit_percent", "design_percent", "contingency_percent"):
        v = getattr(ex, name)
        if v is not None:
            _set_fact(state, name, v, "user", changed)
    valid = set(RATE_CATALOG)
    drop = [c for c in ex.exclude_items if c in valid]
    back = [c for c in ex.include_items if c in valid]
    if drop or back:
        before = set(state.excluded)
        state.excluded = sorted((before | set(drop)) - set(back))
        if set(state.excluded) != before:
            changed.append(("excluded", ", ".join(state.excluded) or "nothing", "user"))
    # A small model sometimes sets these flags unprompted. The words must really be in the message, or nothing is assumed.
    if (not RULES or TYPICAL_RE.search(message)) and (ex.use_typical_markups or (RULES and state.pending[:1] == ["markups"])):
        for k, v in DEFAULT_MARKUPS.items():
            if k not in state.facts or state.facts[k].source != "user":
                _set_fact(state, k, v, "assumed", changed)
    ids = {r.id for r in model.rooms}
    if ex.scope_default is not None:
        for r in model.rooms:
            cur = state.scope.get(r.id)
            if cur is None or cur.source != "user":
                _set_scope(state, r.id, ex.scope_default, "user", changed)
    for rs in ex.scope_rooms:
        if rs.room_id in ids:
            _set_scope(state, rs.room_id, rs.state, "user", changed)
    if not RULES or state.pending[:1] == ["fixtures"] or "fixtures" in state.facts or re.search(r"fixture|toilet|wc|basin|shower|bathtub|sink|wardrobe|cabinet|kitchen", message, re.I):
        fx_ids = {f.id for f in model.fixtures if f.kind in PRICED_FIXTURES}
        ok = _referred_fixtures(model, message) if RULES else fx_ids
        listed = [i for i in ex.fixtures_existing if i in fx_ids and i in ok]
        if ex.fixtures_all_exist:
            listed = sorted(fx_ids)
        if listed or ex.fixtures_all_exist or ex.fixtures_none_exist:
            state.fixtures_existing = sorted(set(listed))
            _set_fact(state, "fixtures", "all exist" if ex.fixtures_all_exist else ("none exist" if not listed else "some exist"), "user", changed)
    gave_up: list[str] = []
    if UNKNOWN_RE.search(message) if RULES else ex.decide_for_me:
        gave_up = [x for x in ex.decide_for_me if x in SLOT_NAMES] or state.pending[:1]
    for slot in gave_up:
        if slot == "scope":
            for r in model.rooms:
                if r.id not in state.scope:
                    _set_scope(state, r.id, "empty", "assumed", changed)
            continue
        for name, value, src in _slot_default(slot, model):
            if name not in state.facts:
                _set_fact(state, name, value, src, changed)
    # a new build always includes structure; asking would be noise
    if state.facts.get("work_type") is not None and state.facts["work_type"].value == "new_build":
        state.facts.pop("structure_in_scope", None)
    return changed


def fingerprint(state: SessionState) -> str:
    blob = json.dumps({"f": {k: v.value for k, v in sorted(state.facts.items())},
                       "s": {k: v.state for k, v in sorted(state.scope.items())},
                       "o": dict(sorted(state.rate_overrides.items())), "x": sorted(state.excluded)}, sort_keys=True, default=str)
    return hashlib.sha1(blob.encode()).hexdigest()[:16]


# ---------- the turn ----------


class TurnOut(_Strict):
    state: SessionState
    reply: str
    question: Question | None = None
    estimate: Estimate | None = None


def _room_list(model: BuildingModel) -> str:
    rooms = "\n".join(f"- {r.id}: {r.name} ({r.type}, {r.net_area:.1f} m²)" for r in model.rooms) or "(no rooms)"
    fx = "\n".join(f"- {f.id}: {f.kind} in {f.room_id}" for f in model.fixtures if f.kind in PRICED_FIXTURES)
    return rooms + ("\n\nFixtures in the plan:\n" + fx if fx else "")


def _facts_text(state: SessionState) -> str:
    return "\n".join(f"- {k} = {v.value} ({v.source})" for k, v in state.facts.items()) or "(none yet)"


ITEM_WORDS: dict[str, tuple[str, ...]] = {
    "doors?|cửa đi|cửa": ("door_interior", "door_entrance"), "entrance doors?|front doors?|main doors?|cửa chính|cửa ra vào": ("door_entrance",),
    "interior doors?": ("door_interior",), "windows?|cửa sổ": ("window",), "paint(ing)?|sơn": ("paint",),
    "floor(ing)? tiles?|floor(ing)?|lát nền": ("floor_tile",), "wall tiles?|ốp tường": ("wall_tile",),
    "ceilings?|trần": ("ceiling",), "plaster(ing)?|trát": ("plaster",), "skirting": ("skirting",),
    "waterproofing|chống thấm": ("waterproofing",), "demolition": ("demolition_finish",),
    "electrical|wiring": ("electrical_point",), "plumbing": ("plumbing_point",),
    "toilets?|wc": ("fixture_toilet",), "basins?": ("fixture_basin",), "showers?": ("fixture_shower",),
    "bath ?tubs?": ("fixture_bathtub",), "sinks?": ("fixture_sink",), "kitchen (counter|cabinet)s?": ("fixture_kitchen_counter",),
    "wardrobes?": ("fixture_wardrobe",),
}
DROP_RE = re.compile(r"\b(remove|exclude|drop|delete|skip|take out|leave out|no need for|don'?t (include|price|count)|without)\b|\bno cost\b|"
                     r"\b(already|is|are) (exist|exists|existing|installed|there|done|built)\b|\balready (have|has)\b", re.I)
# "except the entrance door", "only the entrance door exists", "the entrance door is in place": the work on that item is already done
EXCEPT_RE = re.compile(r"\b(?:except(?: for)?|apart from|other than|but for|besides)\s+(?:the\s+)?([^.;\n]+)|\btrừ\s+([^.;\n]+)", re.I)
ONLY_RE = re.compile(r"\bonly (?:the )?([^.;\n]+?)\s+(?:is|are|exists?|installed|there|in place|done)\b", re.I)
IN_PLACE_RE = re.compile(r"\b(?:in place|installed|existing|exist|exists|done|built|there)\b", re.I)
# the items an "except ..." may take out: openings only; for fixtures and rooms an exception is about their condition, handled elsewhere
OPENING_CODES = {"door_interior", "door_entrance", "window"}
ADD_RE = re.compile(r"\b(add|include|put back|restore|bring back)\b.*\b(again|back)\b|\b(add|include) (the )?[a-z ]+ (back|again)\b", re.I)


def exception_items(message: str) -> list[str]:
    """Opening items the person says already exist by naming them as the exception or the only ones ("fixtures are empty, except the
    entrance door"). Fixtures and rooms are left to their own slots."""
    clauses = [g for m in EXCEPT_RE.finditer(message) for g in m.groups() if g] + [m.group(1) for m in ONLY_RE.finditer(message)]
    out: list[str] = []
    for c in clauses:
        out += [i for i in _items_in(c) if i in OPENING_CODES and i not in out]
    return out


def _items_in(message: str) -> list[str]:
    """Rate codes a message names. Longer phrases claim their words first ("entrance door" before "door")."""
    low, found = message.lower(), []
    for pat in sorted(ITEM_WORDS, key=len, reverse=True):
        if re.search(rf"\b({pat})\b", low):
            if any(c in found for c in ITEM_WORDS[pat]) and pat in ("doors?|cửa đi|cửa",):
                continue
            found += [c for c in ITEM_WORDS[pat] if c not in found]
    return found


CITY_RE: dict[str, re.Pattern] = {
    "Ho Chi Minh City": re.compile(r"\b(ho chi minh|hcmc?|saigon|sài gòn|tp\.? ?hcm)\b", re.I),
    "Hanoi": re.compile(r"\b(hanoi|ha noi|hà nội)\b", re.I),
    "Da Nang": re.compile(r"\b(da nang|đà nẵng)\b", re.I),
}
MEP_RE = re.compile(r"\b(include|including|with|exclude|excluding|without|skip|no)\s+(?:the\s+)?(?:mep|electrical|plumbing)\b", re.I)
HEIGHT_RE = re.compile(r"(?:floor|ceiling|storey|story) height(?: is| of)?\s*(?:about |around )?(\d+(?:[.,]\d+)?)\s*m\b", re.I)


def fallback_extraction(state: SessionState, model: BuildingModel, message: str) -> dict:
    """What plain code can read from the message with certainty: the agent's own chips, a keyword for the kind or
    quality, and a direct answer to the question just asked. The LLM's reading fills in the rest."""
    out: dict = {}
    m = message.strip()
    low = m.lower()
    for k, rx in KIND_RE.items():
        if rx.search(m):
            out["kind"] = k
            break
    if state.pending[:1] == ["quality"] or "quality" in low:
        for k, rx in QUALITY_RE.items():
            if rx.search(m):
                out["quality"] = k
                break
    for city, rx in CITY_RE.items():
        if rx.search(m):
            out["city"] = city
            break
    if hm := HEIGHT_RE.search(m):
        v = float(hm.group(1).replace(",", "."))
        if 2.2 <= v <= 6:
            out["floor_height"] = v
    for k, rx in WORK_RE.items():
        if rx.search(m):
            out["work_type"] = k
            break
    if re.search(r"\b(finishes only|finishing only|without structure|no structure|excluding structure|exclude structure)\b", low):
        out["structure_in_scope"] = False
    elif re.search(r"\b(include|including|with) (the )?structure\b", low):
        out["structure_in_scope"] = True
    items = _items_in(m)
    if items and ADD_RE.search(m) and (state.estimate is not None or state.excluded):
        out["include_items"] = items
    elif items and DROP_RE.search(m):
        out["exclude_items"] = items
    if not out.get("exclude_items") and (ex := exception_items(m)):
        out["exclude_items"] = ex
    if mm := MEP_RE.search(m):
        out["mep_in_scope"] = not re.match(r"(exclude|without|no|skip)", mm.group(1), re.I)
    asked = state.pending[0] if state.pending else ""
    if asked == "work_type":
        for k, rx in WORK_RE.items():
            if rx.search(m):
                out["work_type"] = k
                break
    elif asked == "city" and 1 < len(m) <= 60 and not UNKNOWN_RE.search(m):
        parts = [x.strip() for x in m.split(",") if x.strip()]
        known = out.get("city")  # a city we recognise wins wherever it stands in "District 7, Ho Chi Minh City"
        if known:
            rest = [x for x in parts if not CITY_RE[known].search(x)]
            if rest:
                out["district"] = rest[0]
        elif parts:
            out["city"] = parts[0]
            if len(parts) > 1:
                out["district"] = parts[1]
    elif asked in ("floor_height", "storeys") and (n := NUM_RE.search(m)):
        v = float(n.group(1).replace(",", "."))
        if asked == "floor_height" and 2.2 <= v <= 6:
            out["floor_height"] = v
        elif asked == "storeys" and v == int(v) and 1 <= v <= 12:
            out["storeys"] = int(v)
    elif asked in ("mep_in_scope", "structure_in_scope"):
        if re.match(r"\s*(yes|include)", low):
            out[asked] = True
        elif re.match(r"\s*(no|exclude|finishes only)", low):
            out[asked] = False
    if asked == "fixtures" or "fixture" in low:
        if low.startswith("none of them") or re.search(r"\bfixtures?\b.{0,20}\b(empty|none|bare|absent)\b", low) or re.search(r"\bnone\b.*\b(exist|installed)|nothing (is )?installed", low):
            out["fixtures_none_exist"] = True
        elif low.startswith("all of them") or re.search(r"\ball\b.*\b(exist|installed)", low):
            out["fixtures_all_exist"] = True
    if low.startswith("everything is empty") or low.startswith("everything already exists"):
        out["scope_default"] = "empty" if "empty" in low[:22] else "existing"
        if "except the bathrooms" in low:
            out["scope_rooms"] = [{"room_id": r.id, "state": "existing"} for r in model.rooms if r.type == "bathroom"]
    return out


def _referred_rooms(model: BuildingModel, message: str) -> set[str]:
    """Rooms the message actually refers to: by their own name ("Bedroom 3", "Master wc"), else by their type ("the toilets")."""
    low = message.lower()
    named = {r.id for r in model.rooms if (toks := re.findall(r"[^\W_]+", r.name.lower())) and all(t in low for t in toks)}
    if named:
        return named
    return {r.id for r in model.rooms if r.type in TYPE_WORDS and TYPE_WORDS[r.type].search(message)}


FIXTURE_WORDS: dict[str, re.Pattern] = {
    "toilet": re.compile(r"\b(toilets?|wc|water closets?)\b", re.I), "basin": re.compile(r"\b(basins?|washbasins?|lavator(y|ies))\b", re.I),
    "shower": re.compile(r"\bshowers?\b", re.I), "bathtub": re.compile(r"\b(bath ?tubs?|tubs?)\b", re.I),
    "sink": re.compile(r"\bsinks?\b", re.I), "kitchen_counter": re.compile(r"\b(counters?|kitchen cabinets?|cabinets?|countertops?)\b", re.I),
    "wardrobe": re.compile(r"\bwardrobes?\b", re.I),
}


def _referred_fixtures(model: BuildingModel, message: str) -> set[str]:
    """Fixtures the message refers to: by kind ("the toilet"), narrowed to a named room ("... in the bath") when it names one."""
    kinds = {k for k, rx in FIXTURE_WORDS.items() if rx.search(message)}
    low = message.lower()
    named = {r.id for r in model.rooms if (toks := re.findall(r"[^\W_]+", r.name.lower())) and all(t in low for t in toks)}
    return {f.id for f in model.fixtures if f.kind in PRICED_FIXTURES and (not kinds or f.kind in kinds) and (not named or f.room_id in named)
            and (kinds or named)}


def _merge(ex: Extracted, fb: dict, message: str, model: BuildingModel) -> Extracted:
    """The LLM's reading, with code's certain readings filling the gaps and the LLM's unprompted scope dropped."""
    data = ex.model_dump()
    for k, v in fb.items():
        if data.get(k) in (None, [], False) or k in ("city", "district"):  # code's reading of a place is certain (the model splits it badly)
            data[k] = v
    if not (DROP_RE.search(message) or exception_items(message)):  # a small model must not take work out unprompted
        data["exclude_items"] = []
    if not re.search(r"\b(add|include|put|restore|bring)\b", message, re.I):
        data["include_items"] = []
    if "scope_rooms" not in fb:  # the model's per-room list may only name rooms the message refers to
        ok = _referred_rooms(model, message)
        data["scope_rooms"] = [r for r in data["scope_rooms"] if r["room_id"] in ok]
    if "scope_default" not in fb and not SCOPE_WORDS_RE.search(message):
        # only the weak word "exist(s)": it may name some rooms ("the toilets exist"), never all of them or a default
        rooms = data["scope_rooms"] if re.search(r"\bexist", message, re.I) else []
        every = {r["room_id"] for r in rooms} >= {r.id for r in model.rooms}
        data["scope_default"], data["scope_rooms"] = None, ([] if every else rooms)
    return Extracted.model_validate(data)


def _merged(facts: FactsReading, existing: ExistingReading) -> Extracted:
    return Extracted.model_validate({**facts.model_dump(), **existing.model_dump()})


def _extract(llm: LLMClient, model: BuildingModel, state: SessionState, message: str, history: list[Message]) -> Extracted:
    asked = ""
    if state.pending:
        asked = f"\nQuestion just asked (slot {state.pending[0]}): " + next(
            (m.content for m in reversed(history) if m.role == "assistant"), "")[:600]
    rooms, _, fixtures = _room_list(model).partition("\n\nFixtures in the plan:\n")
    with progress.quiet_thinking():
        facts, existing = read_message(llm, {"facts": _facts_text(state), "asked": asked, "rooms": rooms, "fixtures": fixtures}, message)
    ex = _merged(facts, existing)
    if ex.decide_for_me:  # the model need not know slot names: giving up always means the question just asked
        ex = ex.model_copy(update={"decide_for_me": [x if x in SLOT_NAMES else state.pending[0] for x in ex.decide_for_me][:1] if state.pending
                                   else [x for x in ex.decide_for_me if x in SLOT_NAMES]})
    return _merge(ex, fallback_extraction(state, model, message), message, model) if RULES else ex


REPLY_SYSTEM = (
    "You are a cost-estimation assistant talking with a person about the floor plan they uploaded. Write the next chat message: short, plain, "
    "friendly, no headings and no tables. Reply in the language of the person's last message. Do what the instruction says and nothing else. "
    "Never invent or change a number; use only the figures you are given."
)


def _reply(llm: LLMClient, instruction: str, context: dict, fallback: str) -> str:
    """Stream the reply as `token` events. If the model fails before any text, the fallback is sent instead."""
    parts: list[str] = []
    try:
        for piece in llm.stream_text(REPLY_SYSTEM, f"Instruction: {instruction}\n\nData:\n{json.dumps(context, ensure_ascii=False)}"):
            parts.append(piece)
            progress.emit_event("token", {"text": piece})
    except LLMError:
        if parts:
            raise
    text = "".join(parts)
    if not text.strip():
        parts = [fallback]
        progress.emit_event("token", {"text": fallback})
        text = fallback
    return text


def _money(v: float) -> str:
    return f"{v:,.0f} VND"


def _summary(est: Estimate) -> dict:
    top = sorted(est.packages.items(), key=lambda kv: -kv[1]["base"])[:4]
    none = est.total.get("base", 0) == 0
    return {**({"note": "No unit prices could be found, so nothing is in the total. Say that plainly, say why (see unpriced and warnings), and offer to take the person's own rates."} if none else {}),
            "total_low": _money(est.total.get("low", 0)), "total_base": _money(est.total.get("base", 0)), "total_high": _money(est.total.get("high", 0)),
            "largest_packages": [{"package": k, "base": _money(v["base"])} for k, v in top],
            "assumptions": est.assumptions[:6], "unpriced": [w for w in est.warnings if w.startswith("No price")][:6], "warnings": est.warnings[:4]}


def _matching(state: SessionState, code: str, region: str, quality: str) -> RateResult | None:
    spec = RATE_CATALOG[code]
    q = quality if spec.quality_sensitive else ""
    for r in state.rates:
        if r.code == code and r.region == region and r.quality == q and r.confidence != "none":
            return r
    return None


def _user_rate(code: str, value: float) -> RateResult:
    return RateResult(code=code, low=value, base=value, high=value, unit=RATE_CATALOG[code].unit, confidence="user",
                      sources=[RateSource(url="", quote="entered by the user", retrieved_at="")], note="entered by the user")


def run_turn(llm: LLMClient, source: PriceSource, model: BuildingModel, state_in: SessionState, message: str,
             history: list[Message] | None = None, rate_overrides: dict[str, float] | None = None) -> TurnOut:
    state = state_in.model_copy(deep=True)
    history = history or []
    state.turns += 1
    if rate_overrides:
        for code, v in rate_overrides.items():
            if code in RATE_CATALOG and v > 0:
                state.rate_overrides[code] = v
    explain = None
    changes: list[tuple[str, object, str]] = []
    if message.strip():
        progress.emit("understand", "reading your message")
        ex = _extract(llm, model, state, message, history)
        changes = apply_extraction(state, model, ex, message)
        for name, value, src in changes:
            progress.emit_event("fact", {"name": name, "value": value, "source": src})
        explain = ex.explain

    missing = missing_slots(state, model)
    state.pending = [sl.name for sl in missing]
    if missing:
        state.status = "interviewing"
        q = missing[0].ask(state, model)
        if q.allow_unknown:
            q.options = [*q.options, UNKNOWN_OPTION]
        reply = _reply(llm, "Ask the person this one question (you may add a short reason). Ask nothing else.",
                       {"question": q.text, "known": {k: v.value for k, v in state.facts.items()}}, q.text)
        progress.emit_event("question", q.model_dump(mode="json"))
        return TurnOut(state=state, reply=reply, question=q)

    tk = takeoff(model, state)
    if tk.status == "unsupported":
        state.status = "failed"
        reply = _reply(llm, "Tell the person this plan cannot be estimated and why.", {"reason": tk.warnings}, tk.warnings[0])
        return TurnOut(state=state, reply=reply)

    fp = fingerprint(state)
    if state.estimate is None or fp != state.fingerprint:
        est = _price(source, model, state, tk)
        state.estimate, state.fingerprint, state.status = est, fp, "ready"
        progress.emit_event("estimate", est.model_dump(mode="json"))
        s = _summary(est)
        fallback = f"Estimated total {s['total_base']} (range {s['total_low']} to {s['total_high']}, VAT included)."
        what = [f"{n} = {v}" for n, v, _ in changes]
        if what:
            s = {**s, "changes_applied_this_turn": what}
        reply = _reply(llm, "Present the estimate: the base total and range, the biggest packages, what is assumed or unpriced, "
                            "and offer to change any answer or to explain a line. Use the figures exactly as given. If "
                            "changes_applied_this_turn is given, say first what was changed, using only that list.", s, fallback)
        return TurnOut(state=state, reply=reply, estimate=est)

    state.status = "ready"
    est = state.estimate
    line = _find_line(est, explain) if explain else None
    if line is not None:
        rate = next((r for r in est.rates if r.code == line.code), None)
        ctx = {"line": line.model_dump(mode="json"), "sources": [s.model_dump() for s in (rate.sources if rate else [])][:4]}
        reply = _reply(llm, "Explain this cost line: the quantity and how it was measured, the rate and where it comes from. Be concrete.", ctx,
                       f"{line.description}: {line.quantity:g} {line.unit}. Basis: {line.basis}")
    else:
        reply = _reply(llm, "Nothing in the estimate was changed by this message. Never say or imply that anything was updated, removed, "
                            "recalculated or changed. Answer the question if it is one; otherwise say plainly that you could not apply it and what can "
                            "be changed: take an item out or put it back (for example 'the doors already exist'), set a unit rate, change an answer "
                            "(quality, city, scope, markups).", {"message": message, **_summary(est)},
                       "I did not change anything. I can take an item out (for example \"the doors already exist\"), set a unit rate, or change an answer such as quality, city or markups.")
    return TurnOut(state=state, reply=reply)


def _find_line(est: Estimate, key: str):
    k = key.casefold()
    for ln in est.lines:
        if k == ln.code or k in ln.description.casefold() or k in ln.code.replace("_", " "):
            return ln
    return None


def _price(source: PriceSource, model: BuildingModel, state: SessionState, tk: Takeoff) -> Estimate:
    region = str(state.facts["city"].value)
    quality = str(state.facts["quality"].value)
    rates: dict[str, RateResult] = {}
    todo: list[str] = []
    for code in tk.codes():
        if code in state.rate_overrides:
            rates[code] = _user_rate(code, state.rate_overrides[code])
            continue
        have = _matching(state, code, region, quality)
        if have is not None:
            rates[code] = have
        else:
            todo.append(code)
    if todo:
        state.status = "researching"
        todo = todo[:MAX_RATES]
        progress.emit("research", f"looking up {len(todo)} unit price(s) in {region}")

        def got(r: RateResult) -> None:
            progress.emit_event("rate", r.model_dump(mode="json"))

        for res in source.rates(requests_for(todo, region, quality), got):
            rates[res.code] = res
            state.rates = [x for x in state.rates if x.code != res.code] + [res]
    progress.emit("compute", "computing quantities and totals")
    return price(tk, rates, state)
