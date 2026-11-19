from __future__ import annotations

from typing import Protocol

from ..model.rate import RateRequest
from ..model.search_hit import SearchHit


class WebSearch(Protocol):
    def search(self, query: str, request: RateRequest, n: int = 5) -> list[SearchHit]:
        """The top `n` hits for the query. A real provider is sent nothing but the query; `request` is the rate being
        researched, for the demo price list only. SearchUnavailable when the provider cannot serve."""
