"""What the LLM may ask the Engine to do: one tool per edit operation, plus the draft, and the catalog of limits."""

from __future__ import annotations

from dataclasses import dataclass

from . import plan_edits
from .draft import DraftBrief
from .op_docs import field_sig, op_classes, op_name
from .rate_catalog import DEFAULT_MARKUPS, PRICED_FIXTURES, RATE_CATALOG
from .schemas import (DEFAULT_FIXTURES, FIXTURE_KINDS, MAX_NESTED, MIN_AREA, MIN_WIDTH, NESTABLE_TYPES, EditOps)


@dataclass(frozen=True)
class Tool:
    name: str
    kind: str  # brief_op | plan_op | draft
    description: str
    json_schema: dict
    signature: str  # `name {field, field?, field|null}` as shown in prompts


@dataclass(frozen=True)
class ToolList:
    tools: list[Tool]
    ops_json_schema: dict
    plan_ops_json_schema: dict
    draft_json_schema: dict


def _signature(name: str, cls) -> str:
    fields = ", ".join(field_sig(n, f) for n, f in cls.model_fields.items() if n != "op")
    return f"{name} {{{fields}}}"


def _ops(ops_model, kind: str) -> list[Tool]:
    out = []
    for cls in op_classes(ops_model):
        name = op_name(cls)
        out.append(Tool(name=name, kind=kind, description=(cls.__doc__ or "").strip(),
                        json_schema=cls.model_json_schema(), signature=_signature(name, cls)))
    return out


def list_tools() -> ToolList:
    draft = DraftBrief.model_json_schema()
    tools = _ops(EditOps, "brief_op") + _ops(plan_edits.PlanOps, "plan_op")
    tools.append(Tool(name="draft_brief", kind="draft", description=(DraftBrief.__doc__ or "A brief as drafted from text.").strip(),
                      json_schema=draft, signature="draft_brief {" + ", ".join(
                          field_sig(n, f) for n, f in DraftBrief.model_fields.items()) + "}"))
    return ToolList(tools=tools, ops_json_schema=EditOps.model_json_schema(),
                    plan_ops_json_schema=plan_edits.PlanOps.model_json_schema(), draft_json_schema=draft)


@dataclass(frozen=True)
class RateInfo:
    code: str
    package: str
    description: str
    unit: str
    quality_sensitive: bool = False


@dataclass(frozen=True)
class Catalog:
    min_area: dict[str, float]
    min_width: dict[str, float]
    nestable_types: list[str]
    max_nested: int
    default_fixtures: dict[str, str]
    fixture_kinds: list[str]
    default_markups: dict[str, float]
    rate_catalog: list[RateInfo]
    priced_fixtures: list[str]


def get_catalog() -> Catalog:
    return Catalog(
        min_area=dict(MIN_AREA), min_width=dict(MIN_WIDTH), nestable_types=sorted(NESTABLE_TYPES),
        max_nested=MAX_NESTED, default_fixtures={t: ",".join(f) for t, f in DEFAULT_FIXTURES.items()},
        fixture_kinds=list(FIXTURE_KINDS), default_markups={k: float(v) for k, v in DEFAULT_MARKUPS.items()},
        rate_catalog=[RateInfo(c, s.package, s.description, s.unit, s.quality_sensitive) for c, s in RATE_CATALOG.items()],
        priced_fixtures=sorted(PRICED_FIXTURES))
