from __future__ import annotations

from urllib.parse import parse_qs, quote_plus, urlparse

from ...domain.model.rate import RateRequest
from ...domain.model.search_hit import SearchHit
from .price_list import BASE, HOST, page


class DemoWebSearch:
    """Three invented price lists for every rate code. The address carries what the page needs (the demo reads the request,
    a real provider gets nothing but the query)."""

    def search(self, query: str, request: RateRequest, n: int = 5) -> list[SearchHit]:
        if request.code not in BASE:
            return []
        args = f"?r={quote_plus(request.region)}&q={request.quality}&u={quote_plus(request.unit)}&n={quote_plus(request.description.split(',')[0])}"
        return [SearchHit(f"https://{HOST}/{request.code}/{i}{args}", f"Demo price list {i + 1}", "") for i in range(3)]


class DemoWebPages:
    def text(self, url: str) -> str | None:
        try:
            u = urlparse(url)
            if u.hostname != HOST:
                return None
            code, i = u.path.strip("/").split("/")
            args = {k: v[0] for k, v in parse_qs(u.query).items()}
            return page(code, int(i), args.get("r", ""), args.get("q", ""), args.get("u", ""), args.get("n", code))
        except (ValueError, KeyError):
            return None
