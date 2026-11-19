"""Blueprint endpoints of the Agent (BriefFromText, Revise, RevisePlan, Review): decode, call one use case, encode through mapper/. A refusal
before the first stream message is a gRPC status; after it, an in-band `Error` event."""

from __future__ import annotations

from collections.abc import Iterator
from dataclasses import dataclass

from ...usecase.brief_drafter import BriefDrafter
from ...usecase.brief_reviewer import BriefReviewer
from ...usecase.brief_reviser import BriefReviser
from ...usecase.plan_reviser import PlanReviser
from .common.error_helper import call_context, handled, refuse_before_stream
from .common.streaming_helper import ignore_progress, stream_run
from .mapper import blueprint_outcome_mapper, brief_mapper, error_mapper, progress_mapper


@dataclass(frozen=True)
class BlueprintUsecases:
    brief_drafter: BriefDrafter
    brief_reviser: BriefReviser
    plan_reviser: PlanReviser
    brief_reviewer: BriefReviewer


class BlueprintHandler:
    def __init__(self, usecases: BlueprintUsecases) -> None:
        self._uc = usecases

    @handled
    def BriefFromText(self, request, context):
        prompt, enhance = brief_mapper.draft_to_domain(request)
        return brief_mapper.draft_to_pb(self._uc.brief_drafter.draft(call_context(context), prompt, enhance, ignore_progress))

    def Revise(self, request, context) -> Iterator:
        ctx = call_context(context)
        brief, previous, message, history = blueprint_outcome_mapper.revise_to_domain(request)
        return stream_run(
            context,
            lambda on_progress: blueprint_outcome_mapper.to_event(
                self._uc.brief_reviser.revise(ctx, brief, previous, message, history, on_progress)),
            progress_mapper.to_blueprint_event, error_mapper.to_blueprint_error_event,
            lambda err: refuse_before_stream(context, err))

    def RevisePlan(self, request, context) -> Iterator:
        ctx = call_context(context)
        model, message, history = blueprint_outcome_mapper.revise_plan_to_domain(request)
        return stream_run(
            context,
            lambda on_progress: blueprint_outcome_mapper.to_event(
                self._uc.plan_reviser.revise(ctx, model, message, history, on_progress)),
            progress_mapper.to_blueprint_event, error_mapper.to_blueprint_error_event,
            lambda err: refuse_before_stream(context, err))

    def Review(self, request, context) -> Iterator:
        ctx = call_context(context)
        brief, previous, history, max_rounds = blueprint_outcome_mapper.review_to_domain(request)
        return stream_run(
            context,
            lambda on_progress: blueprint_outcome_mapper.to_event(
                self._uc.brief_reviewer.review(ctx, brief, previous, history, max_rounds, on_progress)),
            progress_mapper.to_blueprint_event, error_mapper.to_blueprint_error_event,
            lambda err: refuse_before_stream(context, err))
