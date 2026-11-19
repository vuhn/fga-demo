from __future__ import annotations

import httpx

from ...domain.errors import SearchUnavailable
from ...domain.model.rate import RateRequest
from ...domain.model.search_hit import SearchHit
from ..entity.serper_search_result import SerperResult

URL = "https://google.serper.dev/search"


class SerperWebSearch:
    def __init__(self, key: str, http: httpx.Client | None = None) -> None:
        self._key, self._http = key, http or httpx.Client(timeout=15)

    def search(self, query: str, request: RateRequest, n: int = 5) -> list[SearchHit]:
        try:
            r = self._http.post(URL, json={"q": query, "num": n}, headers={"X-API-KEY": self._key})
            r.raise_for_status()
            rows = SerperResult.from_response(r.json())
        except (httpx.HTTPError, ValueError, KeyError) as e:
            raise SearchUnavailable(f"serper search: {e}") from e
        return [SearchHit(x.link, x.title, x.snippet) for x in rows[:n]]
