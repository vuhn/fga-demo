"""Deterministic pipeline: brief -> layout -> building model -> checks.

LLM steps (text/sketch -> brief, chat revisions) live in llm_steps.py and only
ever produce or edit a Brief. Fitting an oversized brief is done here, in code.
"""

from __future__ import annotations

from pydantic import BaseModel

from .building import build_model
from .checks import area_budget, brief_issues, failed, run_checks
from .layout import LayoutError, compute_layout
from .schemas import MIN_AREA, Brief, BuildingModel, CheckResult


class GenerateResult(BaseModel):
    brief: Brief
    model: BuildingModel | None
    checks: list[CheckResult]
    issues: list[str]  # brief-level problems (no model when non-empty)
    ok: bool
    notes: list[str] = []


def generate(brief: Brief, previous: BuildingModel | None = None) -> GenerateResult:
    issues = brief_issues(brief)
    if issues:
        return GenerateResult(brief=brief, model=None, checks=[], issues=issues, ok=False)
    try:
        lay = compute_layout(brief, previous)
    except LayoutError as e:
        return GenerateResult(brief=brief, model=None, checks=[], issues=[str(e)], ok=False)
    model = build_model(brief, lay)
    checks = run_checks(brief, model)
    return GenerateResult(
        brief=brief, model=model, checks=checks, issues=[], ok=not failed(checks), notes=lay.notes
    )


def shrink_to(brief: Brief, budget: float) -> Brief | None:
    """Scale every room's area above its minimum so the total fits the budget; None if the
    minimums alone do not fit."""
    total = sum(r.target_area for r in brief.rooms)
    if total <= budget:
        return brief
    excess = {r.id: max(0.0, r.target_area - MIN_AREA[r.type]) for r in brief.rooms}
    need = total - budget
    spare = sum(excess.values())
    if spare < need:
        return None
    f = 1 - need / spare
    out = brief.model_copy(deep=True)
    for r in out.rooms:
        r.target_area = max(MIN_AREA[r.type], int((MIN_AREA[r.type] + excess[r.id] * f) * 2) / 2)
    return out


def generate_fitted(brief: Brief, previous: BuildingModel | None = None) -> tuple[GenerateResult, list[str], int]:
    """generate(), and when the brief is too big for the lot, shrink rooms toward their minimums
    (never below, never changing the lot or the room list). Returns (result, notes, runs)."""
    result = generate(brief, previous)
    runs = 1
    if result.model is not None:
        return result, [], runs
    budget = area_budget(brief)
    minimum = sum(MIN_AREA[r.type] for r in brief.rooms)
    if minimum > budget:
        result.issues.append(
            f"even at minimum sizes these rooms need {minimum:.0f} m² but the lot fits about {budget:.0f} m²; "
            "remove rooms or enlarge the lot"
        )
        return result, [], runs
    for factor in (0.97, 0.92, 0.86):
        fitted = shrink_to(brief, budget * factor)
        if fitted is None:
            break
        runs += 1
        attempt = generate(fitted, previous)
        if attempt.model is not None:
            changed = [f"{a.name} {a.target_area:g}→{b.target_area:g}"
                       for a, b in zip(brief.rooms, fitted.rooms) if a.target_area != b.target_area]
            return attempt, ["shrank rooms to fit the lot: " + ", ".join(changed)], runs
    return result, [], runs
