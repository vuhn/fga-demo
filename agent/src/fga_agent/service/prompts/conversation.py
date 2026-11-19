"""The parts of a prompt that carry the conversation: earlier requests, the current brief, a failed answer's feedback."""

from __future__ import annotations

import json


def history_block(history: list[str] | None) -> str:
    items = [h.strip()[:300] for h in (history or [])[-6:] if h and h.strip()]
    if not items:
        return ""
    return ("Earlier requests on this plan, oldest first. They are already applied; use them only to tell what "
            "'it', 'that room', 'a bit more' or 'bigger please' refer to (usually the latest request's room):\n"
            + "\n".join(f"- {h}" for h in items) + "\n")


def feedback(previous_answer: dict, error: str) -> str:
    """Text2BIM Eq. 4: the failed answer and its error go back to the model."""
    return (f"\nYour previous answer was:\n{json.dumps(previous_answer)[:1500]}\nApplying it failed: {error[:600]}\n"
            "Return a corrected answer that avoids this error.")


def brief_for_prompt(brief: bytes) -> str:
    """The brief as the model reads it: rooms by name (adjacency and nesting by the other room's name, not its id)."""
    b = json.loads(brief)
    name = {r["id"]: r["name"] for r in b["rooms"]}
    rooms = [{"name": r["name"], "type": r["type"], "area": r["target_area"], "side": r.get("side"),
              "adjacent_to": [name[a] for a in r.get("adjacent_to", [])],
              "inside": name[r["parent"]] if r.get("parent") else None, "fixtures": r.get("fixtures", [])}
             for r in b["rooms"]]
    return json.dumps({"lot": [b["lot"]["width"], b["lot"]["depth"]], "entrance_side": b.get("entrance_side"),
                       "open_plan": b.get("open_plan", False), "rooms": rooms})


def describe_op(op: dict) -> str:
    """One applied operation as a line: `set area room=kitchen, area=12`."""
    rest = {k: v for k, v in op.items() if k != "op" and v is not None}
    return str(op.get("op", "")).replace("_", " ") + " " + ", ".join(f"{k}={v}" for k, v in rest.items())
