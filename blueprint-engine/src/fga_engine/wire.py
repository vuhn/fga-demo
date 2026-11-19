"""Composition root: the only place that builds the engine's object graph."""

from __future__ import annotations

from dataclasses import dataclass

from .handler.grpc.brief_handler import BriefHandler, BriefUsecases
from .handler.grpc.catalog_handler import CatalogHandler, CatalogUsecases
from .handler.grpc.estimate_handler import EstimateHandler, EstimateUsecases
from .handler.grpc.file_handler import FileHandler, FileUsecases
from .handler.grpc.plan_handler import PlanHandler, PlanUsecases
from .handler.grpc.servicer import EngineServicer
from .infrastructure.plan_drawing import (EzdxfDxfPatching, EzdxfDxfReading, EzdxfPlanDrawing, EzdxfPlanRendering,
                                          RasterPlanConversion)
from .service.brief_ops_applier import BriefOpsApplierService
from .service.draft_normalizer import DraftNormalizerService
from .service.tool_catalog import ToolCatalogService
from .service.checks_runner import ChecksRunnerService
from .service.dxf_importer import DxfImporterService
from .service.dxf_patcher import DxfPatcherService
from .service.generator import GeneratorService
from .service.plan_converter import PlanConverterService
from .service.plan_creator import PlanCreatorService
from .service.plan_drawer import PlanDrawerService
from .service.plan_ops_applier import PlanOpsApplierService
from .service.plan_summarizer import PlanSummarizerService
from .service.price_estimate import PriceEstimateService
from .service.takeoff import TakeoffService
from .service.plan_renderer import PlanRendererService
from .usecase.brief_ops_applier import BriefOpsApplier
from .usecase.draft_normalizer import DraftNormalizer
from .usecase.tool_catalog import ToolCatalog
from .usecase.checks_runner import ChecksRunner
from .usecase.dxf_importer import DxfImporter
from .usecase.dxf_patcher import DxfPatcher
from .usecase.generator import Generator
from .usecase.plan_converter import PlanConverter
from .usecase.plan_creator import PlanCreator
from .usecase.plan_drawer import PlanDrawer
from .usecase.plan_ops_applier import PlanOpsApplier
from .usecase.plan_summarizer import PlanSummarizer
from .usecase.price_estimate import EstimatePricer
from .usecase.takeoff import QuantityTakeoff
from .usecase.plan_renderer import PlanRenderer


@dataclass(frozen=True)
class Engine:
    generator: Generator
    tool_catalog: ToolCatalog
    draft_normalizer: DraftNormalizer
    brief_ops_applier: BriefOpsApplier
    plan_creator: PlanCreator
    plan_drawer: PlanDrawer
    plan_ops_applier: PlanOpsApplier
    checks_runner: ChecksRunner
    plan_summarizer: PlanSummarizer
    takeoff: QuantityTakeoff
    estimate_pricer: EstimatePricer
    plan_renderer: PlanRenderer
    dxf_importer: DxfImporter
    plan_converter: PlanConverter
    dxf_patcher: DxfPatcher


def build_engine() -> Engine:
    checks = ChecksRunnerService()
    rendering = EzdxfPlanRendering()
    generator = GeneratorService()
    drawer = PlanDrawerService(EzdxfPlanDrawing(), checks)
    return Engine(generator=generator, tool_catalog=ToolCatalogService(), draft_normalizer=DraftNormalizerService(),
                  brief_ops_applier=BriefOpsApplierService(generator), plan_creator=PlanCreatorService(generator, drawer), plan_drawer=drawer,
                  plan_ops_applier=PlanOpsApplierService(checks), checks_runner=checks,
                  plan_summarizer=PlanSummarizerService(), takeoff=TakeoffService(), estimate_pricer=PriceEstimateService(),
                  plan_renderer=PlanRendererService(rendering), dxf_importer=DxfImporterService(EzdxfDxfReading(), rendering),
                  plan_converter=PlanConverterService(RasterPlanConversion()),
                  dxf_patcher=DxfPatcherService(EzdxfDxfPatching()))


def build_servicer(engine: Engine | None = None) -> EngineServicer:
    """The gRPC delivery layer: one handler per domain, each with only the use cases it calls."""
    e = engine or build_engine()
    return EngineServicer(
        CatalogHandler(CatalogUsecases(tool_catalog=e.tool_catalog)),
        BriefHandler(BriefUsecases(draft_normalizer=e.draft_normalizer, generator=e.generator,
                                   brief_ops_applier=e.brief_ops_applier)),
        PlanHandler(PlanUsecases(plan_creator=e.plan_creator, plan_drawer=e.plan_drawer,
                                 plan_ops_applier=e.plan_ops_applier, checks_runner=e.checks_runner,
                                 plan_summarizer=e.plan_summarizer)),
        EstimateHandler(EstimateUsecases(takeoff=e.takeoff, estimate_pricer=e.estimate_pricer)),
        FileHandler(FileUsecases(plan_converter=e.plan_converter, dxf_importer=e.dxf_importer,
                                 dxf_patcher=e.dxf_patcher, plan_renderer=e.plan_renderer)))
