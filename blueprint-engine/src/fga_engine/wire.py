"""Composition root: the only place that builds the engine's object graph."""

from __future__ import annotations

from dataclasses import dataclass

from .infrastructure.plan_drawing import EzdxfPlanDrawing
from .service.checks_runner import ChecksRunnerService
from .service.generator import GeneratorService
from .service.plan_creator import PlanCreatorService
from .service.plan_drawer import PlanDrawerService
from .service.plan_ops_applier import PlanOpsApplierService
from .usecase.checks_runner import ChecksRunner
from .usecase.generator import Generator
from .usecase.plan_creator import PlanCreator
from .usecase.plan_drawer import PlanDrawer
from .usecase.plan_ops_applier import PlanOpsApplier


@dataclass(frozen=True)
class Engine:
    generator: Generator
    plan_creator: PlanCreator
    plan_drawer: PlanDrawer
    plan_ops_applier: PlanOpsApplier
    checks_runner: ChecksRunner


def build_engine() -> Engine:
    checks = ChecksRunnerService()
    generator = GeneratorService()
    drawer = PlanDrawerService(EzdxfPlanDrawing(), checks)
    return Engine(generator=generator, plan_creator=PlanCreatorService(generator, drawer), plan_drawer=drawer,
                  plan_ops_applier=PlanOpsApplierService(checks), checks_runner=checks)
