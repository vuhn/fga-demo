"""Worked examples shown to the model (format only: every count, size and side comes from the person's words)."""

from __future__ import annotations


def enhance_examples() -> str:
    return """Example (format only: take every room count, size and side from the description, never from here)
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


def draft_example() -> str:
    return """Example (format only; use the given requirements, not these rooms)
Requirements: "Master bedroom about 14 m² with an ensuite (about 4.5 m²) inside it; Bedroom 2 about 11 m²; Kitchen 10 m² next to Dining 10 m²; Living 22 m²; Bathroom 5 m²; lot 20 x 30, entrance east"
JSON: {"title": "Two-bedroom house", "lot_width": 20, "lot_depth": 30, "entrance_side": "east", "rooms": [
 {"name": "Living", "type": "living", "target_area": 22, "adjacent_to": ["Dining"]},
 {"name": "Kitchen", "type": "kitchen", "target_area": 10, "adjacent_to": ["Dining"]},
 {"name": "Dining", "type": "dining", "target_area": 10},
 {"name": "Master bedroom", "type": "bedroom", "target_area": 14},
 {"name": "Ensuite", "type": "bathroom", "target_area": 4.5, "inside": "Master bedroom"},
 {"name": "Bedroom 2", "type": "bedroom", "target_area": 11},
 {"name": "Bathroom", "type": "bathroom", "target_area": 5}]}"""


def requirements_schema() -> dict:
    """The enhancer's answer (Text2BIM's Instruction Enhancer, with the architect's rules in its prompt)."""
    return {"type": "object", "required": ["reasoning", "requirements"], "properties": {
        "reasoning": {"type": "string", "description": "first count the bedrooms, bathrooms and other rooms the "
                      "description names, then think step by step about what else the house needs; kept short"},
        "requirements": {"type": "array", "minItems": 1, "maxItems": 30, "items": {"type": "string"},
                         "description": "one line per room or constraint, with areas, nesting and fixtures"},
        "assumptions": {"type": "array", "maxItems": 5, "items": {"type": "string"},
                        "description": "decisions about THIS house the description did not state (lot size, entrance, "
                                       "rooms or sizes you added); never restate rules"}}}
