"""Architect knowledge pack: the prompts every LLM step shares (Text2BIM's "Architect" role).

The edit-op documentation is rendered from the EditOps schema (field names and the op
docstrings), so the prompts cannot drift from what apply_ops accepts.
"""

from __future__ import annotations

import json
import typing

from .schemas import (
    DEFAULT_FIXTURES,
    FIXTURE_KINDS,
    MIN_AREA,
    MIN_WIDTH,
    NESTABLE_TYPES,
    EditOps,
)

_ROOM_TYPES = [t for t in MIN_AREA if t != "corridor"]

CONVENTIONS = f"""Conventions: metres and m². North is up. The lot is width (east-west) x depth (north-south).
entrance_side is the side the street is on: "street on the north", "facing north", "entered from the north" and "north entry" all mean entrance_side north. Single storey only. Never list a hall or corridor; the layout adds it and it always stays.
Open plan ("open-plan living", "open kitchen", "remove the hall partitions/walls", "combine living, kitchen and dining
into one space") means open_plan true: the code removes the walls between the hall, entry, living, kitchen and dining
and keeps the rooms; never remove the hall or merge those rooms for it.
Room types: {", ".join(_ROOM_TYPES)}.
Typical net areas when not given: entry 4-6, living 20-30, kitchen 9-12, dining 10-14, study 8-10,
laundry 3-5, storage 2-4, master bedroom 14-16, bedroom 10-12, bathroom 4-6, ensuite 4-5, walk-in closet 3-4, pantry 2-3.
Defaults when not given: lot 20 x 30, entrance south. Use 'side' only when the user asks for a room on a
particular wall. Use 'adjacent_to' only for pairs the user asks for or that clearly belong together.
Only list rooms the user asked for; add one bathroom if there are bedrooms but none was mentioned.
Names are short human labels, unique, e.g. "Master bedroom", "Bedroom 2", "Ensuite".
A room that is entered through another room (an ensuite in the master bedroom, a walk-in closet, a pantry
in the kitchen) is listed as its own room with 'inside' set to that room's name; only {", ".join(sorted(NESTABLE_TYPES))}
rooms can be inside another room.
Fixtures ({", ".join(FIXTURE_KINDS)}): typical ones are added automatically; list in 'fixtures' only
extra ones the user asks for (e.g. a bathtub, a desk in a bedroom)."""

ARCHITECT_RULES = (
    "Architectural rules the layout code applies (plan with them, do not fight them):\n"
    "- The house is zoned: public rooms (entry, living, kitchen, dining, study) along the street side, "
    "private rooms (bedrooms, bathrooms) at the back, joined by a hall the code adds.\n"
    "- Kitchen next to dining, dining next to living, entry next to living. Keep wet rooms (bathrooms, "
    "laundry, kitchen) close to each other and every bedroom near a bathroom.\n"
    "- Bedrooms, living, kitchen, dining and study need a window, so they need an outside wall.\n"
    "- Minimum net areas (m²): " + ", ".join(f"{t} {MIN_AREA[t]:g}" for t in _ROOM_TYPES) + ".\n"
    "- Minimum widths (m): " + ", ".join(f"{t} {MIN_WIDTH[t]:g}" for t in _ROOM_TYPES) + ".\n"
    "- Default fixtures: " + "; ".join(f"{t}: {', '.join(f) or 'none'}" for t, f in DEFAULT_FIXTURES.items()
                                        if t != "corridor") + ".\n"
    "- Rooms plus about 10% for walls plus a 1.2 m hall must fit the lot after setbacks "
    "(3 m front, 3 m back, 1.5 m each side by default)."
)

ENHANCE_EXAMPLES = """Example (format only: take every room count, size and side from the description, never from here)
Description: "small 2 bed house with ensuite, street to the east"
reasoning: two bedrooms; one is the master because it has the ensuite; a second bathroom is needed for bedroom 2; no lot size given so use the default; street east means entrance east; kitchen, living and dining are implied by a house.
requirements:
- Living room about 22 m² next to the dining room
- Kitchen about 10 m² next to the dining room
- Dining about 10 m²
- Master bedroom about 14 m² with an ensuite (about 4.5 m²) inside it: toilet, basin, shower
- Bedroom 2 about 11 m²
- Bathroom about 5 m² near bedroom 2
- Lot 20 x 30 m, entrance on the east
assumptions:
- lot size not given; used 20 x 30 m
- added a main bathroom for bedroom 2"""

DRAFT_EXAMPLE = """Example (format only; use the given requirements, not these rooms)
Requirements: "Master bedroom about 14 m² with an ensuite (about 4.5 m²) inside it; Bedroom 2 about 11 m²; Kitchen 10 m² next to Dining 10 m²; Living 22 m²; Bathroom 5 m²; lot 20 x 30, entrance east"
JSON: {"title": "Two-bedroom house", "lot_width": 20, "lot_depth": 30, "entrance_side": "east", "rooms": [
 {"name": "Living", "type": "living", "target_area": 22, "adjacent_to": ["Dining"]},
 {"name": "Kitchen", "type": "kitchen", "target_area": 10, "adjacent_to": ["Dining"]},
 {"name": "Dining", "type": "dining", "target_area": 10},
 {"name": "Master bedroom", "type": "bedroom", "target_area": 14},
 {"name": "Ensuite", "type": "bathroom", "target_area": 4.5, "inside": "Master bedroom"},
 {"name": "Bedroom 2", "type": "bedroom", "target_area": 11},
 {"name": "Bathroom", "type": "bathroom", "target_area": 5}]}"""


def _field_sig(name: str, field) -> str:
    if not field.is_required():
        return f"{name}?"
    nullable = type(None) in typing.get_args(field.annotation)
    return f"{name}|null" if nullable else name


def ops_help(exclude: frozenset[str] = frozenset()) -> str:
    """Op documentation rendered from the EditOps union: name {fields}: docstring."""
    union = typing.get_args(EditOps.model_fields["ops"].annotation)[0]
    members = typing.get_args(typing.get_args(union)[0])
    lines = ["Operations (refer to rooms by their current name):"]
    for cls in members:
        op = typing.get_args(cls.model_fields["op"].annotation)[0]
        if op in exclude:
            continue
        fields = ", ".join(_field_sig(n, f) for n, f in cls.model_fields.items() if n != "op")
        lines.append(f"- {op} {{{fields}}}: {(cls.__doc__ or '').strip()}")
    lines.append(f"Fixtures: {', '.join(FIXTURE_KINDS)}.")
    return "\n".join(lines)


def history_block(history: list[str] | None) -> str:
    if not history:
        return ""
    items = [h.strip()[:300] for h in history[-6:] if h and h.strip()]
    return ("Earlier requests on this plan, oldest first. They are already applied; use them only to tell what "
            "'it', 'that room', 'a bit more' or 'bigger please' refer to (usually the latest request's room):\n"
            + "\n".join(f"- {h}" for h in items) + "\n") if items else ""


def feedback(previous_output: str, error: str) -> str:
    """Text2BIM Eq. 4: the failed output and its error go back to the model."""
    return (f"\nYour previous answer was:\n{previous_output[:1500]}\nApplying it failed: {error[:600]}\n"
            "Return a corrected answer that avoids this error.")


def dump(obj) -> str:
    return json.dumps(obj.model_dump(exclude_none=True) if hasattr(obj, "model_dump") else obj)
