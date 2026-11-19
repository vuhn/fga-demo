"""The only places an LLM is used. Each one produces or edits a Brief; none touches geometry.

The agent roles follow Text2BIM (arXiv 2408.08054), with JSON edit ops in place of generated code:
1. Enhancer: description -> explicit requirements and assumptions (enhance).
2. Drafter: description + requirements -> DraftBrief -> Brief (ids, refs and minimums fixed in code).
3. Reviser: chat message (+ earlier requests) -> EditOps -> apply_ops.
4. Reviewer: failed checks with hints -> EditOps, kept only when the issue score improves (review).
Every step that applies model output feeds an apply error back to the model, up to 3 attempts.
Repair of an infeasible brief stays deterministic first (pipeline.generate_fitted).
"""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from typing import TypeVar

from pydantic import BaseModel, Field, ValidationError

from . import progress, prompts
from .domain import plan_edits
from .domain.checks import failed, run_imported_checks
from .domain.edits import EditError, UnsupportedEdit, apply_ops, resolve_room
from .llm import LLMClient
from .domain.draft import DraftBrief, DraftRoom, normalize_draft as draft_to_brief  # noqa: F401
from .domain import results
from .domain.results import GenerateResult, PlanResult, PlanStep, StepResult
from .pipeline import generate, generate_fitted
from .service.generator import fitted_step
from .domain.schemas import (
    CheckResult,
    DEFAULT_FIXTURES,
    MAX_NESTED,
    MIN_AREA,
    NESTABLE_TYPES,
    Brief,
    BuildingModel,
    EditOps,
    FixtureKind,
    Lot,
    Room,
    RoomType,
    Side,
)

class Requirements(BaseModel):
    """Enhancer output (Text2BIM's Instruction Enhancer, with the Architect's rules in its prompt)."""

    reasoning: str = Field(description="first count the bedrooms, bathrooms and other rooms the description names, then think step by step about what else the house needs; kept short")
    requirements: list[str] = Field(min_length=1, max_length=30,
                                    description="one line per room or constraint, with areas, nesting and fixtures")
    assumptions: list[str] = Field(default_factory=list, max_length=5,
                                   description="decisions about THIS house the description did not state "
                                               "(lot size, entrance, rooms or sizes you added); never restate rules")


T = TypeVar("T", bound=BaseModel)
R = TypeVar("R")
MAX_ATTEMPTS = 3
REVIEW_FORBIDDEN = frozenset({"remove_room", "set_lot", "set_entrance", "unsupported", "rename_room",
                              "set_open_plan"})
REVIEW_AREA_CAP = 0.15


def _with_feedback(llm: LLMClient, system: str, user: str, schema: type[T],
                   apply: Callable[[T], R], attempts: int = MAX_ATTEMPTS) -> tuple[T, R, int]:
    """Call the model, apply its output; on an apply error, send the output and the error back
    (Text2BIM Eq. 4) and try again. Unsupported requests are not retried."""
    prompt = user
    last: Exception | None = None
    for n in range(1, attempts + 1):
        progress.emit("ask_model", f"Asking the model… attempt {n}/{attempts}")
        out = llm.structured(system, prompt, schema)
        try:
            return out, apply(out), n
        except UnsupportedEdit:
            raise
        except (EditError, ValidationError) as e:
            last = e
            if n < attempts:
                progress.emit("retry", f"Attempt {n} could not be applied ({str(e)[:160]}); sending the error back")
            prompt = user + prompts.feedback(prompts.dump(out), str(e))
    raise EditError(f"{last} (after {attempts} attempts)")


def enhance(llm: LLMClient, prompt: str) -> Requirements:
    """Text2BIM's Instruction Enhancer: make the implicit explicit before drafting."""
    system = ("You are an architect. Turn a short house description into explicit requirements: every room "
              "with an approximate area, rooms inside other rooms, requested fixtures, the lot and the "
              "entrance. List as assumptions only decisions about this house that the user did not state "
              "(at most 5); do not restate general rules, and do not mention rooms you did not list.\n"
              + prompts.ARCHITECT_RULES + "\n" + prompts.CONVENTIONS + "\n" + prompts.ENHANCE_EXAMPLES)
    return llm.structured(system, f"Description: {prompt}\nLet's think step by step.", Requirements)


def brief_from_text(llm: LLMClient, prompt: str, *, use_enhancer: bool = False) -> tuple[Brief, list[str]]:
    notes: list[str] = []
    user = f"Description: {prompt}"
    if use_enhancer:
        req = enhance(llm, prompt)
        user += "\nRequirements from the architect:\n" + "\n".join(f"- {r}" for r in req.requirements)
        notes += [f"assumed: {a}" for a in req.assumptions[:5]]
    system = ("You turn a house description and its requirements into a JSON brief.\n"
              + prompts.CONVENTIONS + "\n" + prompts.DRAFT_EXAMPLE)
    # Small models over-assign sides; keep one only if the user named that direction.
    said = {d for d in ("north", "south", "east", "west") if re.search(rf"\b{d}", prompt, re.I)}

    def to_brief(draft: DraftBrief) -> tuple[Brief, list[str]]:
        for room in draft.rooms:
            if room.side is not None and room.side not in said:
                room.side = None
        return draft_to_brief(draft)

    _, (brief, draft_notes), _ = _with_feedback(llm, system, user, DraftBrief, to_brief)
    return brief, notes + draft_notes


def _brief_for_prompt(brief: Brief) -> str:
    rooms = [
        {"name": r.name, "type": r.type, "area": r.target_area, "side": r.side,
         "adjacent_to": [brief.room(a).name for a in r.adjacent_to],
         "inside": brief.room(r.parent).name if r.parent else None, "fixtures": r.fixtures}
        for r in brief.rooms
    ]
    return json.dumps({"lot": [brief.lot.width, brief.lot.depth], "entrance_side": brief.entrance_side,
                       "open_plan": brief.open_plan, "rooms": rooms})


def _describe(op) -> str:
    d = op.model_dump(exclude_none=True)
    kind = d.pop("op")
    return kind.replace("_", " ") + " " + ", ".join(f"{k}={v}" for k, v in d.items())


def create(llm: LLMClient, brief: Brief, notes: list[str] | None = None,
           previous: BuildingModel | None = None) -> StepResult:
    return fitted_step(brief, previous, notes)


def _issue_lines(checks) -> list[str]:
    out = []
    for c in failed(checks) + failed(checks, "warning"):
        out.append(f"[{c.severity}] {c.issue_id}: {c.message}" + (f" (hint: {c.hint})" if c.hint else ""))
    return out


def _score(res: GenerateResult) -> int:
    return results.issue_score(res)


def _checks_line(res: GenerateResult) -> str:
    return results.checks_line(res)


def _error_rules(res: GenerateResult) -> set[str]:
    return set(results.error_rules(res))


def revise(llm: LLMClient, brief: Brief, previous: BuildingModel | None, message: str,
           history: list[str] | None = None, *, auto_review: bool = True) -> StepResult:
    system = ("You turn a revision request for a house plan into edit operations.\n"
              + prompts.ops_help() + "\n" + prompts.ARCHITECT_RULES)
    user = f"{prompts.history_block(history)}Current brief: {_brief_for_prompt(brief)}\nRequest: {message}"
    ops, new_brief, tries = _with_feedback(llm, system, user, EditOps, lambda o: apply_ops(brief, o))
    notes = ["applied: " + "; ".join(_describe(o) for o in ops.ops)]
    progress.emit("ops", "The model proposed: " + "; ".join(_describe(o) for o in ops.ops))
    if tries > 1:
        notes.append(f"the first {tries - 1} answer(s) could not be applied; corrected with the error")
    progress.emit("layout", "Rebuilding the layout and running the checks…")
    result, fit_notes, runs = generate_fitted(new_brief, previous)
    progress.emit("checks", _checks_line(result))
    step = StepResult(result=result, notes=notes + fit_notes, ops=ops, attempts=runs, llm_attempts=tries)
    if auto_review and (result.model is None or failed(result.checks)):
        # one reviewer round, only when the edit introduced errors the plan did not have
        before = generate(brief, previous)
        if _error_rules(result) - _error_rules(before) or result.model is None:
            progress.emit("review", "The edit introduced new errors; running one review round")
            rev = review(llm, result.brief, previous, history=[*(history or []), message], max_rounds=1,
                         start=result)
            step.result, step.review_rounds = rev.result, rev.review_rounds
            step.notes += rev.notes
            step.llm_attempts += rev.llm_attempts
    return step


def revise_plan(llm: LLMClient, model: BuildingModel, message: str, history: list[str] | None = None) -> PlanStep:
    """Chat edit of an imported plan: the LLM names operations, code applies them to the model as drawn."""
    system = ("You turn a revision request for an existing floor plan, imported from a CAD file, into edit "
              "operations. The plan is kept as drawn: walls and rooms cannot be moved or resized, so answer "
              "'unsupported' for that.\n" + plan_edits.ops_help())
    user = f"{prompts.history_block(history)}Current plan: {plan_edits.summary(model)}\nRequest: {message}"
    ops, new_model, tries = _with_feedback(llm, system, user, plan_edits.PlanOps,
                                           lambda o: plan_edits.apply_plan_ops(model, o))
    desc = "; ".join(plan_edits.describe(o) for o in ops.ops)
    progress.emit("ops", "The model proposed: " + desc)
    progress.emit("checks", "Running the checks…")
    checks = run_imported_checks(new_model)
    notes = ["applied: " + desc]
    if tries > 1:
        notes.append(f"the first {tries - 1} answer(s) could not be applied; corrected with the error")
    return PlanStep(result=PlanResult(model=new_model, checks=checks, ok=not failed(checks)), notes=notes, ops=ops,
                    llm_attempts=tries)


def review(llm: LLMClient, brief: Brief, previous: BuildingModel | None, history: list[str] | None = None,
           *, max_rounds: int = 3, start: GenerateResult | None = None) -> StepResult:
    """Text2BIM's Reviewer loop over our checker. Each round proposes edit ops for the failed
    checks; a round is kept only when the issue score drops and no new kind of error appears.
    The user's intent is fixed: no removing rooms, renaming, changing the lot or entrance, and
    the total area moves at most 15%. Unresolved issues are left for the person."""
    cur = start or generate(brief, previous)
    base_area = sum(r.target_area for r in cur.brief.rooms)
    system = ("You review a house plan. Fix the listed issues with as few edit operations as possible, "
              "following each hint where it helps; when a hint gives an op with its JSON fields, send exactly that op "
              "(\"side\": null is the JSON null, not a side name). Never remove or rename rooms, change the lot or the "
              "entrance; keep the user's requests.\n"
              + prompts.ops_help(exclude=REVIEW_FORBIDDEN) + "\n" + prompts.ARCHITECT_RULES)
    notes: list[str] = []
    rounds: list[dict] = []
    memory: list[str] = []
    calls = 0
    for rnd in range(1, max_rounds + 1):
        issues = cur.issues if cur.model is None else _issue_lines(cur.checks)
        if not issues:
            break
        user = (f"{prompts.history_block(history)}Current brief: {_brief_for_prompt(cur.brief)}\n"
                f"Issues:\n" + "\n".join(f"- {i}" for i in issues[:20])
                + ("\nEarlier review rounds:\n" + "\n".join(memory) if memory else ""))

        def apply(o: EditOps, b=cur.brief) -> Brief:
            bad = sorted({op.op for op in o.ops} & REVIEW_FORBIDDEN)
            if bad:
                raise EditError(f"the reviewer may not use {', '.join(bad)}")
            nb = apply_ops(b, o)
            if nb == b:
                raise EditError("these operations change nothing; follow the hints (e.g. side null frees a room)")
            total = sum(r.target_area for r in nb.rooms)
            if abs(total - base_area) > REVIEW_AREA_CAP * base_area:
                raise EditError(f"total area would change from {base_area:.0f} to {total:.0f} m²; stay within 15%")
            return nb

        progress.emit("review", f"Review round {rnd}/{max_rounds}: {len(issues)} issue(s) to fix")
        try:
            ops, nb, tries = _with_feedback(llm, system, user, EditOps, apply)
        except EditError as e:
            calls += MAX_ATTEMPTS
            notes.append(f"review round {rnd}: no usable fix ({e})")
            break
        calls += tries
        res = generate(nb, cur.model or previous)
        before, after = _score(cur), _score(res)
        new_errors = _error_rules(res) - _error_rules(cur)
        accepted = after < before and not new_errors
        did = "; ".join(_describe(o) for o in ops.ops)
        rounds.append({"round": rnd, "ops": did, "score_before": before, "score_after": after,
                       "accepted": accepted})
        progress.emit("review", f"Round {rnd}: {'kept' if accepted else 'discarded'} ({did}); score {before} -> {after}")
        if accepted:
            fixed = len(issues) - (len(res.issues) if res.model is None else len(_issue_lines(res.checks)))
            notes.append(f"review round {rnd}: {did} (issues {len(issues)} -> {len(issues) - fixed})")
            memory.append(f"round {rnd}: {did} -> accepted")
            cur = res
        else:
            why = f"new errors {sorted(new_errors)}" if new_errors else "no improvement"
            notes.append(f"review round {rnd}: discarded {did} ({why})")
            memory.append(f"round {rnd}: {did} -> discarded, {why}")
            break
    left = len(cur.issues) if cur.model is None else len(failed(cur.checks))
    if left:
        notes.append(f"{left} error(s) remain for you to decide")
    return StepResult(result=cur, notes=notes, attempts=1, llm_attempts=calls, review_rounds=rounds)
