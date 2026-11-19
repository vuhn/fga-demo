from __future__ import annotations

import json
from collections.abc import Callable

from ..domain.errors import InvalidEdit
from ..domain.gateway import BlueprintEngine
from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.model.call_context import CallContext
from ..domain.model.plan import PlanRevision
from ..domain.model.progress import Progress
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.feedback_asker import FeedbackAsker
from .prompts import conversation, operations, review

MAX_MESSAGE = 2000


class PlanReviserService:
    """The model names plan operations; the Engine applies them to the plan as drawn and runs the checks."""

    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, asker: FeedbackAsker) -> None:
        self._engine, self._catalog, self._asker = engine, catalog, asker

    def revise(self, ctx: CallContext, model: bytes, message: str, history: list[str],
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        if not message.strip():
            raise InvalidEdit("say what to change")
        catalog, limits = self._catalog.operations(ctx), self._catalog.limits(ctx)  # a down Engine refuses before any step
        system = conversation.plan_chat_instructions() + "\n" + operations.ops_help(catalog, "plan_op", limits.fixture_kinds)
        user = (f"{conversation.history_block(history)}Current plan: {conversation.plan_for_prompt(model)}\n"
                f"Request: {message[:MAX_MESSAGE]}")

        def apply(ops: dict) -> PlanRevision:
            return self._engine.apply_plan_ops(ctx, model, json.dumps(ops).encode())

        ops, revised, tries = self._asker.ask(ctx, system, user, catalog.plan_ops_schema, apply, on_progress)
        done = "; ".join(revised.descriptions)
        on_progress(Progress("step", "ops", "The model proposed: " + done))
        on_progress(Progress("step", "checks", "Running the checks…"))
        notes = ["applied: " + done]
        if tries > 1:
            notes.append(f"the first {tries - 1} answer(s) could not be applied; corrected with the error")
        return BlueprintOutcome(brief=b"", model=revised.model, checks=revised.checks, issues=[], ok=revised.ok,
                                notes=notes, llm_attempts=tries, history=revised.descriptions)
