from __future__ import annotations

from typing import Protocol

from ...domain.model.price_find import PriceFind
from ...domain.model.rate_range import RateRange


class PriceChecker(Protocol):
    """Every rule that decides whether a price the model read off a page may count. Page text can change a rate only
    through the model's extraction and these checks."""

    def finds_from(self, items: object) -> list[PriceFind]:
        """The well-formed finds in the model's answer (a positive price, a quote of 3 to 400 characters), at most six."""

    def tidy(self, text: str) -> str:
        """Whitespace collapsed to single spaces."""

    def quote_on_page(self, find: PriceFind, page: str) -> bool:
        """The quote is on the page and contains the digits of the price (so 150k can become 150000)."""

    def rate_in_vnd(self, find: PriceFind, want_unit: str) -> float | None:
        """The price as VND per `want_unit`, or None when the currency or the unit cannot be converted."""

    def plausible(self, prices: list[float]) -> list[bool]:
        """Which prices are within a factor of three of the median (the others are outliers)."""

    def range_of(self, prices: list[float]) -> RateRange:
        """3 or more prices: the 25th percentile, median and 75th percentile after dropping outliers; fewer: a wider
        range around the median, low_confidence; none: confidence none."""
