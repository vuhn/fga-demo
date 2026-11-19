from __future__ import annotations

import difflib
import unicodedata

KINDS = ("apartment", "house")
WORK_TYPES = ("new_build", "fit_out", "renovation")
QUALITIES = ("typical", "normal", "good", "luxury")
ROOM_STATES = ("empty", "existing", "to_replace", "new")
PERCENTS = ("vat_percent", "overhead_percent", "profit_percent", "design_percent", "contingency_percent")
FLAGS = ("structure_in_scope", "mep_in_scope", "use_typical_markups", "fixtures_all_exist", "fixtures_none_exist")
LIST_LIMITS = {"scope_rooms": 40, "fixtures_existing": 60, "exclude_items": 30, "include_items": 30, "decide_for_me": 12}
TRUE, FALSE = {"true", "yes", "1", "on"}, {"false", "no", "0", "off"}


def _norm(s: str) -> str:
    """'Cửa  Chính' and 'cua chinh' compare equal; kana and kanji stay as they are."""
    s = unicodedata.normalize("NFKC", s).replace("đ", "d").replace("Đ", "d")
    s = "".join(c for c in unicodedata.normalize("NFD", s) if unicodedata.category(c) != "Mn")
    return " ".join(s.casefold().split())


def _bounded(v: str, low: float, high: float) -> float:
    n = float(v)
    if not low <= n <= high:
        raise ValueError(v)
    return n


def _one_of(v: str, allowed: tuple[str, ...]) -> str:
    if v not in allowed:
        raise ValueError(v)
    return v


def _flag(v: str) -> bool:
    key = v.lower()
    if key in TRUE:
        return True
    if key in FALSE:
        return False
    raise ValueError(v)


def _text(v: str, longest: int) -> str:
    if not v or len(v) > longest:
        raise ValueError(v)
    return v


def _convert(name: str, value: str, rate_codes: list[str]):
    """The value for one reading field, or ValueError. Booleans and numbers arrive as text."""
    v = value.strip()
    if name == "scope_rooms":
        room_id, _, state = v.partition("=")
        return room_id.strip(), _one_of(state.strip(), ROOM_STATES)
    if name in ("exclude_items", "include_items"):
        return _one_of(v, tuple(rate_codes))
    if name in ("fixtures_existing", "decide_for_me"):
        return v
    if name == "kind":
        return _one_of(v, KINDS)
    if name == "work_type":
        return _one_of(v, WORK_TYPES)
    if name == "quality":
        return _one_of(v, QUALITIES)
    if name == "scope_default":
        return _one_of(v, ROOM_STATES)
    if name in ("city", "district"):
        return _text(v, 60)
    if name == "explain":
        return _text(v, 80)
    if name == "floor_height":
        return _bounded(v, 2.2, 6)
    if name == "storeys":
        n = _bounded(v, 1, 12)
        if n != int(n):
            raise ValueError(v)
        return int(n)
    if name in PERCENTS:
        return _bounded(v, 0, 30)
    if name in FLAGS:
        return _flag(v)
    raise ValueError(name)


class ClaimCheckerService:
    def grounded(self, quote: str, message: str) -> bool:
        q, m = _norm(quote), _norm(message)
        if len(q) < 2:
            return False
        if q in m:
            return True
        match = difflib.SequenceMatcher(None, q, m, autojunk=False).find_longest_match(0, len(q), 0, len(m))
        return match.size >= max(3, 0.85 * len(q))

    def verified(self, fields: dict[str, tuple[str, bool]], claims: list[dict], message: str, rate_codes: list[str]) -> dict:
        data: dict = {}
        for c in claims:
            field = c.get("field")
            if field not in fields or not self.grounded(str(c.get("quote", "")), message):
                continue
            name, many = fields[field]
            try:
                v = _convert(name, str(c.get("value", "")), rate_codes)
            except ValueError:
                continue
            if many:
                if v not in data.setdefault(name, []) and len(data[name]) < LIST_LIMITS[name]:
                    data[name].append(v)
            else:
                data[name] = v
        return data
