from __future__ import annotations

import re

CITY_GROUPS: dict[str, tuple[str, ...]] = {
    "hanoi": ("hà nội", "ha noi", "hanoi"),
    "hcm": ("hồ chí minh", "ho chi minh", "hcm", "sài gòn", "sai gon", "saigon", "tphcm", "tp.hcm", "tp. hcm"),
    "danang": ("đà nẵng", "da nang", "danang"),
    "haiphong": ("hải phòng", "hai phong", "haiphong"),
    "cantho": ("cần thơ", "can tho", "cantho"),
}
MAX_QUERY = 150


def _cities_in(text: str) -> set[str]:
    low = text.casefold()
    return {k for k, names in CITY_GROUPS.items() if any(n in low for n in names)}


class RegionPinnerService:
    def pin(self, query: str, region: str) -> str | None:
        query = re.sub(r"\s+", " ", query).strip()[:MAX_QUERY]
        want, got = _cities_in(region), _cities_in(query)
        if got and want and not (got & want):
            return None
        if region and region.casefold() not in query.casefold() and not (got & want):
            query = f"{query} {region}"
        return query
