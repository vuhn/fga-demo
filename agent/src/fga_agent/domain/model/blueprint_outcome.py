"""What a chat revise or a review hands back: the plan as built, its checks and what was done to get there."""

from __future__ import annotations

from dataclasses import dataclass

from .plan_check import PlanCheck


@dataclass(frozen=True)
class BlueprintOutcome:
    brief: bytes  # empty for a plan chat
    model: bytes  # empty when the brief cannot be built
    checks: list[PlanCheck]
    issues: list[str]  # brief-level problems (no model when non-empty)
    ok: bool
    notes: list[str]
    llm_attempts: int  # model calls, retries with the error included
    history: list[str]  # what the applied operations were, or what each review round did
