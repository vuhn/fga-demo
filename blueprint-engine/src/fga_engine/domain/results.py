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


class SummaryRoom(BaseModel):
    id: str
    name: str
    type: str
    area_m2: float


class SummaryFixture(BaseModel):
    id: str
    kind: str
    room_id: str


class SummaryOpening(BaseModel):
    id: str
    kind: str  # door | window | open (an entrance is a door)
    room_ids: list[str]
    width: float


class PlanSummary(BaseModel):
    rooms: list[SummaryRoom]
    fixtures: list[SummaryFixture]
    openings: list[SummaryOpening]
    pending_fixtures: list[str]  # priced fixtures whose installed state is still unknown
    structure_in_scope: bool


class BriefResult(BaseModel):
    """A built brief with what an LLM retry or review loop needs: the brief as applied, the checks, a score,
    the failed rules and the target areas."""

    brief: Brief
    model: BuildingModel | None
    checks: list[CheckResult]
    issues: list[str]
    ok: bool
    notes: list[str] = []
    attempts: int = 1  # layout runs
    issue_score: int
    error_rules: list[str]
    checks_line: str
    total_target_area: float
    room_targets: dict[str, float]  # room id -> target area
    changed: bool = False  # the ops changed the brief


class CheckReport(BaseModel):
    """The checks of a plan, judged: whether it is ok and how many issues weigh on it (10 per failed error, 1 per warning)."""

    checks: list[CheckResult]
    ok: bool
    issue_score: int


class PlanEdit(BaseModel):
    """A person's edit of an imported plan: the checked step, what the plan is now, what each op did, and whether it changed."""

    step: PlanStep
    summary: str
    descriptions: list[str]
    changed: bool


def issue_score(res: GenerateResult) -> int:
    """Lower is better: an unbuildable brief is 1000+, then 10 per failed error check and 1 per warning."""
    from .checks import failed

    if res.model is None:
        return 1000 + 10 * len(res.issues)
    return 10 * len(failed(res.checks)) + len(failed(res.checks, "warning"))


def error_rules(res: GenerateResult) -> list[str]:
    from .checks import failed

    return ["brief"] if res.model is None else sorted({c.rule for c in failed(res.checks)})


def checks_line(res: GenerateResult) -> str:
    from .checks import failed

    if res.model is None:
        return "The brief could not be built: " + "; ".join(res.issues)[:200]
    errors, warnings = len(failed(res.checks)), len(failed(res.checks, "warning"))
    return f"Checks: {errors} error(s), {warnings} warning(s)" if errors or warnings else "Checks: all pass"


def brief_result(res: GenerateResult, *, notes: list[str] | None = None, attempts: int = 1,
                 changed: bool = False) -> BriefResult:
    return BriefResult(brief=res.brief, model=res.model, checks=res.checks, issues=res.issues, ok=res.ok,
                       notes=list(notes if notes is not None else res.notes), attempts=attempts,
                       issue_score=issue_score(res), error_rules=error_rules(res), checks_line=checks_line(res),
                       total_target_area=sum(r.target_area for r in res.brief.rooms),
                       room_targets={r.id: r.target_area for r in res.brief.rooms}, changed=changed)
