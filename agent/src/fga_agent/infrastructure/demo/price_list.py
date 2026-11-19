"""Invented price pages for demos and tests without a search API key (ESTIMATE_SEARCH_PROVIDER=demo).

THE PRICES ARE NOT REAL MARKET DATA. Every URL is on demo.invalid and the estimate says so in its warnings."""

from __future__ import annotations

import hashlib

HOST = "demo.invalid"
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
    """The rate the demo pages are built around. `quality` is "" for a rate that does not depend on it."""
    return BASE[code] * QUALITY_FACTOR.get(quality, 1.0) * _region_factor(region)


def _vn(n: float) -> str:
    return f"{round(n / 1000) * 1000:,.0f}".replace(",", ".")


def page(code: str, i: int, region: str, quality: str, unit: str, name: str) -> str:
    spread = [0.92, 1.0, 1.09][i % 3]  # three "shops" disagree a little, like real ones
    h = int(hashlib.sha1(f"{code}{i}".encode()).hexdigest(), 16) % 5 / 100
    price = demo_rate(code, region, quality) * (spread + h)
    return (f"[DEMO DATA, not a real price list] Bảng giá tham khảo tại {region or 'Việt Nam'}. "
            f"{name.capitalize()}: {_vn(price)}đ/{UNIT_TEXT.get(unit, unit)} (đã gồm vật tư và nhân công). Giá chưa gồm VAT.")
