"""Composition root: the only place that builds the Agent's object graph."""

from __future__ import annotations

import os
from dataclasses import dataclass

from .domain.gateway import CatalogSource, LLM, PlanTools, ToolRegistry
from .infrastructure.engine_grpc.catalog import CatalogLoader
from .infrastructure.engine_grpc.client import EngineTools, open_channel
from .infrastructure.ollama import OllamaClient


@dataclass(frozen=True)
class Agent:
    llm: LLM
    plan_tools: PlanTools
    registry: ToolRegistry
    catalog: CatalogSource


def build_agent(engine_addr: str | None = None) -> Agent:
    addr = engine_addr or os.environ.get("ENGINE_ADDR", "localhost:8092")
    tools = EngineTools(open_channel(addr))
    return Agent(llm=OllamaClient(), plan_tools=tools, registry=tools, catalog=CatalogLoader(tools))
