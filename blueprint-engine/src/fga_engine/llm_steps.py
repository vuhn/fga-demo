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

import io
import json
import re
import base64
from collections.abc import Callable
from typing import TypeVar

from pydantic import BaseModel, Field, ValidationError

from . import progress, prompts
from .domain import plan_edits
from .domain.checks import failed, run_imported_checks
from .draw import TitleBlock, build_dxf, to_dxf_bytes
from .dxf_plan import import_dxf
from .domain.edits import EditError, UnsupportedEdit, apply_ops, resolve_room
from .llm import LLMClient
from .pipeline import GenerateResult, generate, generate_fitted
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

_HALL = re.compile(r"\b(hall|hallway|corridor|passage)\b", re.I)


class DraftRoom(BaseModel):
    name: str
    type: RoomType
    target_area: float = Field(gt=0, le=200)
    side: Side | None = None
    adjacent_to: list[str] = Field(default_factory=list, description="names of other rooms")
    inside: str | None = Field(default=None, description="name of the room this one is inside (ensuite, walk-in closet)")
    fixtures: list[FixtureKind] = Field(default_factory=list, description="extra fixtures the user asked for")


class DraftBrief(BaseModel):
    title: str
    lot_width: float = Field(gt=4, le=200)
    lot_depth: float = Field(gt=4, le=200)
    entrance_side: Side
    rooms: list[DraftRoom] = Field(min_length=1, max_length=16)
    open_plan: bool = Field(default=False, description="true only when the user asks for open-plan living "
                                                       "(no walls between hall, living, kitchen and dining)")


class StepResult(BaseModel):
    result: GenerateResult
    notes: list[str] = []
    ops: EditOps | None = None
    attempts: int = 1  # layout runs (generate_fitted)
    llm_attempts: int = 0  # LLM calls, including retries with error feedback
    review_rounds: list[dict] = []  # reviewer trace: round, ops, issues before/after, accepted


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


def draft_to_brief(draft: DraftBrief) -> tuple[Brief, list[str]]:
    """Deterministic clean-up of model output: ids, references, minimum areas, corridors."""
    notes: list[str] = []
    rooms: list[Room] = []
    pairs: list[tuple[DraftRoom, Room]] = []
    used: set[str] = set()
    for d in draft.rooms:
        if _HALL.search(d.name):
            notes.append(f"dropped '{d.name}': the layout adds the hall itself")
            continue
        base = re.sub(r"[^a-z0-9]+", "_", d.name.lower()).strip("_")[:30] or d.type
        rid, n = base, 2
        while rid in used:
            rid, n = f"{base}_{n}", n + 1
        used.add(rid)
        area = d.target_area
        if area < MIN_AREA[d.type]:
            notes.append(f"raised {d.name} from {area:g} to the {d.type} minimum of {MIN_AREA[d.type]:g} m²")
            area = MIN_AREA[d.type]
        name = d.name.strip()[:56] or d.type.title()
        if "_" in name or name.islower():
            name = name.replace("_", " ").capitalize()
        taken = {r.name.lower() for r in rooms}
        label, k = name, 2
        while label.lower() in taken:
            label, k = f"{name} {k}", k + 1
        fixtures = list(DEFAULT_FIXTURES[d.type])
        fixtures += [f for f in d.fixtures if f not in fixtures]
        room = Room(id=rid, name=label, type=d.type, target_area=area, side=d.side, fixtures=fixtures[:8])
        rooms.append(room)
        pairs.append((d, room))
    if not rooms:
        raise EditError("the description has no rooms besides halls")
    brief = Brief(
        title=draft.title[:80] or "Single-storey house",
        lot=Lot(width=draft.lot_width, depth=draft.lot_depth),
        entrance_side=draft.entrance_side,
        rooms=rooms,
        open_plan=draft.open_plan,
    )
    for d, room in pairs:
        for ref in d.adjacent_to:
            try:
                other = resolve_room(brief, ref)
            except EditError:
                notes.append(f"ignored adjacency {room.name} - {ref!r} (no such room)")
                continue
            if other.id != room.id and other.id not in room.adjacent_to:
                room.adjacent_to.append(other.id)
    for d, room in pairs:
        if not d.inside:
            continue
        try:
            host = resolve_room(brief, d.inside)
        except EditError:
            notes.append(f"{room.name} is not inside {d.inside!r} (no such room)")
            continue
        if room.type not in NESTABLE_TYPES or host.id == room.id or host.parent is not None \
                or len(brief.children(host.id)) >= MAX_NESTED:
            notes.append(f"{room.name} opens onto the hall; it cannot be inside {host.name}")
            continue
        room.parent, room.side = host.id, None
    for room in brief.rooms:  # a parent that ended up nested itself: un-nest its children
        if room.parent and brief.room(room.parent).parent:
            room.parent = None
    return Brief.model_validate(brief.model_dump()), notes


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
    notes = list(notes or [])
    result, fit_notes, runs = generate_fitted(brief, previous)
    return StepResult(result=result, notes=notes + fit_notes, attempts=runs)


def _issue_lines(checks) -> list[str]:
    out = []
    for c in failed(checks) + failed(checks, "warning"):
        out.append(f"[{c.severity}] {c.issue_id}: {c.message}" + (f" (hint: {c.hint})" if c.hint else ""))
    return out


def _score(res: GenerateResult) -> int:
    if res.model is None:
        return 1000 + 10 * len(res.issues)
    return 10 * len(failed(res.checks)) + len(failed(res.checks, "warning"))


def _checks_line(res: GenerateResult) -> str:
    if res.model is None:
        return "The brief could not be built: " + "; ".join(res.issues)[:200]
    errors, warnings = len(failed(res.checks)), len(failed(res.checks, "warning"))
    return f"Checks: {errors} error(s), {warnings} warning(s)" if errors or warnings else "Checks: all pass"


def _error_rules(res: GenerateResult) -> set[str]:
    return {"brief"} if res.model is None else {c.rule for c in failed(res.checks)}


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


class PlanResult(BaseModel):
    model: BuildingModel
    checks: list[CheckResult]
    ok: bool
    issues: list[str] = []
    notes: list[str] = []


class PlanStep(BaseModel):
    """Result of a chat edit on an imported plan (no brief). Shaped like StepResult (result, notes) so the
    API reads both the same way; `result.brief` is absent."""

    result: PlanResult
    notes: list[str] = []
    ops: plan_edits.PlanOps
    llm_attempts: int = 1


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


class PlanBuild(BaseModel):
    """What creating a plan gave: the drawn model (None when the brief cannot be built) and why."""

    model: BuildingModel | None
    checks: list[CheckResult] = []
    ok: bool = False
    issues: list[str] = []
    notes: list[str] = []


class PlanCreated(BaseModel):
    result: PlanBuild
    notes: list[str] = []
    dxf: str | None = None  # base64 of the CAD file the plan was drawn into: its original from now on


def draw_plan(model: BuildingModel, title: str) -> PlanCreated:
    """Draw a model into our own DXF and read it back as a plan with provenance: the file becomes the plan's
    original. Used for a plan made from a brief and for converting a plan made before DXF plans existed."""
    data = to_dxf_bytes(build_dxf(model, TitleBlock(project=title[:80] or "Floor plan"))[0])
    plans = import_dxf(data).plans
    if len(plans) != 1:  # our own drawing must read back as one plan; anything else is a bug, not a bad input
        raise EditError("the drawn plan could not be read back as one plan")
    plan = plans[0].model
    before, after = sum(r.net_area for r in model.rooms), sum(r.net_area for r in plan.rooms)
    if abs(after - before) > 0.03 * before or len(plan.fixtures) != len(model.fixtures):
        raise EditError(f"the drawing reads back as {after:.0f} m² and {len(plan.fixtures)} fixtures, but the plan has "
                        f"{before:.0f} m² and {len(model.fixtures)}; it cannot be converted without losing parts of it")
    notes = []
    if len(plan.rooms) != len(model.rooms):  # an open plan has no walls between its rooms: a CAD file shows one space
        notes.append(f"{len(model.rooms)} rooms became {len(plan.rooms)} spaces: rooms with no wall between them "
                     "(an open plan) are one space in the drawing")
    checks = run_imported_checks(plan)
    return PlanCreated(result=PlanBuild(model=plan, checks=checks, ok=not failed(checks), notes=notes), notes=[],
                       dxf=base64.b64encode(data).decode("ascii"))


def create_plan(llm: LLMClient, brief: Brief, notes: list[str] | None = None) -> PlanCreated:
    """Create a plan the way an import does. The brief is only the input: the layout is drawn into our own
    DXF, and that file is read back like any CAD file, so the plan has no brief afterwards, keeps the file
    as its original, and is edited, chatted about and exported like an imported one."""
    step = create(llm, brief, notes)
    r = step.result
    if r.model is None:
        return PlanCreated(result=PlanBuild(model=None, issues=r.issues, notes=r.notes), notes=step.notes)
    out = draw_plan(r.model, brief.title)
    out.result.notes = r.notes + out.result.notes
    out.notes = step.notes
    return out


def edit_plan(model: BuildingModel, ops: plan_edits.PlanOps) -> PlanStep:
    """Direct edit of an imported plan: the ops come from a person (the viewer), not from the LLM."""
    new_model = plan_edits.apply_plan_ops(model, ops)
    checks = run_imported_checks(new_model)
    desc = "; ".join(plan_edits.describe(o) for o in ops.ops)
    return PlanStep(result=PlanResult(model=new_model, checks=checks, ok=not failed(checks)),
                    notes=["edited: " + desc], ops=ops, llm_attempts=0)


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
