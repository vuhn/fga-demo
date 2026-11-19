"""What the Engine publishes: one tool per operation the LLM may produce, and the catalog of limits."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class Tool:
    name: str
    kind: str  # brief_op | plan_op | draft
    description: str
    json_schema: str  # JSON text
    signature: str


@dataclass(frozen=True)
class ToolList:
    tools: list[Tool]
    ops_json_schema: str
    plan_ops_json_schema: str
    draft_json_schema: str


@dataclass(frozen=True)
class RateSpec:
    code: str
    package: str
    description: str
    unit: str


@dataclass(frozen=True)
class Catalog:
    min_area: dict[str, float]
    min_width: dict[str, float]
    nestable_types: list[str]
    max_nested: int
    default_fixtures: dict[str, str]
    fixture_kinds: list[str]
    default_markups: dict[str, float]
    rate_catalog: list[RateSpec]
    priced_fixtures: list[str]


@dataclass(frozen=True)
class Check:
    rule: str
    severity: str
    ok: bool
    message: str
    room_id: str = ""
    category: str = ""
    issue_id: str = ""
    elements: list[str] = field(default_factory=list)
    hint: str = ""


@dataclass(frozen=True)
class DraftNormalized:
    brief: bytes
    notes: list[str]


@dataclass(frozen=True)
class BriefResult:
    brief: bytes
    model: bytes  # empty when the brief cannot be built
    checks: list[Check]
    issues: list[str]
    ok: bool
    notes: list[str]
    attempts: int
    issue_score: int
    error_rules: list[str]
    checks_line: str
    total_target_area: float
    room_targets: dict[str, float]
    changed: bool


@dataclass(frozen=True)
class PlanStep:
    model: bytes
    checks: list[Check]
    ok: bool
    notes: list[str]
    summary: str
    descriptions: list[str]
    changed: bool


@dataclass(frozen=True)
class ChecksResult:
    checks: list[Check]
    ok: bool
    issue_score: int
