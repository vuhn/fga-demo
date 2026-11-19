"""The architect's knowledge pack (Text2BIM's "Architect" role), rendered from the Engine's design limits so it cannot
drift from what the layout and the checks apply. Pure functions: limits in, text out."""

from __future__ import annotations

from ...domain.model.design_limits import DesignLimits


def room_types(limits: DesignLimits) -> list[str]:
    return [t for t in limits.min_area if t != "corridor"]


def conventions(limits: DesignLimits) -> str:
    return f"""Conventions: metres and m². North is up. The lot is width (east-west) x depth (north-south).
entrance_side is the side the street is on: "street on the north", "facing north", "entered from the north" and "north entry" all mean entrance_side north. Single storey only. Never list a hall or corridor; the layout adds it and it always stays.
Open plan ("open-plan living", "open kitchen", "remove the hall partitions/walls", "combine living, kitchen and dining
into one space") means open_plan true: the code removes the walls between the hall, entry, living, kitchen and dining
and keeps the rooms; never remove the hall or merge those rooms for it.
Room types: {", ".join(room_types(limits))}.
Typical net areas when not given: entry 4-6, living 20-30, kitchen 9-12, dining 10-14, study 8-10,
laundry 3-5, storage 2-4, master bedroom 14-16, bedroom 10-12, bathroom 4-6, ensuite 4-5, walk-in closet 3-4, pantry 2-3.
Defaults when not given: lot 20 x 30, entrance south. Use 'side' only when the user asks for a room on a
particular wall. Use 'adjacent_to' only for pairs the user asks for or that clearly belong together.
Only list rooms the user asked for; add one bathroom if there are bedrooms but none was mentioned.
Names are short human labels, unique, e.g. "Master bedroom", "Bedroom 2", "Ensuite".
A room that is entered through another room (an ensuite in the master bedroom, a walk-in closet, a pantry
in the kitchen) is listed as its own room with 'inside' set to that room's name; only {", ".join(sorted(limits.nestable_types))}
rooms can be inside another room.
Fixtures ({", ".join(limits.fixture_kinds)}): typical ones are added automatically; list in 'fixtures' only
extra ones the user asks for (e.g. a bathtub, a desk in a bedroom)."""


def architect_rules(limits: DesignLimits) -> str:
    types = room_types(limits)
    fixtures = "; ".join(f"{t}: {limits.default_fixtures.get(t) or 'none'}".replace(",", ", ") for t in types)
    return (
        "Architectural rules the layout code applies (plan with them, do not fight them):\n"
        "- The house is zoned: public rooms (entry, living, kitchen, dining, study) along the street side, "
        "private rooms (bedrooms, bathrooms) at the back, joined by a hall the code adds.\n"
        "- Kitchen next to dining, dining next to living, entry next to living. Keep wet rooms (bathrooms, "
        "laundry, kitchen) close to each other and every bedroom near a bathroom.\n"
        "- Bedrooms, living, kitchen, dining and study need a window, so they need an outside wall.\n"
        "- Minimum net areas (m²): " + ", ".join(f"{t} {limits.min_area[t]:g}" for t in types) + ".\n"
        "- Minimum widths (m): " + ", ".join(f"{t} {limits.min_width[t]:g}" for t in types if t in limits.min_width) + ".\n"
        "- Default fixtures: " + fixtures + ".\n"
        "- Rooms plus about 10% for walls plus a 1.2 m hall must fit the lot after setbacks "
        "(3 m front, 3 m back, 1.5 m each side by default)."
    )
