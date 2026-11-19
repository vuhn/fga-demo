"""The adapters behind the price research: search providers, the page reader, the demo price list and the rate cache (no network:
httpx's mock transport answers)."""

import httpx
import pytest

from fga_agent.domain.errors import SearchUnavailable
from fga_agent.domain.model.rate import RateRequest, RateResult
from fga_agent.infrastructure.demo import DemoWebPages, DemoWebSearch
from fga_agent.infrastructure.memory import InMemoryRateCache
from fga_agent.infrastructure.web_page import HttpWebPages
from fga_agent.infrastructure.web_search import BraveWebSearch, SerperWebSearch

REQ = RateRequest(code="floor_tile", description="floor tile", unit="m2", region="Hanoi", quality="good")


def client(handler) -> httpx.Client:
    return httpx.Client(transport=httpx.MockTransport(handler), follow_redirects=True)


def test_brave_sends_only_the_query_and_strips_the_markup_from_snippets():
    seen = []

    def handler(request):
        seen.append(request)
        return httpx.Response(200, json={"web": {"results": [
            {"url": "https://a.test/1", "title": "A", "description": "giá <strong>gạch</strong> &amp; công"},
            {"url": "https://a.test/2"}, {"title": "no url"}]}})

    hits = BraveWebSearch("secret", client(handler)).search("giá gạch Hanoi", REQ, 5)
    assert [(h.url, h.title, h.snippet) for h in hits] == [("https://a.test/1", "A", "giá gạch & công"), ("https://a.test/2", "", "")]
    r = seen[0]
    assert r.headers["X-Subscription-Token"] == "secret" and dict(r.url.params) == {"q": "giá gạch Hanoi", "count": "5"}
    assert b"floor_tile" not in r.content and "floor" not in str(r.url)  # nothing of the request leaves, only the query


def test_serper_sends_only_the_query():
    seen = []

    def handler(request):
        seen.append(request)
        return httpx.Response(200, json={"organic": [{"link": "https://b.test/1", "title": "B", "snippet": "s"}, {"title": "no link"}]})

    hits = SerperWebSearch("k", client(handler)).search("floor tile price", REQ, 5)
    assert [(h.url, h.title, h.snippet) for h in hits] == [("https://b.test/1", "B", "s")]
    assert seen[0].headers["X-API-KEY"] == "k" and b'"q"' in seen[0].content and b"Hanoi" not in seen[0].content


@pytest.mark.parametrize("search", [BraveWebSearch, SerperWebSearch])
@pytest.mark.parametrize("response", [httpx.Response(429), httpx.Response(200, text="not json"), httpx.Response(200, json={"web": {"results": [{"title": "x"}]}, "organic": [{"title": "x"}]})])
def test_a_provider_that_cannot_serve_is_search_unavailable(search, response):
    s = search("k", client(lambda request: response))
    if response.status_code == 200 and response.headers.get("content-type", "").startswith("application/json"):
        assert s.search("q", REQ) == []  # a well-formed answer with nothing usable is just no hits
    else:
        with pytest.raises(SearchUnavailable):
            s.search("q", REQ)


def test_a_provider_that_is_unreachable_is_search_unavailable():
    def handler(request):
        raise httpx.ConnectError("refused")

    with pytest.raises(SearchUnavailable):
        BraveWebSearch("k", client(handler)).search("q", REQ)


def test_a_page_is_read_as_text_when_the_site_allows_it():
    html = "<html><style>p{}</style><script>evil()</script><body><p>Gạch&nbsp;lát   nền: 250.000đ/m2</p></body></html>"

    def handler(request):
        if request.url.path == "/robots.txt":
            return httpx.Response(200, text="User-agent: *\nDisallow: /private\n")
        return httpx.Response(200, text=html, headers={"content-type": "text/html; charset=utf-8"})

    pages = HttpWebPages(client(handler))
    assert pages.text("https://shop.test/gia") == "Gạch lát nền: 250.000đ/m2"
    assert pages.text("https://shop.test/private/gia") is None  # robots.txt disallows it


@pytest.mark.parametrize("url,response", [
    ("ftp://shop.test/x", httpx.Response(200, text="x", headers={"content-type": "text/plain"})),
    ("https://shop.test/x", httpx.Response(404)),
    ("https://shop.test/x", httpx.Response(200, content=b"%PDF", headers={"content-type": "application/pdf"})),
    ("https://shop.test/x", httpx.Response(200, content=b"x" * 2_000_001, headers={"content-type": "text/html"})),
])
def test_a_page_that_must_not_or_cannot_be_read_is_none(url, response):
    def handler(request):
        return httpx.Response(404) if request.url.path == "/robots.txt" else response

    assert HttpWebPages(client(handler)).text(url) is None


def test_an_unreachable_page_is_none():
    def handler(request):
        raise httpx.ConnectError("refused")

    assert HttpWebPages(client(handler)).text("https://shop.test/x") is None


def test_the_demo_list_is_three_pages_per_code_on_an_invalid_host_and_says_it_is_demo_data():
    hits = DemoWebSearch().search("anything", REQ)
    assert len(hits) == 3 and all(h.url.startswith("https://demo.invalid/floor_tile/") for h in hits)
    page = DemoWebPages().text(hits[0].url)
    assert page.startswith("[DEMO DATA, not a real price list]") and "Hanoi" in page and "đ/m2" in page
    assert DemoWebSearch().search("q", RateRequest("nothing", "x", "m2", "", "")) == []
    assert DemoWebPages().text("https://example.com/floor_tile/0") is None and DemoWebPages().text("https://demo.invalid/bad") is None


def test_a_cached_rate_is_served_while_fresh_and_per_code_region_and_quality():
    now = [1000.0]
    cache = InMemoryRateCache(ttl_seconds=100, clock=lambda: now[0])
    rate = RateResult(code="floor_tile", unit="m2", base=5, confidence="ok", region="Hanoi", quality="good")
    cache.put(rate)
    assert cache.get("floor_tile", "Hanoi", "good") is rate
    assert cache.get("floor_tile", "Hanoi", "normal") is None and cache.get("floor_tile", "Da Nang", "good") is None
    now[0] = 1099.0
    assert cache.get("floor_tile", "Hanoi", "good") is rate
    now[0] = 1100.0
    assert cache.get("floor_tile", "Hanoi", "good") is None  # the seven days are over


def test_the_default_cache_keeps_a_rate_for_seven_days():
    from fga_agent.infrastructure.memory.rate_cache import SEVEN_DAYS

    assert SEVEN_DAYS == 7 * 24 * 3600
