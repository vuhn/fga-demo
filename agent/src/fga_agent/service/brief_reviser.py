from __future__ import annotations

import json
from collections.abc import Callable

from ..domain.errors import InvalidEdit
from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.gateway import BlueprintEngine
from ..domain.model.brief import BriefBuild
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.feedback_asker import FeedbackAsker
from .prompts import architect, conversation, operations

MAX_MESSAGE = 2000


class BriefReviserService:
    """The model names operations; the Engine applies them, rebuilds the layout and runs the checks in one call.
    The automatic review round after an edit that introduces errors comes with the reviewer (T11b)."""

    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, asker: FeedbackAsker) -> None:
        self._engine, self._catalog, self._asker = engine, catalog, asker

    def revise(self, ctx: CallContext, brief: bytes, previous_model: bytes, message: str, history: list[str],
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        if not message.strip():
            raise InvalidEdit("say what to change")
        catalog, limits = self._catalog.operations(ctx), self._catalog.limits(ctx)  # a down Engine refuses before any step
        system = ("You turn a revision request for a house plan into edit operations.\n"
                  + operations.ops_help(catalog, "brief_op", limits.fixture_kinds) + "\n" + architect.architect_rules(limits))
        user = (f"{conversation.history_block(history)}Current brief: {conversation.brief_for_prompt(brief)}\n"
                f"Request: {message[:MAX_MESSAGE]}")

        def apply(ops: dict) -> BriefBuild:
            return self._engine.apply_brief_ops(ctx, brief, json.dumps(ops).encode(), previous_model)

        ops, built, tries = self._asker.ask(ctx, system, user, catalog.brief_ops_schema, apply, on_progress)
        done = [conversation.describe_op(o) for o in ops.get("ops", [])]
        on_progress(Progress("step", "ops", "The model proposed: " + "; ".join(done)))
        on_progress(Progress("step", "layout", "Rebuilding the layout and running the checks…"))
        on_progress(Progress("step", "checks", built.checks_line))
        notes = ["applied: " + "; ".join(done)]
        if tries > 1:
            notes.append(f"the first {tries - 1} answer(s) could not be applied; corrected with the error")
        return BlueprintOutcome(brief=built.brief, model=built.model, checks=built.checks, issues=built.issues,
                                ok=built.ok, notes=notes + built.notes, llm_attempts=tries, history=done)
