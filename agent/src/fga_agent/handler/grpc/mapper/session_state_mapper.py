"""The estimate session <-> the JSON the API stores as opaque `state_json`. The shape is the one the Engine-era agent wrote, so a
session started before the Agent took over carries on."""

from __future__ import annotations

import json

from ....domain.errors import InvalidEdit
from ....domain.model.estimate_session import EstimateFact, RoomScope, SessionState
from . import estimate_mapper, rate_mapper


def from_json(data: bytes) -> SessionState:
    if not data:
        return SessionState()
    try:
        x = json.loads(data)
        return SessionState(
            facts={k: EstimateFact(v["value"], v["source"]) for k, v in x.get("facts", {}).items()},
            scope={k: RoomScope(v["state"], v["source"]) for k, v in x.get("scope", {}).items()},
            fixtures_existing=list(x.get("fixtures_existing", [])), excluded=list(x.get("excluded", [])),
            rate_overrides={k: float(v) for k, v in x.get("rate_overrides", {}).items()},
            rates=[rate_mapper.from_dict(r) for r in x.get("rates", [])], status=x.get("status", "interviewing"),
            pending=list(x.get("pending", [])), fingerprint=x.get("fingerprint", ""),
            estimate=estimate_mapper.from_document(x["estimate"]) if x.get("estimate") else None, turns=int(x.get("turns", 0)))
    except (ValueError, KeyError, TypeError, AttributeError) as e:
        raise InvalidEdit(f"state_json is not a valid estimate session: {e}") from e


def to_json(s: SessionState) -> bytes:
    doc = {"facts": {k: {"value": v.value, "source": v.source} for k, v in s.facts.items()},
           "scope": {k: {"state": v.state, "source": v.source} for k, v in s.scope.items()},
           "fixtures_existing": s.fixtures_existing, "excluded": s.excluded, "rate_overrides": s.rate_overrides,
           "rates": [rate_mapper.to_dict(r) for r in s.rates], "status": s.status, "pending": s.pending,
           "fingerprint": s.fingerprint, "estimate": estimate_mapper.to_document(s.estimate) if s.estimate else None, "turns": s.turns}
    return json.dumps(doc, ensure_ascii=False).encode()
