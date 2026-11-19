"""A plan summary and the Engine's estimate catalog for the interview tests, and a language model that answers the four
readings of a message from a script (what the model would claim), the way the Engine-era tests did."""

from __future__ import annotations

import threading

from fga_agent.domain.model.design_limits import DesignLimits, RateSpec
from fga_agent.domain.model.estimate_session import EstimateFact, RoomScope, SessionState
from fga_agent.domain.model.plan_summary import PlanSummary, SummaryFixture, SummaryRoom
from fga_agent.service.estimate_reader import CORE, ITEMS, SCOPE, SETTINGS

RATE_CODES = ["demolition_finish", "brick_wall_100", "plaster", "paint", "floor_tile", "wall_tile", "ceiling", "skirting",
              "waterproofing", "door_interior", "door_entrance", "window", "electrical_point", "plumbing_point", "fixture_toilet",
              "fixture_basin", "fixture_shower", "fixture_bathtub", "fixture_sink", "fixture_kitchen_counter", "fixture_wardrobe",
              "excavation", "concrete", "rebar", "formwork", "roof_tile"]
MARKUPS = {"overhead_percent": 6.5, "profit_percent": 7.5, "design_percent": 5.5, "contingency_percent": 7.5, "vat_percent": 8.0}
ESTIMATE_LIMITS = DesignLimits(
    min_area={}, min_width={}, nestable_types=[], max_nested=2, default_fixtures={}, fixture_kinds=["toilet", "basin", "bed"],
    default_markups=MARKUPS, rate_catalog=[RateSpec(c, "p", c, "m2") for c in RATE_CODES],
    priced_fixtures=["basin", "bathtub", "kitchen_counter", "shower", "sink", "toilet", "wardrobe"])

ROOMS = [SummaryRoom("r1", "Living", "living", 22.0), SummaryRoom("r2", "Kitchen", "kitchen", 10.0),
         SummaryRoom("r3", "Bedroom 1", "bedroom", 12.0), SummaryRoom("r4", "Bedroom 2", "bedroom", 11.0),
         SummaryRoom("r5", "Bath", "bathroom", 4.5), SummaryRoom("r6", "Wc", "bathroom", 2.0), SummaryRoom("r7", "Balcony", "balcony", 5.0)]
FIXTURES = [SummaryFixture("f1", "toilet", "r5"), SummaryFixture("f2", "basin", "r5"), SummaryFixture("f3", "kitchen_counter", "r2"),
            SummaryFixture("f4", "bed", "r3")]  # a bed is loose furniture: never priced
SUMMARY = PlanSummary(rooms=ROOMS, fixtures=FIXTURES, pending_fixtures=["f1", "f2", "f3"])
NO_FIXTURES = PlanSummary(rooms=ROOMS)


def full_state(**over) -> SessionState:
    facts = dict(kind="apartment", work_type="fit_out", city="Ho Chi Minh City", quality="normal", floor_height=3.0,
                 mep_in_scope=False, **MARKUPS)
    facts.update(over)
    return SessionState(facts={k: EstimateFact(v, "user") for k, v in facts.items()},
                        scope={r.id: RoomScope("empty", "user") for r in ROOMS})


class ClaimsLanguageModel:
    """answer_json(): the next scripted reading, turned into the claims the model would return for each of the four readings
    of a message. A script entry is one message's reading in the reader's field names; its quotes are the whole message unless
    "evidence" ({field: quote}) is given, and then a field without an entry gets no claim."""

    def __init__(self, *readings) -> None:
        self.readings = list(readings)
        self.calls: list[dict] = []
        self.lock = threading.Lock()

    @staticmethod
    def _claims(data: dict, message: str, table: dict) -> list[dict]:
        evidence, out = data.get("evidence"), []
        for claim_field, (name, many) in table.items():
            v = data.get(name)
            if v is None or v == [] or (v is False and name in ("use_typical_markups", "fixtures_all_exist", "fixtures_none_exist")):
                continue
            quote = message if evidence is None else evidence.get(name)
            if quote is None:
                continue
            for x in (v if many else [v]):
                if name == "scope_rooms":
                    x = f"{x['room_id']}={x['state']}"
                out.append({"quote": quote, "field": claim_field, "value": str(x).lower() if isinstance(x, bool) else str(x)})
        return out

    def answer_json(self, system, user, schema, max_tokens=None, on_progress=None, think=True) -> dict:
        with self.lock:
            n = len(self.calls)
            self.calls.append({"system": system, "user": user, "think": think, "max_tokens": max_tokens})
        data = self.readings[n // 4] if n // 4 < len(self.readings) else {}
        message = user.split("<message>\n", 1)[1].rsplit("\n</message>", 1)[0]
        fields = schema["properties"]["claims"]["items"]["properties"]["field"]["enum"]
        table = next(t for t in (CORE, SETTINGS, SCOPE, ITEMS) if list(t) == fields)
        return {"claims": self._claims(data, message, table)}
