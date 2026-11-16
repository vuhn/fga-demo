"""A fake search provider for demos and tests without a search API key (ESTIMATE_SEARCH_PROVIDER=demo).

It returns invented price pages for every rate code, so the whole pipeline runs for real around it: the LLM
writes the queries, the LLM reads these pages, and code checks the quotes, converts units and aggregates.
THE PRICES ARE NOT REAL MARKET DATA. Every URL is on demo.invalid and the estimate says so in its warnings."""

from __future__ import annotations

import hashlib

from .price_research import Hit, SearchProvider, current_request
from .takeoff import RATE_CATALOG

DEMO_HOST = "demo.invalid"
# VND per catalog unit, for a normal-quality job in Ho Chi Minh City. Invented, plausible orders of magnitude.
BASE: dict[str, float] = {
    "demolition_finish": 110_000, "brick_wall_100": 260_000, "plaster": 90_000, "paint": 75_000, "floor_tile": 450_000,
    "wall_tile": 450_000, "ceiling": 220_000, "skirting": 75_000, "waterproofing": 150_000, "door_interior": 4_800_000,
    "door_entrance": 11_000_000, "window": 2_400_000, "electrical_point": 320_000, "plumbing_point": 580_000,
    "fixture_toilet": 4_500_000, "fixture_basin": 2_400_000, "fixture_shower": 4_800_000, "fixture_bathtub": 12_000_000,
    "fixture_sink": 3_200_000, "fixture_kitchen_counter": 4_000_000, "fixture_wardrobe": 3_700_000,
    "excavation": 200_000, "concrete": 2_100_000, "rebar": 19_000, "formwork": 220_000, "roof_tile": 550_000,
}
QUALITY_FACTOR = {"typical": 0.8, "normal": 1.0, "good": 1.3, "luxury": 1.8}
REGION_FACTOR = {"hanoi": 0.98, "ho chi minh": 1.0, "da nang": 0.92}
UNIT_TEXT = {"m2": "m2", "m3": "m3", "kg": "kg", "m": "m", "each": "cái"}


def _region_factor(region: str) -> float:
    low = region.lower()
    return next((f for k, f in REGION_FACTOR.items() if k in low), 0.95)


def demo_rate(code: str, region: str, quality: str) -> float:
    q = QUALITY_FACTOR.get(quality, 1.0) if RATE_CATALOG[code].quality_sensitive else 1.0
    return BASE[code] * q * _region_factor(region)


def _vn(n: float) -> str:
    return f"{round(n / 1000) * 1000:,.0f}".replace(",", ".")


def _page(code: str, i: int, region: str, quality: str) -> str:
    spread = [0.92, 1.0, 1.09][i % 3]  # three "shops" disagree a little, like real ones
    h = int(hashlib.sha1(f"{code}{i}".encode()).hexdigest(), 16) % 5 / 100
    price = demo_rate(code, region, quality) * (spread + h)
    unit = UNIT_TEXT[RATE_CATALOG[code].unit]
    name = RATE_CATALOG[code].description.split(",")[0]
    return (f"[DEMO DATA, not a real price list] Bảng giá tham khảo tại {region or 'Việt Nam'}. "
            f"{name.capitalize()}: {_vn(price)}đ/{unit} (đã gồm vật tư và nhân công). Giá chưa gồm VAT.")


def demo_fetch(url: str) -> str | None:
    # https://demo.invalid/<code>/<i>?r=<region>&q=<quality>
    try:
        path, _, qs = url.removeprefix(f"https://{DEMO_HOST}/").partition("?")
        code, i = path.split("/")
        args = dict(x.split("=", 1) for x in qs.split("&") if "=" in x)
        return _page(code, int(i), args.get("r", "").replace("+", " "), args.get("q", ""))
    except (ValueError, KeyError):
        return None


class DemoSearch(SearchProvider):
    fetch = staticmethod(demo_fetch)

    def search(self, query: str, n: int = 5) -> list[Hit]:
        req = current_request.get()
        if req is None or req.code not in BASE:
            return []
        suffix = f"?r={req.region.replace(' ', '+')}&q={req.quality}"
        return [Hit(f"https://{DEMO_HOST}/{req.code}/{i}{suffix}", f"Demo price list {i + 1}", "") for i in range(3)]
