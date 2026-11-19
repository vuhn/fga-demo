"""Plan endpoints (CreatePlan, DrawPlan, ApplyPlanOps, RunChecks, SummarizePlan). Decode, call one use case, encode."""

from __future__ import annotations

from dataclasses import dataclass

from ...usecase.checks_runner import ChecksRunner
from ...usecase.plan_creator import PlanCreator
from ...usecase.plan_drawer import PlanDrawer
from ...usecase.plan_ops_applier import PlanOpsApplier
from ...usecase.plan_summarizer import PlanSummarizer
from .common.error_helper import handled
from .mapper import check_mapper, plan_mapper, plan_summary_mapper


@dataclass(frozen=True)
class PlanUsecases:
    plan_creator: PlanCreator
    plan_drawer: PlanDrawer
    plan_ops_applier: PlanOpsApplier
    checks_runner: ChecksRunner
    plan_summarizer: PlanSummarizer


class PlanHandler:
    def __init__(self, usecases: PlanUsecases) -> None:
        self._uc = usecases

    @handled
    def CreatePlan(self, request, context):
        return plan_mapper.outcome_to_pb(self._uc.plan_creator.create_plan(plan_mapper.create_to_domain(request)))

    @handled
    def DrawPlan(self, request, context):
        model, title = plan_mapper.draw_to_domain(request)
        return plan_mapper.outcome_to_pb(self._uc.plan_drawer.draw_plan(model, title))

    @handled
    def ApplyPlanOps(self, request, context):
        model, ops = plan_mapper.edit_to_domain(request)
        return plan_mapper.edit_to_pb(self._uc.plan_ops_applier.edit_plan(model, ops))

    @handled
    def RunChecks(self, request, context):
        model, brief = check_mapper.run_checks_to_domain(request)
        return check_mapper.report_to_pb(self._uc.checks_runner.check_plan(model, brief))

    @handled
    def SummarizePlan(self, request, context):
        model, facts = plan_summary_mapper.to_domain(request)
        return plan_summary_mapper.to_pb(self._uc.plan_summarizer.summarize_plan(model, facts))
