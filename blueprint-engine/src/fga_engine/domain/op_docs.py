"""Op-documentation helper shared by the edit-op prompts and plan_edits (pure, no LLM)."""

from __future__ import annotations

import typing


def field_sig(name: str, field) -> str:
    """One field of an op as shown in the op docs: `name`, `name?` (optional) or `name|null` (nullable)."""
    if not field.is_required():
        return f"{name}?"
    nullable = type(None) in typing.get_args(field.annotation)
    return f"{name}|null" if nullable else name


def op_classes(ops_model) -> list[type]:
    """The member classes of an ops document (`EditOps`, `PlanOps`): one per operation, in declaration order."""
    union = typing.get_args(ops_model.model_fields["ops"].annotation)[0]
    return list(typing.get_args(typing.get_args(union)[0]))


def op_name(cls) -> str:
    return typing.get_args(cls.model_fields["op"].annotation)[0]
