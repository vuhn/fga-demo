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
from collections.abc import Callable
from typing import TypeVar

from pydantic import BaseModel, Field, ValidationError

from . import prompts
from .checks import failed
from .edits import EditError, UnsupportedEdit, apply_ops, resolve_room
from .llm import LLMClient
from .pipeline import GenerateResult, generate, generate_fitted
from .schemas import (
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

MAX_UPLOAD = 10 * 1024 * 1024
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
        out = llm.structured(system, prompt, schema)
        try:
            return out, apply(out), n
        except UnsupportedEdit:
            raise
        except (EditError, ValidationError) as e:
            last = e
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


def _to_png(data: bytes, mime: str) -> bytes:
    from PIL import Image

    if mime == "application/pdf":
        import pypdfium2 as pdfium

        pdf = pdfium.PdfDocument(data)
        if len(pdf) == 0:
            raise EditError("the PDF has no pages")
        img = pdf[0].render(scale=2).to_pil()
    elif mime in ("image/png", "image/jpeg"):
        img = Image.open(io.BytesIO(data))
    else:
        raise EditError("upload a PNG, JPG, PDF or DXF")
    img = img.convert("RGB")
    img.thumbnail((1600, 1600))
    out = io.BytesIO()
    img.save(out, format="PNG")
    return out.getvalue()


def _dxf_summary(data: bytes) -> str:
    """Text labels and outline sizes of a DXF, in metres, for the text model (no vision needed)."""
    import ezdxf
    from ezdxf import bbox

    try:
        doc = ezdxf.read(io.StringIO(data.decode("utf-8", errors="replace")))
    except Exception as e:  # ezdxf raises several error types on malformed files
        raise EditError(f"cannot read the DXF: {e}") from e
    msp = doc.modelspace()
    labels: list[str] = []
    for e in msp:
        if e.dxftype() == "TEXT":
            t = e.dxf.text
        elif e.dxftype() == "MTEXT":
            t = e.plain_text()
        else:
            continue
        t = " ".join(t.split())
        if t and t not in labels:
            labels.append(t)
    ext = bbox.extents(msp)
    if not ext.has_data:
        raise EditError("the DXF has no drawing content")
    # $INSUNITS 4 = mm, 5 = cm; metres or unset is trusted only when the drawing is house-sized
    # (many mm files never set it, and no house is 200 m wide).
    big = max(ext.size.x, ext.size.y) > 200
    scale = {4: 0.001, 5: 0.01}.get(doc.header.get("$INSUNITS", 0), 0.001 if big else 1.0)
    if scale == 1.0 and big:
        scale = 0.001
    areas = []
    for e in msp.query("LWPOLYLINE"):
        if e.closed and len(e) >= 3:
            pts = [(x * scale, y * scale) for x, y, *_ in e.get_points()]
            a = abs(sum(x1 * y2 - x2 * y1 for (x1, y1), (x2, y2) in zip(pts, pts[1:] + pts[:1]))) / 2
            if a >= 2:
                areas.append(round(float(a), 1))
    return (
        f"Overall extents: {float(ext.size.x) * scale:.1f} x {float(ext.size.y) * scale:.1f} m.\n"
        f"Text labels in the drawing: {'; '.join(labels[:80]) or 'none'}.\n"
        f"Closed outline areas (m2): {sorted(areas, reverse=True)[:40] or 'none'}."
    )


def _brief_from_dxf(llm: LLMClient, data: bytes, note: str) -> tuple[Brief, list[str]]:
    system = (
        "You read the text content of a CAD floor plan (DXF) and extract the room program as a JSON brief. "
        "Use the room labels and any areas written in them; do not invent rooms that are not labelled. "
        "Ignore dimensions, titles, scales and sheet numbers. Estimate missing areas with typical values. "
        "Set 'side' only if the user says so.\n" + prompts.CONVENTIONS
    )
    user = "Extract the brief from this DXF summary.\n" + _dxf_summary(data) + (f"\nNotes from the user: {note}" if note else "")
    draft = llm.structured(system, user, DraftBrief)
    for room in draft.rooms:
        room.side = None
    brief, notes = draft_to_brief(draft)
    return brief, ["read from the DXF's text labels (geometry is not copied); review the rooms before generating", *notes]


def brief_from_sketch(llm: LLMClient, data: bytes, mime: str, note: str = "") -> tuple[Brief, list[str]]:
    if len(data) > MAX_UPLOAD:
        raise EditError("upload is larger than 10 MB")
    if mime == "image/vnd.dxf":
        return _brief_from_dxf(llm, data, note)
    png = _to_png(data, mime)
    system = (
        "You read a hand sketch or an existing floor plan and extract the room program as a JSON brief. "
        "Do not copy geometry; estimate each room's area from any dimensions shown, otherwise use typical areas. "
        "Set 'side' only for rooms clearly on one outside wall that matters (e.g. kitchen at the back). "
        "If a north arrow is drawn use it, otherwise assume north is up.\n" + prompts.CONVENTIONS
    )
    user = "Extract the brief from this drawing." + (f" Notes from the user: {note}" if note else "")
    draft = llm.structured(system, user, DraftBrief, images=[png], vision=True)
    brief, notes = draft_to_brief(draft)
    return brief, ["read from the uploaded drawing; review the rooms before generating", *notes]


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


def _error_rules(res: GenerateResult) -> set[str]:
    return {"brief"} if res.model is None else {c.rule for c in failed(res.checks)}


def revise(llm: LLMClient, brief: Brief, previous: BuildingModel | None, message: str,
           history: list[str] | None = None, *, auto_review: bool = True) -> StepResult:
    system = ("You turn a revision request for a house plan into edit operations.\n"
              + prompts.ops_help() + "\n" + prompts.ARCHITECT_RULES)
    user = f"{prompts.history_block(history)}Current brief: {_brief_for_prompt(brief)}\nRequest: {message}"
    ops, new_brief, tries = _with_feedback(llm, system, user, EditOps, lambda o: apply_ops(brief, o))
    notes = ["applied: " + "; ".join(_describe(o) for o in ops.ops)]
    if tries > 1:
        notes.append(f"the first {tries - 1} answer(s) could not be applied; corrected with the error")
    result, fit_notes, runs = generate_fitted(new_brief, previous)
    step = StepResult(result=result, notes=notes + fit_notes, ops=ops, attempts=runs, llm_attempts=tries)
    if auto_review and (result.model is None or failed(result.checks)):
        # one reviewer round, only when the edit introduced errors the plan did not have
        before = generate(brief, previous)
        if _error_rules(result) - _error_rules(before) or result.model is None:
            rev = review(llm, result.brief, previous, history=[*(history or []), message], max_rounds=1,
                         start=result)
            step.result, step.review_rounds = rev.result, rev.review_rounds
            step.notes += rev.notes
            step.llm_attempts += rev.llm_attempts
    return step


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
