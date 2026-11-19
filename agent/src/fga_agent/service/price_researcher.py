from __future__ import annotations

from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import date

from ..domain.errors import InvalidEdit, LlmUnavailable, SearchUnavailable
from ..domain.gateway import LanguageModel, WebPages, WebSearch
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress
from ..domain.model.rate import RateRequest, RateResult, RateSource
from ..domain.model.search_hit import SearchHit
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.price_checker import PriceChecker
from ..usecase.helper.region_pinner import RegionPinner
from .prompts import price_research as prompt

NO_PROVIDER = "no search provider is configured (set ESTIMATE_SEARCH_PROVIDER and ESTIMATE_SEARCH_API_KEY)"
MAX_SOURCES = 6


class PriceResearcherService:
    """`search=None` means no provider is configured: every rate comes back unpriced and the interview asks the person."""

    def __init__(self, llm: LanguageModel, search: WebSearch | None, pages: WebPages, checker: PriceChecker,
                 pinner: RegionPinner, catalog: DesignCatalogLoader, parallel: int = 3, max_pages: int = 4,
                 today: Callable[[], str] | None = None) -> None:
        self._llm, self._search, self._pages = llm, search, pages
        self._checker, self._pinner, self._catalog = checker, pinner, catalog
        self._parallel, self._max_pages = max(1, parallel), max_pages
        self._today = today or (lambda: date.today().isoformat())

    def requests_for(self, ctx: CallContext, codes: list[str], region: str, quality: str) -> list[RateRequest]:
        specs = {s.code: s for s in self._catalog.limits(ctx).rate_catalog}
        out = []
        for code in codes:
            if code not in specs:
                raise InvalidEdit(f"unknown rate code {code}")
            spec = specs[code]
            out.append(RateRequest(code=code, description=prompt.item_description(spec, quality), unit=spec.unit, region=region,
                                   quality=quality if spec.quality_sensitive else ""))
        return out

    def research(self, ctx: CallContext, requests: list[RateRequest], on_progress: Callable[[Progress], None],
                 on_rate: Callable[[RateResult], None] | None = None) -> list[RateResult]:
        results: dict[str, RateResult] = {}
        todo = list(requests)  # no cache: every estimate researches its prices afresh
        if self._search is None:
            for r in todo:
                results[r.code] = self._unpriced(r, NO_PROVIDER)
                self._heard(on_rate, results[r.code])
        elif todo:
            self._research_all(todo, on_progress, on_rate, results)
        return [results[r.code] for r in requests]

    def _research_all(self, todo: list[RateRequest], on_progress: Callable[[Progress], None],
                      on_rate: Callable[[RateResult], None] | None, results: dict[str, RateResult]) -> None:
        with ThreadPoolExecutor(max_workers=self._parallel) as pool:
            futures = {pool.submit(self._research_one, r, on_progress): r for r in todo}
            for f in as_completed(futures):
                r = futures[f]
                try:
                    res = f.result()  # Cancelled from the progress callback is not caught: the run stops
                except (LlmUnavailable, SearchUnavailable) as e:
                    res = self._unpriced(r, f"research failed: {str(e)[:120]}")
                results[r.code] = res
                self._heard(on_rate, res)

    @staticmethod
    def _heard(on_rate: Callable[[RateResult], None] | None, result: RateResult) -> None:
        if on_rate:
            on_rate(result)

    @staticmethod
    def _unpriced(r: RateRequest, note: str) -> RateResult:
        return RateResult(code=r.code, unit=r.unit, region=r.region, quality=r.quality, confidence="none", note=note)

    @staticmethod
    def _each(fn, items):
        """fn over items on threads, results in input order; a failing call raises as it would in a plain loop."""
        items = list(items)
        if len(items) < 2:
            return [fn(i) for i in items]
        with ThreadPoolExecutor(max_workers=len(items)) as pool:
            return list(pool.map(fn, items))

    def _research_one(self, r: RateRequest, on_progress: Callable[[Progress], None]) -> RateResult:
        on_progress(Progress("step", "research", prompt.searching(r)))
        hits = self._hits(r, self._queries(r))
        top = hits[: self._max_pages]
        # pages are fetched at once (network); the model reads them one after another, as the local model serves one request at a time
        pages = self._each(lambda h: self._pages.text(h.url) or self._checker.tidy(f"{h.title}. {h.snippet}"), top)
        found = self._found(r, top, pages)
        prices, sources = [p for p, _ in found], [s for _, s in found]
        rng = self._checker.range_of(prices)
        if rng.confidence == "none":
            return self._unpriced(r, "no usable price found on the pages searched")
        return RateResult(code=r.code, unit=r.unit, low=float(round(rng.low)), base=float(round(rng.base)), high=float(round(rng.high)),
                          confidence=rng.confidence, region=r.region, quality=r.quality, sources=sources[:MAX_SOURCES],
                          note=f"{len(prices)} price(s) from {len({s.url for s in sources})} page(s)")

    def _queries(self, r: RateRequest) -> list[str]:
        asked = self._llm.answer_json(prompt.QUERY_SYSTEM, prompt.query_user(r), prompt.queries_schema(), think=False)
        raw = asked.get("queries")
        written = [q for q in raw if isinstance(q, str)][:2] if isinstance(raw, list) else []
        pinned = [q for q in (self._pinner.pin(x, r.region) for x in written) if q]
        return pinned or [prompt.fallback_query(r)]

    def _hits(self, r: RateRequest, queries: list[str]) -> list[SearchHit]:
        assert self._search is not None
        hits: list[SearchHit] = []
        seen: set[str] = set()
        for found in self._each(lambda q: self._search.search(q, r, 5), queries):  # searches run at once, order kept
            for h in found:
                if h.url not in seen:
                    seen.add(h.url)
                    hits.append(h)
        return hits

    def _found(self, r: RateRequest, hits: list[SearchHit], pages: list[str]) -> list[tuple[float, RateSource]]:
        found: list[tuple[float, RateSource]] = []
        for h, page in zip(hits, pages):
            if not page:
                continue
            asked = self._llm.answer_json(prompt.EXTRACT_SYSTEM, prompt.extract_user(r, page), prompt.finds_schema(), think=False)
            for f in self._checker.finds_from(asked.get("items")):
                rate = self._checker.rate_in_vnd(f, r.unit)
                if rate is None or not self._checker.quote_on_page(f, page):
                    continue
                found.append((rate, RateSource(url=h.url, quote=self._checker.tidy(f.quote)[:300], retrieved_at=self._today())))
        keep = self._checker.plausible([p for p, _ in found])  # a price far from the median is dropped, and so is its source
        return [x for x, ok in zip(found, keep) if ok]
