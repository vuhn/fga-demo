from __future__ import annotations

import re
import statistics

from ...domain.model.price_find import PriceFind
from ...domain.model.rate_range import RateRange

UNIT_ALIASES: dict[str, str] = {
    "m2": "m2", "m²": "m2", "sqm": "m2", "m 2": "m2", "mét vuông": "m2", "m vuông": "m2",
    "m3": "m3", "m³": "m3", "cbm": "m3", "mét khối": "m3", "khối": "m3",
    "kg": "kg", "ký": "kg", "tấn": "ton", "ton": "ton", "tonne": "ton", "tấn thép": "ton",
    "m": "m", "md": "m", "mét": "m", "mét dài": "m", "lm": "m",
    "each": "each", "cái": "each", "bộ": "each", "chiếc": "each", "điểm": "each", "unit": "each", "pc": "each", "pcs": "each",
}
DONG = ("VND", "VNĐ", "D", "DONG")
DOLLAR = ("USD", "$")
MAX_FINDS = 6


class PriceCheckerService:
    def __init__(self, usd_vnd: float = 25000.0) -> None:
        self._usd_vnd = usd_vnd

    def finds_from(self, items: object) -> list[PriceFind]:
        out: list[PriceFind] = []
        for x in items if isinstance(items, list) else []:
            find = self._find(x)
            if find is not None:
                out.append(find)
        return out[:MAX_FINDS]

    def _find(self, x: object) -> PriceFind | None:
        if not isinstance(x, dict):
            return None
        price, quote = x.get("price"), x.get("quote")
        if isinstance(price, bool) or not isinstance(price, (int, float)) or price <= 0:
            return None
        if not isinstance(quote, str) or not 3 <= len(quote) <= 400:
            return None
        text = {k: x[k] if isinstance(x.get(k), str) else "" for k in ("currency", "unit", "material_spec", "date")}
        return PriceFind(price=float(price), quote=quote, **text)

    def tidy(self, text: str) -> str:
        return re.sub(r"\s+", " ", text).strip()

    def quote_on_page(self, find: PriceFind, page: str) -> bool:
        if self.tidy(find.quote).casefold() not in self.tidy(page).casefold():
            return False
        digits = re.sub(r"\D", "", find.quote)
        want = re.sub(r"\D", "", f"{find.price:.0f}").rstrip("0") or "0"
        return want in digits

    def _unit(self, u: str) -> str:
        u = self.tidy(u.lower().rsplit("/", 1)[-1]).strip(". ")  # "đ/m2", "VND/m²" -> the part after the slash
        return UNIT_ALIASES.get(u, u)

    def rate_in_vnd(self, find: PriceFind, want_unit: str) -> float | None:
        cur = find.currency.strip().upper().replace("Đ", "D")
        if cur in DONG:
            price = find.price
        elif cur in DOLLAR:
            price = find.price * self._usd_vnd
        else:
            return None
        got, want = self._unit(find.unit), self._unit(want_unit)
        if got == want:
            return price
        if got == "ton" and want == "kg":
            return price / 1000
        return None

    def plausible(self, prices: list[float]) -> list[bool]:
        if not prices:
            return []
        med = statistics.median(prices)
        return [med / 3 <= p <= med * 3 for p in prices]

    def range_of(self, prices: list[float]) -> RateRange:
        if not prices:
            return RateRange(0, 0, 0, "none")
        kept = [p for p, ok in zip(prices, self.plausible(prices)) if ok]
        if len(kept) >= 3:
            q = statistics.quantiles(kept, n=4, method="inclusive")
            return RateRange(q[0], statistics.median(kept), q[2], "ok")
        base = statistics.median(kept or prices)
        return RateRange(base * 0.8, base, base * 1.25, "low_confidence")
