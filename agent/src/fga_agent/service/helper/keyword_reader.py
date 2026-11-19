from __future__ import annotations

import re

from ...domain.model.estimate_session import SessionState
from ...domain.model.extracted import Extracted, RoomStateClaim
from ...domain.model.plan_summary import PlanSummary

UNKNOWN_RE = re.compile(r"don'?t know|do not know|you decide|up to you|not sure|không biết|tùy|tuỳ|bạn quyết|bạn chọn", re.I)
TYPICAL_RE = re.compile(r"\btypical (rates|percent|markup)|\busual (rates|percent)|\bstandard rates\b|thông thường", re.I)
SCOPE_WORDS_RE = re.compile(r"\b(empty|bare|blank|shell|raw|unfinished|finished|furnished|renovated|replace|redo|strip|demolish|brand new|new)\b|trống|chưa hoàn thiện|đã hoàn thiện", re.I)
EXIST_RE = re.compile(r"\bexist", re.I)
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
FIXTURE_WORDS: dict[str, re.Pattern] = {
    "toilet": re.compile(r"\b(toilets?|wc|water closets?)\b", re.I), "basin": re.compile(r"\b(basins?|washbasins?|lavator(y|ies))\b", re.I),
    "shower": re.compile(r"\bshowers?\b", re.I), "bathtub": re.compile(r"\b(bath ?tubs?|tubs?)\b", re.I),
    "sink": re.compile(r"\bsinks?\b", re.I), "kitchen_counter": re.compile(r"\b(counters?|kitchen cabinets?|cabinets?|countertops?)\b", re.I),
    "wardrobe": re.compile(r"\bwardrobes?\b", re.I),
}
FIXTURE_TALK_RE = re.compile(r"fixture|toilet|wc|basin|shower|bathtub|sink|wardrobe|cabinet|kitchen", re.I)
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
# "except the entrance door", "only the entrance door exists": the work on that item is already done
EXCEPT_RE = re.compile(r"\b(?:except(?: for)?|apart from|other than|but for|besides)\s+(?:the\s+)?([^.;\n]+)|\btrừ\s+([^.;\n]+)", re.I)
ONLY_RE = re.compile(r"\bonly (?:the )?([^.;\n]+?)\s+(?:is|are|exists?|installed|there|in place|done)\b", re.I)
# the items an "except ..." may take out: openings only; for fixtures and rooms an exception is about their condition
OPENING_CODES = {"door_interior", "door_entrance", "window"}
ADD_RE = re.compile(r"\b(add|include|put back|restore|bring back)\b.*\b(again|back)\b|\b(add|include) (the )?[a-z ]+ (back|again)\b", re.I)
ADD_WORD_RE = re.compile(r"\b(add|include|put|restore|bring)\b", re.I)
CITY_RE: dict[str, re.Pattern] = {
    "Ho Chi Minh City": re.compile(r"\b(ho chi minh|hcmc?|saigon|sài gòn|tp\.? ?hcm)\b", re.I),
    "Hanoi": re.compile(r"\b(hanoi|ha noi|hà nội)\b", re.I),
    "Da Nang": re.compile(r"\b(da nang|đà nẵng)\b", re.I),
}
MEP_RE = re.compile(r"\b(include|including|with|exclude|excluding|without|skip|no)\s+(?:the\s+)?(?:mep|electrical|plumbing)\b", re.I)
HEIGHT_RE = re.compile(r"(?:floor|ceiling|storey|story) height(?: is| of)?\s*(?:about |around )?(\d+(?:[.,]\d+)?)\s*m\b", re.I)
NUM_RE = re.compile(r"(\d+(?:[.,]\d+)?)")
STRUCTURE_OUT_RE = re.compile(r"\b(finishes only|finishing only|without structure|no structure|excluding structure|exclude structure)\b")
STRUCTURE_IN_RE = re.compile(r"\b(include|including|with) (the )?structure\b")
NONE_EXIST_RE = re.compile(r"\bfixtures?\b.{0,20}\b(empty|none|bare|absent)\b|\bnone\b.*\b(exist|installed)|nothing (is )?installed")
ALL_EXIST_RE = re.compile(r"\ball\b.*\b(exist|installed)")


def _tokens(name: str) -> list[str]:
    return re.findall(r"[^\W_]+", name.lower())


def _first(rx_by_value: dict[str, re.Pattern], message: str) -> str | None:
    return next((k for k, rx in rx_by_value.items() if rx.search(message)), None)


def _number(message: str) -> float | None:
    n = NUM_RE.search(message)
    return float(n.group(1).replace(",", ".")) if n else None


class KeywordReaderService:
    def read(self, state: SessionState, summary: PlanSummary, message: str) -> Extracted:
        m = message.strip()
        out: dict = {}
        out.update(self._about_the_home(state, m))
        out.update(self._about_the_work(state, m))
        out.update(self._about_items(state, m))
        out.update(self._answer_to_the_question_asked(state, summary, m))
        out.update(self._fixtures_and_scope_chips(state, summary, m))
        return Extracted(**out)

    def _about_the_home(self, state: SessionState, m: str) -> dict:
        out: dict = {}
        if kind := _first(KIND_RE, m):
            out["kind"] = kind
        if state.pending[:1] == ["quality"] or "quality" in m.lower():
            if quality := _first(QUALITY_RE, m):
                out["quality"] = quality
        if city := next((c for c, rx in CITY_RE.items() if rx.search(m)), None):
            out["city"] = city
        if (hm := HEIGHT_RE.search(m)) and 2.2 <= (v := float(hm.group(1).replace(",", "."))) <= 6:
            out["floor_height"] = v
        return out

    def _about_the_work(self, state: SessionState, m: str) -> dict:
        out: dict = {}
        low = m.lower()
        if work := _first(WORK_RE, m):
            out["work_type"] = work
        if STRUCTURE_OUT_RE.search(low):
            out["structure_in_scope"] = False
        elif STRUCTURE_IN_RE.search(low):
            out["structure_in_scope"] = True
        if mm := MEP_RE.search(m):
            out["mep_in_scope"] = not re.match(r"(exclude|without|no|skip)", mm.group(1), re.I)
        return out

    def _about_items(self, state: SessionState, m: str) -> dict:
        out: dict = {}
        items = self._items_in(m)
        if items and ADD_RE.search(m) and (state.estimate is not None or state.excluded):
            out["include_items"] = items
        elif items and DROP_RE.search(m):
            out["exclude_items"] = items
        if not out.get("exclude_items") and (ex := self.exception_items(m)):
            out["exclude_items"] = ex
        return out

    def _answer_to_the_question_asked(self, state: SessionState, summary: PlanSummary, m: str) -> dict:
        out: dict = {}
        low, asked = m.lower(), (state.pending[0] if state.pending else "")
        if asked == "work_type" and (work := _first(WORK_RE, m)):
            out["work_type"] = work
        elif asked == "city" and 1 < len(m) <= 60 and not UNKNOWN_RE.search(m):
            out.update(self._place(m))
        elif asked in ("floor_height", "storeys") and (v := _number(m)) is not None:
            if asked == "floor_height" and 2.2 <= v <= 6:
                out["floor_height"] = v
            elif asked == "storeys" and v == int(v) and 1 <= v <= 12:
                out["storeys"] = int(v)
        elif asked in ("mep_in_scope", "structure_in_scope"):
            if re.match(r"\s*(yes|include)", low):
                out[asked] = True
            elif re.match(r"\s*(no|exclude|finishes only)", low):
                out[asked] = False
        return out

    def _place(self, m: str) -> dict:
        """A place answer: the city by code, the rest as the district ("District 7, Ho Chi Minh City")."""
        parts = [x.strip() for x in m.split(",") if x.strip()]
        known = next((c for c, rx in CITY_RE.items() if rx.search(m)), None)
        if known:  # a city we recognise wins wherever it stands
            rest = [x for x in parts if not CITY_RE[known].search(x)]
            return {"city": known, **({"district": rest[0]} if rest else {})}
        if not parts:
            return {}
        return {"city": parts[0], **({"district": parts[1]} if len(parts) > 1 else {})}

    def _fixtures_and_scope_chips(self, state: SessionState, summary: PlanSummary, m: str) -> dict:
        out: dict = {}
        low, asked = m.lower(), (state.pending[0] if state.pending else "")
        if asked == "fixtures" or "fixture" in low:
            if low.startswith("none of them") or NONE_EXIST_RE.search(low):
                out["fixtures_none_exist"] = True
            elif low.startswith("all of them") or ALL_EXIST_RE.search(low):
                out["fixtures_all_exist"] = True
        if low.startswith("everything is empty") or low.startswith("everything already exists"):
            out["scope_default"] = "empty" if "empty" in low[:22] else "existing"
            if "except the bathrooms" in low:
                out["scope_rooms"] = [RoomStateClaim(r.id, "existing") for r in summary.rooms if r.type == "bathroom"]
        return out

    def _items_in(self, message: str) -> list[str]:
        """Rate codes a message names. Longer phrases claim their words first ("entrance door" before "door")."""
        low, found = message.lower(), []
        for pat in sorted(ITEM_WORDS, key=len, reverse=True):
            if re.search(rf"\b({pat})\b", low):
                if any(c in found for c in ITEM_WORDS[pat]) and pat in ("doors?|cửa đi|cửa",):
                    continue
                found += [c for c in ITEM_WORDS[pat] if c not in found]
        return found

    def exception_items(self, message: str) -> list[str]:
        clauses = [g for m in EXCEPT_RE.finditer(message) for g in m.groups() if g] + [m.group(1) for m in ONLY_RE.finditer(message)]
        out: list[str] = []
        for c in clauses:
            out += [i for i in self._items_in(c) if i in OPENING_CODES and i not in out]
        return out

    def says_unknown(self, message: str) -> bool:
        return bool(UNKNOWN_RE.search(message))

    def says_typical(self, message: str) -> bool:
        return bool(TYPICAL_RE.search(message))

    def names_removal(self, message: str) -> bool:
        return bool(DROP_RE.search(message) or self.exception_items(message))

    def names_addition(self, message: str) -> bool:
        return bool(ADD_WORD_RE.search(message))

    def names_condition(self, message: str) -> bool:
        return bool(SCOPE_WORDS_RE.search(message))

    def mentions_existing(self, message: str) -> bool:
        return bool(EXIST_RE.search(message))

    def mentions_fixtures(self, message: str) -> bool:
        return bool(FIXTURE_TALK_RE.search(message))

    def referred_rooms(self, summary: PlanSummary, message: str) -> set[str]:
        named = self._named_rooms(summary, message)
        if named:
            return named
        return {r.id for r in summary.rooms if r.type in TYPE_WORDS and TYPE_WORDS[r.type].search(message)}

    def _named_rooms(self, summary: PlanSummary, message: str) -> set[str]:
        low = message.lower()
        return {r.id for r in summary.rooms if (toks := _tokens(r.name)) and all(t in low for t in toks)}

    def referred_fixtures(self, summary: PlanSummary, priced_kinds: list[str], message: str) -> set[str]:
        kinds = {k for k, rx in FIXTURE_WORDS.items() if rx.search(message)}
        named = self._named_rooms(summary, message)
        return {f.id for f in summary.fixtures if f.kind in priced_kinds and (not kinds or f.kind in kinds)
                and (not named or f.room_id in named) and (kinds or named)}
