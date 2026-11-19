"""Op-documentation helper shared by the edit-op prompts and plan_edits (pure, no LLM)."""

from __future__ import annotations

import typing


def field_sig(name: str, field) -> str:
    """One field of an op as shown in the op docs: `name`, `name?` (optional) or `name|null` (nullable)."""
    if not field.is_required():
        return f"{name}?"
    nullable = type(None) in typing.get_args(field.annotation)
    return f"{name}|null" if nullable else name
