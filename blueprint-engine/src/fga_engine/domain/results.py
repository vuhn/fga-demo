"""Result shapes the brief and plan operations return (the JSON the HTTP handlers send)."""

from __future__ import annotations

from pydantic import BaseModel

from . import plan_edits
from .schemas import Brief, BuildingModel, CheckResult, EditOps


class GenerateResult(BaseModel):
    brief: Brief
    model: BuildingModel | None
    checks: list[CheckResult]
    issues: list[str]  # brief-level problems (no model when non-empty)
    ok: bool
    notes: list[str] = []


class StepResult(BaseModel):
    result: GenerateResult
    notes: list[str] = []
    ops: EditOps | None = None
    attempts: int = 1  # layout runs (generate_fitted)
    llm_attempts: int = 0  # LLM calls, including retries with error feedback
    review_rounds: list[dict] = []  # reviewer trace: round, ops, issues before/after, accepted


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
