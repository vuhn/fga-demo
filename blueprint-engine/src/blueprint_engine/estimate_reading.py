"""Reading the person's message in any language.

Two narrow LLM calls run side by side: one reads the facts (place, quality, scope of work, percentages), the other
reads what already exists (items, fixtures, the condition of rooms). The model returns only what the message states, as
short claims: {quote, field, value}. The quote is the message's own words, copied. Code checks that the quote is really in
the message (accent- and case-insensitive, with a small tolerance for a model that copies loosely) and that the value is
valid for its field, and drops any claim that fails. That check needs no word lists, so it works the same in English,
Vietnamese or Japanese, and it stops a small model from inventing values nobody said. A short list of claims is also much
cheaper to generate than a form with twenty empty fields. Nothing here knows a language."""

from __future__ import annotations

import contextvars
import difflib
import unicodedata
from concurrent.futures import ThreadPoolExecutor
from typing import Literal, get_args

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter, ValidationError, create_model

from .estimate_models import Quality, RoomState, WorkType
from .takeoff import RATE_CATALOG


class _Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


class RoomStateIn(_Strict):
    room_id: str
    state: RoomState


class FactsReading(_Strict):
    """The facts the person's latest message states. Null unless the person said it."""

    kind: Literal["apartment", "house"] | None = None
    work_type: WorkType | None = None
    city: str | None = Field(default=None, max_length=60)
    district: str | None = Field(default=None, max_length=60)
    quality: Quality | None = None
    floor_height: float | None = Field(default=None, ge=2.2, le=6)
    storeys: int | None = Field(default=None, ge=1, le=12)
    structure_in_scope: bool | None = None
    mep_in_scope: bool | None = None
    vat_percent: float | None = Field(default=None, ge=0, le=30)
    overhead_percent: float | None = Field(default=None, ge=0, le=30)
    profit_percent: float | None = Field(default=None, ge=0, le=30)
    design_percent: float | None = Field(default=None, ge=0, le=30)
    contingency_percent: float | None = Field(default=None, ge=0, le=30)
    use_typical_markups: bool = False
    decide_for_me: list[str] = Field(default_factory=list, max_length=12)
    explain: str | None = Field(default=None, max_length=80)


class ExistingReading(_Strict):
    """What the person says already exists, is empty, or is to be left out of the estimate or put back in."""

    scope_default: RoomState | None = None
    scope_rooms: list[RoomStateIn] = Field(default_factory=list, max_length=40)
    fixtures_existing: list[str] = Field(default_factory=list, max_length=60)
    fixtures_all_exist: bool = False
    fixtures_none_exist: bool = False
    exclude_items: list[str] = Field(default_factory=list, max_length=30)
    include_items: list[str] = Field(default_factory=list, max_length=30)


# claim field -> (reading field, is it a list). A claim names a field; list fields take one claim per value.
CORE_TABLE: dict[str, tuple[str, bool]] = {n: (n, False) for n in ("kind", "work_type", "city", "district", "quality", "floor_height", "storeys")}
SETTINGS_TABLE: dict[str, tuple[str, bool]] = {
    **{n: (n, False) for n in ("structure_in_scope", "mep_in_scope", "vat_percent", "overhead_percent", "profit_percent", "design_percent",
                              "contingency_percent", "use_typical_markups", "explain")},
    "decide_for_me": ("decide_for_me", True),
}
SCOPE_TABLE: dict[str, tuple[str, bool]] = {"scope_default": ("scope_default", False), "scope_room": ("scope_rooms", True)}
ITEMS_TABLE: dict[str, tuple[str, bool]] = {
    "fixtures_all_exist": ("fixtures_all_exist", False), "fixtures_none_exist": ("fixtures_none_exist", False),
    "fixture_exists": ("fixtures_existing", True), "exclude_item": ("exclude_items", True), "include_item": ("include_items", True),
}


def _claims_model(name: str, table: dict[str, tuple[str, bool]], limit: int) -> type[BaseModel]:
    """A Claims schema whose `field` may only name this reader's fields. `quote` comes first so the value is decided from the words."""
    field_type = Literal[tuple(table)]  # type: ignore[valid-type]
    claim = create_model(name + "Claim", __config__=ConfigDict(extra="forbid"), quote=(str, Field(max_length=160)),
                         field=(field_type, ...), value=(str, Field(max_length=80)))
    return create_model(name + "Claims", __config__=ConfigDict(extra="forbid"), claims=(list[claim], Field(default_factory=list, max_length=limit)))


CoreClaims = _claims_model("Core", CORE_TABLE, 10)
SettingsClaims = _claims_model("Settings", SETTINGS_TABLE, 12)
ScopeClaims = _claims_model("Scope", SCOPE_TABLE, 30)
ItemsClaims = _claims_model("Items", ITEMS_TABLE, 40)
CLAIM_TABLES = {CoreClaims: CORE_TABLE, SettingsClaims: SETTINGS_TABLE, ScopeClaims: SCOPE_TABLE, ItemsClaims: ITEMS_TABLE}

_COMMON = (
    "Read ONE message from a person who is getting a construction cost estimate for a floor plan. The message may be in any language "
    "(English, Vietnamese, Japanese, ...): understand it in that language, but write each value in the fixed English form below.\n"
    "Return `claims`, one for each thing the message states: {quote, field, value}. quote = the exact words of the message that say it, "
    "copied word for word in the message's own language. If the message states nothing about these fields, return no claims. "
    "Never guess and never add a claim the words do not support.\n"
)

CORE_SYSTEM = _COMMON + (
    "Fields and values:\n"
    "- kind: apartment | house. work_type: new_build | fit_out (finishing an empty shell) | renovation.\n"
    "- city: only the city, in Latin letters ('Ho Chi Minh City', 'Da Nang'). district: the district or ward ('District 7', 'Thu Duc'), never in city.\n"
    "- quality: typical | normal | good | luxury. floor_height: metres. storeys: a whole number of floors.\n"
    "Examples (message -> claims):\n"
    "'My flat in Hue needs a standard finish' -> [{quote:'flat', field:kind, value:apartment}, {quote:'Hue', field:city, value:Hue}, "
    "{quote:'standard finish', field:quality, value:normal}]\n"
    "'Nhà phố ở Thủ Đức, TP HCM, 3 tầng' -> [{quote:'Nhà phố', field:kind, value:house}, {quote:'TP HCM', field:city, "
    "value:Ho Chi Minh City}, {quote:'Thủ Đức', field:district, value:Thu Duc}, {quote:'3 tầng', field:storeys, value:3}]\n"
    "'ハノイの一戸建て、高品質で' -> [{quote:'一戸建て', field:kind, value:house}, {quote:'ハノイ', field:city, value:Hanoi}, "
    "{quote:'高品質', field:quality, value:good}]\n"
    "'Ceilings are 2.9 m high' -> [{quote:'2.9 m', field:floor_height, value:2.9}]\n"
    "'The rooms already exist' / 'Hello' -> []"
)

SETTINGS_SYSTEM = _COMMON + (
    "Fields and values:\n"
    "- structure_in_scope: true | false (foundation, columns, slabs are part of the work). mep_in_scope: true | false (electrical and plumbing).\n"
    "- vat_percent, overhead_percent, profit_percent, design_percent, contingency_percent: a number, only when the message gives one.\n"
    "- use_typical_markups: true, ONLY when the person says to use typical or usual percentages.\n"
    "- decide_for_me: the person gives up on the question just asked ('I don't know', 'you decide', 'tùy bạn', 'お任せします'); "
    "value = the slot named in 'Question just asked'.\n"
    "- explain: the cost item the person asks about.\n"
    "Examples (message -> claims):\n"
    "'Overhead 6%, VAT 8%' -> [{quote:'Overhead 6%', field:overhead_percent, value:6}, {quote:'VAT 8%', field:vat_percent, value:8}]\n"
    "'Include the wiring and pipes' / 'Tính cả đường điện nước' / '電気と配管も含める' -> one claim: mep_in_scope true.\n"
    "'Only the finishing, structure is not needed' / 'Chỉ phần hoàn thiện thôi' / '仕上げのみで構造は不要' -> one claim: structure_in_scope false.\n"
    "'Not sure' (question just asked, slot quality) -> [{quote:'Not sure', field:decide_for_me, value:quality}]\n"
    "'Why does the ceiling cost that much?' -> [{quote:'the ceiling', field:explain, value:ceiling}]\n"
    "'Use the usual percentages' -> [{quote:'usual percentages', field:use_typical_markups, value:true}]\n"
    "'Hello' / 'normal quality' -> []"
)

SCOPE_SYSTEM = _COMMON + (
    "Fields and values: the CONDITION of rooms.\n"
    "- scope_default: the condition of all rooms: empty (bare shell, nothing finished) | existing (already finished, no work) | to_replace "
    "(strip and redo) | new (to be built).\n"
    "- scope_room: one room whose condition differs from the rest, as 'room_id=state' with the room_id from the room list. A room kind "
    "('the toilets', 'nhà vệ sinh', 'トイレ') means EVERY room of that kind: one claim per room.\n"
    "A message that does not say whether rooms are empty, finished, to be replaced or new gives NO claim: 'the rooms exist' only says "
    "they are on the plan, and 'finishes only' or 'no structure' is about the scope of work.\n"
    "Examples (the toilets are r7 and r9):\n"
    "'The place is a bare shell' / 'Nhà còn thô, chưa hoàn thiện gì' / '全室スケルトンです' -> one claim: scope_default empty.\n"
    "'Everything is bare except the toilets' -> [{quote:'Everything is bare', field:scope_default, value:empty}, "
    "{quote:'except the toilets', field:scope_room, value:r7=existing}, {quote:'except the toilets', field:scope_room, value:r9=existing}]\n"
    "'Chỉ có nhà vệ sinh là đã xong, còn lại để trống' -> [{quote:'còn lại để trống', field:scope_default, value:empty}, "
    "{quote:'nhà vệ sinh là đã xong', field:scope_room, value:r7=existing}, {quote:'nhà vệ sinh là đã xong', field:scope_room, value:r9=existing}]\n"
    "'トイレ以外は何も仕上がっていません' -> [{quote:'何も仕上がっていません', field:scope_default, value:empty}, "
    "{quote:'トイレ以外', field:scope_room, value:r7=existing}, {quote:'トイレ以外', field:scope_room, value:r9=existing}]\n"
    "'Finishes only, no structure' / 'The rooms already exist' / 'Các phòng có sẵn rồi' / 'Hello' -> []"
)

ITEMS_SYSTEM = _COMMON + (
    "Fields and values: what is already installed, and what is out of or back in the estimate.\n"
    "- fixture_exists: the id of a fixture from the fixture list that the person says is ALREADY installed (one claim per fixture). A "
    "floor plan only shows where fixtures go, so they are assumed NOT installed. fixtures_all_exist / fixtures_none_exist: true, only when "
    "the person says all of them are installed / none of them is.\n"
    "- exclude_item: a rate code the person wants OUT of the estimate because that work already exists or is not wanted. include_item: a "
    "code they want back. One claim per code. Codes: " + ", ".join(RATE_CATALOG) + ". A door that is not called entrance, front or main "
    "means two claims: door_interior and door_entrance. An exception ('except the entrance door') or an 'only X' names what exists.\n"
    "Installed fixtures are fixture_exists with their ids, never exclude_item.\n"
    "Examples (the toilets' fixtures are f8 and f9):\n"
    "'Nothing is installed yet except the front door' -> [{quote:'Nothing is installed yet', field:fixtures_none_exist, value:true}, "
    "{quote:'except the front door', field:exclude_item, value:door_entrance}]\n"
    "'Thiết bị chưa lắp cái nào, trừ cửa ra vào' -> [{quote:'Thiết bị chưa lắp cái nào', field:fixtures_none_exist, value:true}, "
    "{quote:'trừ cửa ra vào', field:exclude_item, value:door_entrance}]\n"
    "'設備は未設置です。玄関ドアだけは取り付け済み' -> [{quote:'設備は未設置', field:fixtures_none_exist, value:true}, "
    "{quote:'玄関ドアだけは取り付け済み', field:exclude_item, value:door_entrance}]\n"
    "'The toilets already have everything' -> [{quote:'The toilets already have everything', field:fixture_exists, value:f8}, "
    "{quote:'The toilets already have everything', field:fixture_exists, value:f9}]\n"
    "'Nothing yet' (question just asked: which fixtures already exist?) -> [{quote:'Nothing yet', field:fixtures_none_exist, value:true}]\n"
    "'Skip the cost of the glazing, it is done' -> [{quote:'Skip the cost of the glazing', field:exclude_item, value:window}]\n"
    "'Count the front door again' -> [{quote:'Count the front door again', field:include_item, value:door_entrance}]\n"
    "'Rooms are bare' / 'Hello' / 'normal quality' -> []"
)


# ---------- grounding: is the quote really in the message? ----------


def _norm(s: str) -> str:
    """Case, accent, width and spacing insensitive form: 'Cửa  Chính' and 'cua chinh' compare equal; kana and kanji stay as they are."""
    s = unicodedata.normalize("NFKC", s).replace("đ", "d").replace("Đ", "d")
    s = "".join(c for c in unicodedata.normalize("NFD", s) if unicodedata.category(c) != "Mn")
    return " ".join(s.casefold().split())


def grounded(quote: str, message: str) -> bool:
    """True when the quote is in the message. A small tolerance (85% of the quote in one run) forgives a model that copies loosely."""
    q, m = _norm(quote), _norm(message)
    if len(q) < 2:
        return False
    if q in m:
        return True
    match = difflib.SequenceMatcher(None, q, m, autojunk=False).find_longest_match(0, len(q), 0, len(m))
    return match.size >= max(3, 0.85 * len(q))


# ---------- claims -> a reading ----------


def _convert(reading: type[BaseModel], name: str, value: str) -> object:
    """The value for one reading field, or raises ValueError. Booleans and numbers arrive as text."""
    v = value.strip()
    if name == "scope_rooms":
        rid, _, st = v.partition("=")
        if st.strip() not in get_args(RoomState):
            raise ValueError(v)
        return {"room_id": rid.strip(), "state": st.strip()}
    if name in ("exclude_items", "include_items") and v not in RATE_CATALOG:
        raise ValueError(v)
    if name in ("exclude_items", "include_items", "fixtures_existing", "decide_for_me"):
        return v
    ann = reading.model_fields[name].annotation
    try:
        return TypeAdapter(ann).validate_python(v.lower() if v.lower() in ("true", "false") else v)
    except ValidationError as e:
        raise ValueError(v) from e


def assemble(reading: type[BaseModel], table: dict[str, tuple[str, bool]], claims: list, message: str):
    """Build a reading from claims: a claim is kept only when its quote is in the message and its value is valid for its field."""
    data: dict = {}
    for c in claims:
        if c.field not in table or not grounded(c.quote, message):
            continue
        name, many = table[c.field]
        try:
            v = _convert(reading, name, c.value)
        except ValueError:
            continue
        if many:
            if v not in data.setdefault(name, []):
                data[name].append(v)
        else:
            data[name] = v
    try:
        return reading.model_validate(data)
    except ValidationError:  # one out-of-range number must not cost the rest: keep what validates alone
        ok = {}
        for k, v in data.items():
            try:
                reading.model_validate({k: v})
                ok[k] = v
            except ValidationError:
                pass
        return reading.model_validate(ok)


# ---------- the four calls ----------

MAX_TOKENS = 300  # a claims list is short; the cap stops a runaway generation instead of waiting minutes for it


def read_message(llm, ctx: dict[str, str], message: str) -> tuple[FactsReading, ExistingReading]:
    """The four readings, run at once (the engine allows several model calls together), each built from verified claims.
    ctx: facts (so far), asked (the question just asked, or ""), rooms, fixtures."""
    q = ctx.get("asked", "")
    msg = f"\n\nThe person's message:\n<message>\n{message}\n</message>"
    jobs = [
        (CORE_SYSTEM, CoreClaims, f"Facts so far:\n{ctx['facts']}{q}{msg}"),
        (SETTINGS_SYSTEM, SettingsClaims, f"Facts so far:\n{ctx['facts']}{q}{msg}"),
        (SCOPE_SYSTEM, ScopeClaims, f"Rooms in the plan:\n{ctx['rooms']}{q}{msg}"),
        (ITEMS_SYSTEM, ItemsClaims, f"Rooms in the plan:\n{ctx['rooms']}\n\nFixtures in the plan:\n{ctx['fixtures'] or '(none)'}{q}{msg}"),
    ]
    with ThreadPoolExecutor(max_workers=len(jobs)) as pool:
        futs = [pool.submit(contextvars.copy_context().run, llm.structured, sys_, user, cls, MAX_TOKENS) for sys_, cls, user in jobs]
        out = [f.result() for f in futs]
    facts_claims = out[0].claims + out[1].claims
    existing_claims = out[2].claims + out[3].claims
    return (assemble(FactsReading, {**CORE_TABLE, **SETTINGS_TABLE}, facts_claims, message),
            assemble(ExistingReading, {**SCOPE_TABLE, **ITEMS_TABLE}, existing_claims, message))
