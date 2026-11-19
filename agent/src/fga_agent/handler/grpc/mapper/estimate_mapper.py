"""A priced estimate <-> its JSON document (the Engine's own format, shown and stored unchanged)."""

from __future__ import annotations

import json

from ....domain.model.estimate import Estimate, EstimateLine
from . import rate_mapper


def _line(x: dict) -> EstimateLine:
    return EstimateLine(code=x["code"], package=x.get("package", ""), description=x.get("description", ""), quantity=x.get("quantity", 0),
                        unit=x.get("unit", ""), basis=x.get("basis", ""), confidence=x.get("confidence", "assumed"),
                        priced=x.get("priced", True), rate_base=x.get("rate_base"), rate_confidence=x.get("rate_confidence", "none"),
                        low=x.get("low", 0), base=x.get("base", 0), high=x.get("high", 0))


def from_document(doc: dict) -> Estimate:
    return Estimate(total=doc.get("total", {}), packages=doc.get("packages", {}), assumptions=doc.get("assumptions", []),
                    warnings=doc.get("warnings", []), lines=[_line(x) for x in doc.get("lines", [])],
                    rates=[rate_mapper.from_dict(x) for x in doc.get("rates", [])], document=doc)


def to_document(est: Estimate) -> dict:
    return est.document


def to_json(est: Estimate) -> bytes:
    return json.dumps(est.document, ensure_ascii=False).encode()
