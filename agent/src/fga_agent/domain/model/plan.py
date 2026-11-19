"""A plan (the building model) after explicit edit operations."""

from __future__ import annotations

from dataclasses import dataclass

from .plan_check import PlanCheck


@dataclass(frozen=True)
class PlanRevision:
    model: bytes
    checks: list[PlanCheck]
    ok: bool
    notes: list[str]
    summary: str  # what the plan holds, for prompts
    descriptions: list[str]  # one line per applied operation
    changed: bool
