from __future__ import annotations

import copy
import time
from collections.abc import Callable
from dataclasses import replace
from typing import Any

from ..domain.errors import TurnTimedOut
from ..domain.gateway import BlueprintEngine
from ..domain.model.call_context import CallContext
from ..domain.model.estimate import Estimate, EstimateLine
from ..domain.model.estimate_session import FactChange, Question, SessionState
from ..domain.model.estimate_turn import EstimateTurnRequest, EstimateTurnResult
from ..domain.model.plan_summary import PlanSummary
from ..domain.model.progress import Progress
from ..usecase.estimate_interview import EstimateInterview
from ..usecase.estimate_pricer import EstimatePricer
from ..usecase.estimate_reader import EstimateReader
from ..usecase.estimate_replier import EstimateReplier
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from .prompts import estimate_questions as questions
from .prompts import estimate_reply as prompt

CEILING_SECONDS = 30 * 60
EmitEvent = Callable[[Any], None]


class EstimateTurnerService:
    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, reader: EstimateReader, interview: EstimateInterview,
                 pricer: EstimatePricer, replier: EstimateReplier, ceiling_seconds: float = CEILING_SECONDS,
                 clock: Callable[[], float] = time.monotonic) -> None:
        self._engine, self._catalog, self._reader, self._interview = engine, catalog, reader, interview
        self._pricer, self._replier, self._ceiling, self._clock = pricer, replier, ceiling_seconds, clock

    def turn(self, ctx: CallContext, request: EstimateTurnRequest, on_event: EmitEvent) -> EstimateTurnResult:
        emit = self._within_ceiling(on_event)
        state = copy.deepcopy(request.state)
        state.turns += 1
        self._take_rates(ctx, state, request.rate_overrides)
        summary = self._engine.summarize_plan(ctx, request.model, self._interview.facts(state))
        state, changes, explain, summary = self._listen(ctx, request, state, summary, emit)
        missing = self._interview.missing(ctx, state, summary)
        state.pending = [q.id for q in missing]
        if missing:
            return self._ask(ctx, state, missing[0], emit)
        facts = self._interview.facts(state)
        takeoff = self._engine.takeoff(ctx, request.model, facts)
        if takeoff.status == "unsupported":
            state.status = "failed"
            reason = takeoff.warnings or ["this plan cannot be estimated"]
            return EstimateTurnResult(state, self._replier.say(ctx, prompt.UNSUPPORTED, {"reason": reason}, reason[0], emit))
        fingerprint = self._interview.fingerprint(state)
        if state.estimate is None or fingerprint != state.fingerprint:
            return self._priced(ctx, state, facts, takeoff, fingerprint, changes, emit)
        return self._unchanged(ctx, state, request.message, explain, emit)

    def _within_ceiling(self, on_event: EmitEvent) -> EmitEvent:
        started = self._clock()

        def emit(event: Any) -> None:
            if self._clock() - started > self._ceiling:
                raise TurnTimedOut(f"the estimate turn ran longer than {self._ceiling / 60:g} minutes")
            on_event(event)

        return emit

    def _take_rates(self, ctx: CallContext, state: SessionState, overrides: dict[str, float]) -> None:
        codes = {s.code for s in self._catalog.limits(ctx).rate_catalog}
        for code, value in overrides.items():
            if code in codes and value > 0:
                state.rate_overrides[code] = value

    def _listen(self, ctx: CallContext, request: EstimateTurnRequest, state: SessionState, summary: PlanSummary,
                emit: EmitEvent) -> tuple[SessionState, list[FactChange], str | None, PlanSummary]:
        """What the message states, recorded in the session; the plan is summarised again under the new facts."""
        if not request.message.strip():
            return state, [], None, summary
        emit(Progress("step", "understand", "reading your message"))
        extracted = self._reader.read(ctx, summary, state, request.message, request.messages)
        state, changes = self._interview.record(ctx, state, summary, extracted, request.message)
        for change in changes:
            emit(change)
        return state, changes, extracted.explain, self._engine.summarize_plan(ctx, request.model, self._interview.facts(state))

    def _ask(self, ctx: CallContext, state: SessionState, first: Question, emit: EmitEvent) -> EstimateTurnResult:
        state.status = "interviewing"
        question = replace(first, options=[*first.options, questions.UNKNOWN_OPTION]) if first.allow_unknown else first
        known = {k: v.value for k, v in state.facts.items()}
        reply = self._replier.say(ctx, prompt.ASK, {"question": question.text, "known": known}, question.text, emit)
        emit(question)
        return EstimateTurnResult(state, reply, question=question)

    def _priced(self, ctx: CallContext, state: SessionState, facts, takeoff, fingerprint: str, changes: list[FactChange],
                emit: EmitEvent) -> EstimateTurnResult:
        state, estimate = self._pricer.price(ctx, state, facts, takeoff, emit)
        state.estimate, state.fingerprint, state.status = estimate, fingerprint, "ready"
        emit(estimate)
        said = prompt.summary(estimate)
        fallback = prompt.estimate_fallback(said)
        if changes:
            said = {**said, "changes_applied_this_turn": [f"{c.name} = {c.value}" for c in changes]}
        return EstimateTurnResult(state, self._replier.say(ctx, prompt.PRESENT, said, fallback, emit), estimate=estimate)

    def _unchanged(self, ctx: CallContext, state: SessionState, message: str, explain: str | None, emit: EmitEvent) -> EstimateTurnResult:
        state.status = "ready"
        estimate = state.estimate
        assert estimate is not None
        line = self._line_named(estimate, explain) if explain else None
        if line is not None:
            rate = next((r for r in estimate.rates if r.code == line.code), None)
            reply = self._replier.say(ctx, prompt.EXPLAIN, prompt.line_context(line, rate), prompt.line_fallback(line), emit)
        else:
            reply = self._replier.say(ctx, prompt.NOTHING_CHANGED, {"message": message, **prompt.summary(estimate)},
                                      prompt.NOTHING_CHANGED_FALLBACK, emit)
        return EstimateTurnResult(state, reply)

    @staticmethod
    def _line_named(estimate: Estimate, key: str) -> EstimateLine | None:
        k = key.casefold()
        return next((ln for ln in estimate.lines if k == ln.code or k in ln.description.casefold() or k in ln.code.replace("_", " ")), None)
