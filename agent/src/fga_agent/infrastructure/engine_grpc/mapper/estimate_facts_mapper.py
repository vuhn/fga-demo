"""Engine DTO for what takeoff, pricing and the plan summary read from the interview: domain -> `common.EstimateFacts`."""

from __future__ import annotations

from ....domain.model.estimate_facts import EstimateFacts
from fga.demo.common.v1 import common_pb2 as c

_SOURCE = {"user": c.FACT_SOURCE_USER, "plan": c.FACT_SOURCE_PLAN, "assumed": c.FACT_SOURCE_ASSUMED}
_STATE = {"empty": c.ROOM_STATE_EMPTY, "existing": c.ROOM_STATE_EXISTING, "to_replace": c.ROOM_STATE_TO_REPLACE,
          "new": c.ROOM_STATE_NEW}


def _fact(name: str, value, source: str) -> c.Fact:
    # bool before number: a flag is an int in Python
    if isinstance(value, bool):
        return c.Fact(name=name, flag=value, source=_SOURCE[source])
    if isinstance(value, (int, float)):
        return c.Fact(name=name, number=float(value), source=_SOURCE[source])
    return c.Fact(name=name, text=str(value), source=_SOURCE[source])


def facts_to_pb(x: EstimateFacts) -> c.EstimateFacts:
    return c.EstimateFacts(facts=[_fact(k, f.value, f.source) for k, f in x.facts.items()],
                           scope=[c.RoomScope(room_id=k, state=_STATE[s.state], source=_SOURCE[s.source]) for k, s in x.scope.items()],
                           fixtures_existing=x.fixtures_existing, excluded=x.excluded)
