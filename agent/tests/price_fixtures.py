"""The Engine's rate catalog as the Agent reads it, and doubles for the price research: a model that answers the two
questions (the search queries, the prices on a page) from a script, a search provider and a page reader that count calls."""

from __future__ import annotations

import re
import threading

from fga_agent.domain.model.design_limits import DesignLimits, RateSpec
from fga_agent.domain.model.rate import RateRequest
from fga_agent.domain.model.search_hit import SearchHit
from fga_agent.service.prompts import price_research as prompt

CATALOG = [RateSpec(code, package, description, unit, sensitive) for code, package, description, unit, sensitive in [
    ("demolition_finish", "Demolition", "demolition of floor tiles and wall plaster, labour and disposal", "m2", False),
    ("brick_wall_100", "Masonry", "100 mm brick wall built and plastered-ready, material and labour", "m2", False),
    ("plaster", "Plaster", "cement plaster 15 mm on walls, material and labour", "m2", False),
    ("paint", "Finishes", "interior wall paint, putty and 2 coats, material and labour", "m2", True),
    ("floor_tile", "Finishes", "ceramic or porcelain floor tile, supply and install", "m2", True),
    ("wall_tile", "Finishes", "bathroom wall tile, supply and install", "m2", True),
    ("ceiling", "Finishes", "gypsum board false ceiling, supply and install", "m2", True),
    ("skirting", "Finishes", "floor skirting, supply and install", "m", True),
    ("waterproofing", "Finishes", "waterproofing membrane for bathroom floor, material and labour", "m2", False),
    ("door_interior", "Openings", "interior door with frame, supply and install", "each", True),
    ("door_entrance", "Openings", "entrance door with frame, supply and install", "each", True),
    ("window", "Openings", "aluminium glass window, supply and install", "m2", True),
    ("electrical_point", "MEP", "electrical point (outlet or light) including wiring, supply and install", "each", True),
    ("plumbing_point", "MEP", "water supply and drain point for a sanitary fixture, supply and install", "each", True),
    ("fixture_toilet", "Fixtures", "toilet (water closet), supply and install", "each", True),
    ("fixture_basin", "Fixtures", "washbasin with tap, supply and install", "each", True),
    ("fixture_shower", "Fixtures", "shower set with tray or screen, supply and install", "each", True),
    ("fixture_bathtub", "Fixtures", "bathtub with tap, supply and install", "each", True),
    ("fixture_sink", "Fixtures", "kitchen sink with tap, supply and install", "each", True),
    ("fixture_kitchen_counter", "Fixtures", "built-in kitchen cabinet and countertop, per metre run, supply and install", "m", True),
    ("fixture_wardrobe", "Fixtures", "built-in wardrobe, per metre run, supply and install", "m", True),
    ("excavation", "Earthwork and foundation", "foundation excavation by machine, per cubic metre", "m3", False),
    ("concrete", "Structure", "ready-mix concrete C25 supplied, poured and cured", "m3", False),
    ("rebar", "Structure", "reinforcement steel bar cut, bent and tied", "kg", False),
    ("formwork", "Structure", "formwork for columns beams and slabs", "m2", False),
    ("roof_tile", "Roof", "pitched roof with tiles on steel or concrete frame", "m2", True),
]]
LIMITS = DesignLimits(min_area={}, min_width={}, nestable_types=[], max_nested=2, default_fixtures={}, fixture_kinds=[],
                      default_markups={}, rate_catalog=CATALOG, priced_fixtures=[])
UNITS = {s.code: s.unit for s in CATALOG}

PAGE = "Gạch lát nền 60x60: 250.000đ/m2"
REQUEST = RateRequest(code="floor_tile", description="floor tile", unit="m2", region="Ho Chi Minh City", quality="normal")


def find(**kw) -> dict:
    d = dict(price=250000, currency="VND", unit="m2", quote=PAGE, material_spec="", date="")
    d.update(kw)
    return d


class ResearchLanguageModel:
    """answer_json(): the search queries for the query prompt, the finds for the extraction prompt (`finds` is a list, or a
    function of the user prompt). Calls come from several threads, so they are recorded under a lock."""

    def __init__(self, finds, queries=("đơn giá gạch lát nền",)) -> None:
        self.finds, self.queries = finds, list(queries)
        self.calls: list[dict] = []
        self.lock = threading.Lock()

    def answer_json(self, system, user, schema, max_tokens=None, on_progress=None, think=True) -> dict:
        with self.lock:
            self.calls.append({"system": system, "user": user, "think": think})
        if system == prompt.QUERY_SYSTEM:
            return {"queries": self.queries}
        return {"items": self.finds(user) if callable(self.finds) else self.finds}

    def extractions(self) -> list[dict]:
        return [c for c in self.calls if c["system"] == prompt.EXTRACT_SYSTEM]


class PageReader(ResearchLanguageModel):
    """A stand-in for the model that reads the price off the page text it is given."""

    def __init__(self) -> None:
        super().__init__(self.read)

    @staticmethod
    def read(user: str) -> list[dict]:
        page = user.split("<page>\n", 1)[1].rsplit("\n</page>", 1)[0]
        m = re.search(r"[^.]*: ([\d.]+)đ/(\S+)", page)
        if not m:
            return []
        return [find(price=float(m.group(1).replace(".", "")), unit=m.group(2), quote=m.group(0).strip())]


class OneSearch:
    def __init__(self, hits=3) -> None:
        self.queries: list[str] = []
        self.requests: list[RateRequest] = []
        self.hits = hits

    def search(self, query, request, n=5):
        self.queries.append(query)
        self.requests.append(request)
        return [SearchHit(f"https://example.test/{i}", "t", "s") for i in range(self.hits)]


class Pages:
    def __init__(self, text_of=lambda url: PAGE) -> None:
        self.text_of, self.urls = text_of, []

    def text(self, url):
        self.urls.append(url)
        return self.text_of(url)
