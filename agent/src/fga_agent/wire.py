"""Composition root: the only place that builds the Agent's object graph. It imports the layers; no layer imports it."""

from __future__ import annotations

import os
from collections.abc import Callable, Mapping
from dataclasses import dataclass

from .domain.model.call_context import CallContext
from .domain.gateway import WebPages, WebSearch
from .handler.grpc.blueprint_handler import BlueprintHandler, BlueprintUsecases
from .handler.grpc.estimate_handler import EstimateHandler, EstimateUsecases
from .handler.grpc.servicer import AgentServicer
from .infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from .infrastructure.demo import DemoWebPages, DemoWebSearch
from .infrastructure.memory import InMemoryRateCache
from .infrastructure.ollama import OllamaLanguageModel
from .infrastructure.web_page import HttpWebPages
from .infrastructure.web_search import BraveWebSearch, SerperWebSearch
from .service.brief_drafter import BriefDrafterService
from .service.brief_reviewer import BriefReviewerService
from .service.brief_reviser import BriefReviserService
from .service.estimate_interview import EstimateInterviewService
from .service.estimate_pricer import EstimatePricerService
from .service.estimate_reader import EstimateReaderService
from .service.estimate_replier import EstimateReplierService
from .service.estimate_turner import CEILING_SECONDS, EstimateTurnerService
from .service.helper.claim_checker import ClaimCheckerService
from .service.helper.design_catalog_loader import DesignCatalogLoaderService
from .service.helper.feedback_asker import FeedbackAskerService
from .service.helper.keyword_reader import KeywordReaderService
from .service.helper.price_checker import PriceCheckerService
from .service.helper.region_pinner import RegionPinnerService
from .service.plan_reviser import PlanReviserService
from .service.price_researcher import PriceResearcherService


@dataclass(frozen=True)
class App:
    """The built graph: the servicer that routes to the `<Domain>Handler`s (each built with its own use cases) and the
    start-up readiness check: the Engine's operations and limits, which the services need for their prompts, must be
    loaded before the Agent reports SERVING."""

    servicer: AgentServicer
    ready_check: Callable[[CallContext], None]


def _web(env: Mapping[str, str]) -> tuple[WebSearch | None, WebPages]:
    """The price source the environment names: `demo` (an invented list, labelled), `brave` or `serper` (need a key), else none."""
    name, key = env.get("ESTIMATE_SEARCH_PROVIDER", "none").lower(), env.get("ESTIMATE_SEARCH_API_KEY", "")
    if name == "demo":
        return DemoWebSearch(), DemoWebPages()
    if name == "brave" and key:
        return BraveWebSearch(key), HttpWebPages()
    if name == "serper" and key:
        return SerperWebSearch(key), HttpWebPages()
    return None, HttpWebPages()


def _estimate(env: Mapping[str, str], engine, llm, design_catalog) -> EstimateHandler:
    search, pages = _web(env)
    researcher = PriceResearcherService(
        llm, search, pages, InMemoryRateCache(), PriceCheckerService(float(env.get("ESTIMATE_USD_VND", "25000"))), RegionPinnerService(),
        design_catalog, parallel=int(env.get("ESTIMATE_SEARCH_PARALLEL", "3")))
    keywords, rules = KeywordReaderService(), env.get("ESTIMATE_RULES", "on") != "off"
    return EstimateHandler(EstimateUsecases(estimate_turner=EstimateTurnerService(
        engine, design_catalog, EstimateReaderService(llm, design_catalog, ClaimCheckerService(), keywords, rules),
        EstimateInterviewService(design_catalog, keywords, rules), EstimatePricerService(engine, design_catalog, researcher),
        EstimateReplierService(llm), ceiling_seconds=float(env.get("ESTIMATE_TURN_SECONDS", CEILING_SECONDS)))))


def build_app(engine_addr: str | None = None, env: Mapping[str, str] | None = None) -> App:
    env = os.environ if env is None else env
    engine = GrpcBlueprintEngine(open_channel(engine_addr or env.get("ENGINE_ADDR", "localhost:8092")))
    llm = OllamaLanguageModel()
    design_catalog = DesignCatalogLoaderService(engine)
    asker = FeedbackAskerService(llm)
    reviewer = BriefReviewerService(engine, design_catalog, asker)
    blueprint = BlueprintHandler(BlueprintUsecases(
        brief_drafter=BriefDrafterService(engine, llm, design_catalog, asker),
        brief_reviser=BriefReviserService(engine, design_catalog, asker, reviewer),
        plan_reviser=PlanReviserService(engine, design_catalog, asker),
        brief_reviewer=reviewer))
    return App(servicer=AgentServicer(blueprint, _estimate(env, engine, llm, design_catalog)), ready_check=design_catalog.load)
