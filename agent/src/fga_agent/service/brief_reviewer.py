from __future__ import annotations

import json
from collections.abc import Callable

from ..domain.errors import InvalidEdit
from ..domain.gateway import BlueprintEngine
from ..domain.model.blueprint_outcome import BlueprintOutcome
from ..domain.model.brief import BriefBuild
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.feedback_asker import FeedbackAsker
from .prompts import architect, operations, review

# The person's intent is fixed: these operations change it, so the reviewer may not use them.
FORBIDDEN_OPS = frozenset({"remove_room", "set_lot", "set_entrance", "unsupported", "rename_room", "set_open_plan"})
AREA_CAP = 0.15  # the total target area moves at most 15%
MAX_ROUNDS = 3
ATTEMPTS = 3


class BriefReviewerService:
    """Each round: the model proposes operations for the issues, the Engine applies and builds them. A round is kept only
    when the issue score drops and no new kind of error appears; the first refused round ends the review."""

    def __init__(self, engine: BlueprintEngine, catalog: DesignCatalogLoader, asker: FeedbackAsker) -> None:
        self._engine, self._catalog, self._asker = engine, catalog, asker

    def review(self, ctx: CallContext, brief: bytes, previous_model: bytes, history: list[str], max_rounds: int,
               on_progress: Callable[[Progress], None]) -> BlueprintOutcome:
        catalog, limits = self._catalog.operations(ctx), self._catalog.limits(ctx)
        system = "\n".join([review.instructions(), operations.ops_help(catalog, "brief_op", limits.fixture_kinds, FORBIDDEN_OPS),
                            architect.architect_rules(limits)])
        cur = self._engine.generate(ctx, brief, previous_model)
        base_area = cur.total_target_area
        rounds = min(max_rounds, MAX_ROUNDS) if max_rounds > 0 else MAX_ROUNDS
        notes: list[str] = []
        done: list[str] = []
        memory: list[str] = []
        calls = 0
        for rnd in range(1, rounds + 1):
            issues = cur.issues if not cur.model else review.issue_lines(cur.checks)
            if not issues:
                break
            on_progress(Progress("step", "review", f"Review round {rnd}/{rounds}: {len(issues)} issue(s) to fix"))
            user = review.request(history, cur.brief, issues, memory)
            try:
                ops, built, tries = self._asker.ask(ctx, system, user, catalog.brief_ops_schema,
                                                    self._applier(ctx, cur, previous_model, base_area), on_progress)
            except InvalidEdit as e:
                calls += ATTEMPTS
                notes.append(f"review round {rnd}: no usable fix ({e})")
                break
            calls += tries
            kept = self._judge(rnd, review.ops_line(ops), cur, built, len(issues), on_progress, notes, done, memory)
            if not kept:
                break
            cur = built
        left = len(cur.issues) if not cur.model else len([c for c in cur.checks if not c.ok and c.severity == "error"])
        if left:
            notes.append(f"{left} error(s) remain for you to decide")
        return BlueprintOutcome(brief=cur.brief, model=cur.model, checks=cur.checks, issues=cur.issues, ok=cur.ok,
                                notes=notes, llm_attempts=calls, history=done)

    def _applier(self, ctx: CallContext, cur: BriefBuild, previous_model: bytes, base_area: float):
        def apply(ops: dict) -> BriefBuild:
            bad = sorted({o.get("op") for o in ops.get("ops", [])} & FORBIDDEN_OPS)
            if bad:
                raise InvalidEdit(f"the reviewer may not use {', '.join(bad)}")
            built = self._engine.apply_brief_ops(ctx, cur.brief, json.dumps(ops).encode(), cur.model or previous_model)
            if not built.changed:
                raise InvalidEdit("these operations change nothing; follow the hints (e.g. side null frees a room)")
            if abs(built.total_target_area - base_area) > AREA_CAP * base_area:
                raise InvalidEdit(f"total area would change from {base_area:.0f} to {built.total_target_area:.0f} m²; "
                                  "stay within 15%")
            return built

        return apply

    def _judge(self, rnd: int, did: str, cur: BriefBuild, built: BriefBuild, issue_count: int,
               on_progress: Callable[[Progress], None], notes: list[str], done: list[str], memory: list[str]) -> bool:
        """Keeps the round when the score drops and no new kind of error appears; records what happened."""
        before, after = cur.issue_score, built.issue_score
        new_errors = sorted(set(built.error_rules) - set(cur.error_rules))
        kept = after < before and not new_errors
        on_progress(Progress("step", "review", f"Round {rnd}: {'kept' if kept else 'discarded'} ({did}); score {before} -> {after}"))
        done.append(f"round {rnd}: {did} (score {before} -> {after}, {'kept' if kept else 'discarded'})")
        if kept:
            remaining = len(built.issues) if not built.model else len(review.issue_lines(built.checks))
            notes.append(f"review round {rnd}: {did} (issues {issue_count} -> {remaining})")
            memory.append(f"round {rnd}: {did} -> accepted")
        else:
            why = f"new errors {new_errors}" if new_errors else "no improvement"
            notes.append(f"review round {rnd}: discarded {did} ({why})")
            memory.append(f"round {rnd}: {did} -> discarded, {why}")
        return kept
