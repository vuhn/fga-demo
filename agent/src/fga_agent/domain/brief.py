"""A brief (what the person asked for) as drafted, normalized and built. Briefs and models cross as opaque JSON
bytes in the Engine's schema; the Agent never interprets geometry."""

from __future__ import annotations

from dataclasses import dataclass

from .plan_check import PlanCheck


@dataclass(frozen=True)
class BriefDraft:
    """A drafted brief after the Engine normalized it."""

    brief: bytes
    notes: list[str]


@dataclass(frozen=True)
class BriefBuild:
    """The result of building a brief into a plan, with everything a retry or review loop needs."""

    brief: bytes  # normalized, as applied
    model: bytes  # empty when the brief cannot be built
    checks: list[PlanCheck]
    issues: list[str]  # brief-level problems (no model when non-empty)
    ok: bool
    notes: list[str]
    attempts: int
    issue_score: int
    error_rules: list[str]
    checks_line: str
    total_target_area: float
    room_targets: dict[str, float]
    changed: bool  # the applied operations changed the brief
