"""The edit operations as the Engine publishes them, rendered for a prompt, so the text cannot drift from what the
Engine accepts."""

from __future__ import annotations

from ...domain.model.operation_catalog import OperationCatalog


def ops_help(catalog: OperationCatalog, kind: str, fixture_kinds: list[str], exclude: frozenset[str] = frozenset()) -> str:
    """`- name {fields}: what it does` for each operation of `kind` (brief_op or plan_op) except those excluded."""
    lines = ["Operations (refer to rooms by their current name):"]
    lines += [f"- {op.signature}: {op.description}" for op in catalog.operations if op.kind == kind and op.name not in exclude]
    lines.append(f"Fixtures: {', '.join(fixture_kinds)}.")
    return "\n".join(lines)
