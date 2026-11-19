"""Composition root: the only place that builds the Agent's object graph. It imports the layers; no layer imports it."""

from __future__ import annotations

import os
from collections.abc import Callable
from dataclasses import dataclass

from .domain.model.call_context import CallContext
from .handler.grpc.blueprint_handler import BlueprintHandler, BlueprintUsecases
from .handler.grpc.servicer import AgentServicer
from .infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from .infrastructure.ollama import OllamaLanguageModel
from .service.brief_drafter import BriefDrafterService
from .service.brief_reviewer import BriefReviewerService
from .service.brief_reviser import BriefReviserService
from .service.helper.design_catalog_loader import DesignCatalogLoaderService
from .service.helper.feedback_asker import FeedbackAskerService
from .service.plan_reviser import PlanReviserService


@dataclass(frozen=True)
class App:
    """The built graph: the servicer that routes to the `<Domain>Handler`s (each built with its own use cases) and the
    start-up readiness check: the Engine's operations and limits, which the services need for their prompts, must be
    loaded before the Agent reports SERVING."""

    servicer: AgentServicer
    ready_check: Callable[[CallContext], None]


def build_app(engine_addr: str | None = None) -> App:
    engine = GrpcBlueprintEngine(open_channel(engine_addr or os.environ.get("ENGINE_ADDR", "localhost:8092")))
    llm = OllamaLanguageModel()
    design_catalog = DesignCatalogLoaderService(engine)
    asker = FeedbackAskerService(llm)
    reviewer = BriefReviewerService(engine, design_catalog, asker)
    blueprint = BlueprintHandler(BlueprintUsecases(
        brief_drafter=BriefDrafterService(engine, llm, design_catalog, asker),
        brief_reviser=BriefReviserService(engine, design_catalog, asker, reviewer),
        plan_reviser=PlanReviserService(engine, design_catalog, asker),
        brief_reviewer=reviewer))
    return App(servicer=AgentServicer(blueprint), ready_check=design_catalog.load)
