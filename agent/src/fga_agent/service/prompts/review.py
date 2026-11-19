"""The reviewer's instructions and what it is shown: the failed checks and what earlier rounds did."""

from __future__ import annotations

from ...domain.model.plan_check import PlanCheck
from . import conversation

MAX_ISSUES_SHOWN = 20


def instructions() -> str:
    return ("You review a house plan. Fix the listed issues with as few edit operations as possible, following each "
            "hint where it helps; when a hint gives an op with its JSON fields, send exactly that op (\"side\": null is the "
            "JSON null, not a side name). Never remove or rename rooms, change the lot or the entrance; keep the user's "
            "requests.")


def issue_lines(checks: list[PlanCheck]) -> list[str]:
    """Failed errors first, then failed warnings: `[error] side:kitchen: message (hint: set_side ...)`."""
    failed = [c for c in checks if not c.ok]
    ordered = [c for c in failed if c.severity == "error"] + [c for c in failed if c.severity == "warning"]
    return [f"[{c.severity}] {c.issue_id}: {c.message}" + (f" (hint: {c.hint})" if c.hint else "") for c in ordered]


def request(history: list[str], brief: bytes, issues: list[str], memory: list[str]) -> str:
    text = (f"{conversation.history_block(history)}Current brief: {conversation.brief_for_prompt(brief)}\nIssues:\n"
            + "\n".join(f"- {i}" for i in issues[:MAX_ISSUES_SHOWN]))
    return text + ("\nEarlier review rounds:\n" + "\n".join(memory) if memory else "")


def ops_line(ops: dict) -> str:
    return "; ".join(conversation.describe_op(o) for o in ops.get("ops", []))
