"""Composition root: the only place that builds the engine's object graph."""

from __future__ import annotations

from dataclasses import dataclass

from .infrastructure.plan_drawing import (EzdxfDxfPatching, EzdxfDxfReading, EzdxfPlanDrawing, EzdxfPlanRendering,
                                          RasterPlanConversion)
from .service.checks_runner import ChecksRunnerService
from .service.dxf_importer import DxfImporterService
from .service.dxf_patcher import DxfPatcherService
from .service.generator import GeneratorService
from .service.plan_converter import PlanConverterService
from .service.plan_creator import PlanCreatorService
from .service.plan_drawer import PlanDrawerService
from .service.plan_ops_applier import PlanOpsApplierService
from .service.plan_renderer import PlanRendererService
from .usecase.checks_runner import ChecksRunner
from .usecase.dxf_importer import DxfImporter
from .usecase.dxf_patcher import DxfPatcher
from .usecase.generator import Generator
from .usecase.plan_converter import PlanConverter
from .usecase.plan_creator import PlanCreator
from .usecase.plan_drawer import PlanDrawer
from .usecase.plan_ops_applier import PlanOpsApplier
from .usecase.plan_renderer import PlanRenderer


@dataclass(frozen=True)
class Engine:
    generator: Generator
    plan_creator: PlanCreator
    plan_drawer: PlanDrawer
    plan_ops_applier: PlanOpsApplier
    checks_runner: ChecksRunner
    plan_renderer: PlanRenderer
    dxf_importer: DxfImporter
    plan_converter: PlanConverter
    dxf_patcher: DxfPatcher


def build_engine() -> Engine:
    checks = ChecksRunnerService()
    rendering = EzdxfPlanRendering()
    generator = GeneratorService()
    drawer = PlanDrawerService(EzdxfPlanDrawing(), checks)
    return Engine(generator=generator, plan_creator=PlanCreatorService(generator, drawer), plan_drawer=drawer,
                  plan_ops_applier=PlanOpsApplierService(checks), checks_runner=checks,
                  plan_renderer=PlanRendererService(rendering), dxf_importer=DxfImporterService(EzdxfDxfReading(), rendering),
                  plan_converter=PlanConverterService(RasterPlanConversion()),
                  dxf_patcher=DxfPatcherService(EzdxfDxfPatching()))
