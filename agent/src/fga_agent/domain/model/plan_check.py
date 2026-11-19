"""One rule result of the plan checks, and the report of a run."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class PlanCheck:
    rule: str
    severity: str  # error | warning
    ok: bool
    message: str
    room_id: str | None = None
    category: str = ""
    issue_id: str = ""
    elements: list[str] = field(default_factory=list)
    hint: str = ""  # a suggested edit operation


@dataclass(frozen=True)
class PlanCheckReport:
    checks: list[PlanCheck]
    ok: bool
    issue_score: int
