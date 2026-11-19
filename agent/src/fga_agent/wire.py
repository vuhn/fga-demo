"""Composition root: the only place that builds the Agent's object graph."""

from __future__ import annotations

import os
from dataclasses import dataclass

from .infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from .service.design_catalog_loader import DesignCatalogLoaderService
from .usecase.design_catalog_loader import DesignCatalogLoader


@dataclass(frozen=True)
class Usecases:
    """What the handlers call. Each field is a use case interface; the brief, plan and estimate use cases join it
    in T11a onward."""

    design_catalog: DesignCatalogLoader


def build_usecases(engine_addr: str | None = None) -> Usecases:
    engine = GrpcBlueprintEngine(open_channel(engine_addr or os.environ.get("ENGINE_ADDR", "localhost:8092")))
    return Usecases(design_catalog=DesignCatalogLoaderService(engine))
