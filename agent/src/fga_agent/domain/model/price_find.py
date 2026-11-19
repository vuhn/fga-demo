from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class PriceFind:
    """A price the model read off a page, as written: nothing in it is trusted until the checks pass."""

    price: float
    currency: str
    unit: str
    quote: str
    material_spec: str = ""
    date: str = ""
