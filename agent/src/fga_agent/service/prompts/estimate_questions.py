"""The questions the estimate interview asks, one per slot: what the person is shown, with the answers offered as chips."""

from __future__ import annotations

from ...domain.model.estimate_session import Question, SessionState
from ...domain.model.plan_summary import PlanSummary

UNKNOWN_OPTION = "I don't know, you decide"
SLOT_NAMES = ("city", "kind", "scope", "work_type", "fixtures", "quality", "structure_in_scope", "storeys", "floor_height",
              "mep_in_scope", "markups")
MARKUP_LABELS = {"overhead_percent": "site overhead and temporary works (typically 5-8%)", "profit_percent": "contractor profit (5-10%)",
                 "design_percent": "design, supervision and permits (3-8%)", "contingency_percent": "contingency (5-10%)",
                 "vat_percent": "VAT (the rate in force)"}


def city() -> Question:
    return Question("city", "Where is the project located (city and district)? Prices differ by region.",
                    ["Ho Chi Minh City", "Hanoi", "Da Nang"])


def kind() -> Question:
    return Question("kind", "Is this an apartment or a house?", ["Apartment", "House"])


def scope(summary: PlanSummary, state: SessionState) -> Question:
    unset = [r for r in summary.rooms if r.id not in state.scope]
    names = ", ".join(r.name for r in unset[:12])
    options = ["Everything is empty"]
    if any(r.type == "bathroom" for r in unset):
        options.append("Everything is empty except the bathrooms")
    options.append("Everything already exists")
    return Question("scope", f"Is the space empty, or do some rooms or items already exist? I see: {names}. "
                    "Tell me which rooms are already finished (if any).", options)


def work_type() -> Question:
    return Question("work_type", "What kind of work is this: a new build, fitting out an empty shell, or renovating an existing home?",
                    ["New build", "Fit-out of an empty shell", "Renovation"])


def fixtures(summary: PlanSummary) -> Question:
    rooms = {r.id: r.name for r in summary.rooms}
    pending = set(summary.pending_fixtures)
    items = [f"{f.kind.replace('_', ' ')} ({rooms.get(f.room_id, f.room_id)})" for f in summary.fixtures if f.id in pending]
    return Question("fixtures", "The plan shows these fixtures: " + ", ".join(items[:14]) + (" …" if len(items) > 14 else "")
                    + ". A floor plan does not say what is installed, so I assume none of them exist yet and will price supplying "
                    "and fitting each. Which of them already exist?", ["None of them exist", "All of them already exist"])


def quality() -> Question:
    return Question("quality", "What quality level do you expect: typical (economy), normal, good or luxury?",
                    ["Typical", "Normal", "Good", "Luxury"])


def structure_in_scope() -> Question:
    return Question("structure_in_scope", "Does the work include structure (foundation, columns, slabs), or finishes only?",
                    ["Finishes only", "Include structure"])


def storeys() -> Question:
    return Question("storeys", "How many storeys does the structure have?", ["1", "2", "3"])


def floor_height() -> Question:
    return Question("floor_height", "What is the floor-to-ceiling height? The plan does not show it and it drives wall and finish areas.",
                    ["2.7 m", "3.0 m", "3.3 m"])


def mep_in_scope() -> Question:
    return Question("mep_in_scope", "Should electrical and plumbing work be included?",
                    ["Yes, include electrical and plumbing", "No, exclude them"])


def markups(missing: list[str]) -> Question:
    labels = "; ".join(MARKUP_LABELS.get(n, n) for n in missing)
    return Question("markups", "Which percentages should I add on top of the direct cost: " + labels
                    + "? I can use the typical mid-range values if you agree.", ["Use typical rates"])
