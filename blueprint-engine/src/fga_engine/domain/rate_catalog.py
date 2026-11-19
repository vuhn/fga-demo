"""The rate catalog: which rate codes exist, what they are and how they are counted. No prices anywhere in the codebase."""

from __future__ import annotations

from dataclasses import dataclass

# Fixtures the plan draws that are part of the building work; loose furniture and appliances are not priced.
PRICED_FIXTURES: dict[str, tuple[str, str]] = {  # kind -> (rate code, how it is counted)
    "toilet": ("fixture_toilet", "each"), "basin": ("fixture_basin", "each"), "shower": ("fixture_shower", "each"),
    "bathtub": ("fixture_bathtub", "each"), "sink": ("fixture_sink", "each"),
    "kitchen_counter": ("fixture_kitchen_counter", "run"), "wardrobe": ("fixture_wardrobe", "run"),
}
PLUMBED = {"toilet", "basin", "shower", "bathtub", "sink", "washer"}


@dataclass(frozen=True)
class RateSpec:
    package: str
    description: str  # what to search for; quality words are added for quality-sensitive codes
    unit: str
    quality_sensitive: bool = False


# Rate codes: descriptions and units only. There are no prices in the codebase.
RATE_CATALOG: dict[str, RateSpec] = {
    "demolition_finish": RateSpec("Demolition", "demolition of floor tiles and wall plaster, labour and disposal", "m2"),
    "brick_wall_100": RateSpec("Masonry", "100 mm brick wall built and plastered-ready, material and labour", "m2"),
    "plaster": RateSpec("Plaster", "cement plaster 15 mm on walls, material and labour", "m2"),
    "paint": RateSpec("Finishes", "interior wall paint, putty and 2 coats, material and labour", "m2", True),
    "floor_tile": RateSpec("Finishes", "ceramic or porcelain floor tile, supply and install", "m2", True),
    "wall_tile": RateSpec("Finishes", "bathroom wall tile, supply and install", "m2", True),
    "ceiling": RateSpec("Finishes", "gypsum board false ceiling, supply and install", "m2", True),
    "skirting": RateSpec("Finishes", "floor skirting, supply and install", "m", True),
    "waterproofing": RateSpec("Finishes", "waterproofing membrane for bathroom floor, material and labour", "m2"),
    "door_interior": RateSpec("Openings", "interior door with frame, supply and install", "each", True),
    "door_entrance": RateSpec("Openings", "entrance door with frame, supply and install", "each", True),
    "window": RateSpec("Openings", "aluminium glass window, supply and install", "m2", True),
    "electrical_point": RateSpec("MEP", "electrical point (outlet or light) including wiring, supply and install", "each", True),
    "plumbing_point": RateSpec("MEP", "water supply and drain point for a sanitary fixture, supply and install", "each", True),
    "fixture_toilet": RateSpec("Fixtures", "toilet (water closet), supply and install", "each", True),
    "fixture_basin": RateSpec("Fixtures", "washbasin with tap, supply and install", "each", True),
    "fixture_shower": RateSpec("Fixtures", "shower set with tray or screen, supply and install", "each", True),
    "fixture_bathtub": RateSpec("Fixtures", "bathtub with tap, supply and install", "each", True),
    "fixture_sink": RateSpec("Fixtures", "kitchen sink with tap, supply and install", "each", True),
    "fixture_kitchen_counter": RateSpec("Fixtures", "built-in kitchen cabinet and countertop, per metre run, supply and install", "m", True),
    "fixture_wardrobe": RateSpec("Fixtures", "built-in wardrobe, per metre run, supply and install", "m", True),
    "excavation": RateSpec("Earthwork and foundation", "foundation excavation by machine, per cubic metre", "m3"),
    "concrete": RateSpec("Structure", "ready-mix concrete C25 supplied, poured and cured", "m3"),
    "rebar": RateSpec("Structure", "reinforcement steel bar cut, bent and tied", "kg"),
    "formwork": RateSpec("Structure", "formwork for columns beams and slabs", "m2"),
    "roof_tile": RateSpec("Roof", "pitched roof with tiles on steel or concrete frame", "m2", True),
}


# mid-range of the PRD; used only when the user agrees ("use typical")
DEFAULT_MARKUPS = {
    "overhead_percent": 6.5,   # site overhead and temporary works, 5-8%
    "profit_percent": 7.5,     # contractor profit, 5-10%
    "design_percent": 5.5,     # design, supervision, permits, 3-8%
    "contingency_percent": 7.5,  # 5-10%
    "vat_percent": 8.0,        # the user confirms the rate in force
}
CONTINGENCY_EXTRA_MAX = 5.0  # added at 100% assumed or low-confidence cost
