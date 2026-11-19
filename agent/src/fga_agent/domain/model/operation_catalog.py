"""The edit operations the Engine accepts, as it publishes them. The Agent builds prompts and structured-output
schemas from these and never duplicates them."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class EditOperation:
    name: str
    kind: str  # brief_op | plan_op | draft
    description: str
    schema: dict  # JSON Schema of the operation
    signature: str  # `name {field, field?}` as shown in prompts


@dataclass(frozen=True)
class OperationCatalog:
    operations: list[EditOperation]
    brief_ops_schema: dict  # the whole brief-ops document
    plan_ops_schema: dict
    draft_schema: dict  # a brief as drafted from text
