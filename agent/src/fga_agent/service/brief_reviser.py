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
from ..usecase.brief_reviewer import BriefReviewer
from ..usecase.helper.feedback_asker import FeedbackAsker
from .prompts import architect, conversation, operations

MAX_MESSAGE = 2000


class BriefReviserService:
    """The model names operations; the Engine applies them, rebuilds the layout and runs the checks in one call. When the
    edit introduces errors the plan did not have, one review round tries to fix them."""

    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, asker: FeedbackAsker,
                 reviewer: BriefReviewer) -> None:
        self._engine, self._catalog, self._asker, self._reviewer = engine, catalog, asker, reviewer

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
        outcome = BlueprintOutcome(brief=built.brief, model=built.model, checks=built.checks, issues=built.issues,
                                   ok=built.ok, notes=notes + built.notes, llm_attempts=tries, history=done)
        if not self._introduced_errors(ctx, brief, previous_model, built):
            return outcome
        on_progress(Progress("step", "review", "The edit introduced new errors; running one review round"))
        rev = self._reviewer.review(ctx, built.brief, previous_model, [*history, message], 1, on_progress)
        return BlueprintOutcome(brief=rev.brief, model=rev.model, checks=rev.checks, issues=rev.issues, ok=rev.ok,
                                notes=outcome.notes + rev.notes, llm_attempts=tries + rev.llm_attempts,
                                history=done + rev.history)

    def _introduced_errors(self, ctx: CallContext, brief: bytes, previous_model: bytes, built: BriefBuild) -> bool:
        """True when the edit cannot be built, or fails an error rule the brief before it did not."""
        if built.model and not [c for c in built.checks if not c.ok and c.severity == "error"]:
            return False
        before = self._engine.generate(ctx, brief, previous_model)
        return not built.model or bool(set(built.error_rules) - set(before.error_rules))
