"""Composition root: the only place that builds the Agent's object graph. It imports the layers; no layer imports it."""

from __future__ import annotations

import os
from collections.abc import Callable
from dataclasses import dataclass

from .domain.model.call_context import CallContext
from .infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from .service.helper.design_catalog_loader import DesignCatalogLoaderService


@dataclass(frozen=True)
class App:
    """The built graph. T10 has no endpoints yet, so it holds only the start-up readiness check: the Engine's
    operations and limits, which the services need for their prompts, must be loaded before the Agent reports
    SERVING. From T11a it also holds the `<Domain>Handler`s, each built with its own use cases."""

    ready_check: Callable[[CallContext], None]


def build_app(engine_addr: str | None = None) -> App:
    engine = GrpcBlueprintEngine(open_channel(engine_addr or os.environ.get("ENGINE_ADDR", "localhost:8092")))
    design_catalog = DesignCatalogLoaderService(engine)
    return App(ready_check=design_catalog.load)
