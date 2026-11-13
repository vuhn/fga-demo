"""Hand-written briefs (no LLM) used by tests and the dev harness."""

from blueprint_engine.schemas import Brief

FAMILY_3BR = Brief.model_validate({
    "title": "Family house, 3 bedrooms",
    "lot": {"width": 20, "depth": 30},
    "entrance_side": "south",
    "rooms": [
        {"id": "entry", "name": "Entry", "type": "entry", "target_area": 5},
        {"id": "living", "name": "Living", "type": "living", "target_area": 24},
        {"id": "kitchen", "name": "Kitchen", "type": "kitchen", "target_area": 11, "adjacent_to": ["dining"]},
        {"id": "dining", "name": "Dining", "type": "dining", "target_area": 12},
        {"id": "bed1", "name": "Master bedroom", "type": "bedroom", "target_area": 15},
        {"id": "bed2", "name": "Bedroom 2", "type": "bedroom", "target_area": 11},
        {"id": "bed3", "name": "Bedroom 3", "type": "bedroom", "target_area": 10},
        {"id": "bath1", "name": "Bathroom", "type": "bathroom", "target_area": 6},
        {"id": "bath2", "name": "Ensuite", "type": "bathroom", "target_area": 4.5, "adjacent_to": ["bed1"]},
    ],
})

STUDIO = Brief.model_validate({
    "title": "Garden studio",
    "lot": {"width": 12, "depth": 15},
    "entrance_side": "east",
    "rooms": [
        {"id": "living", "name": "Studio", "type": "living", "target_area": 20},
        {"id": "kitchen", "name": "Kitchenette", "type": "kitchen", "target_area": 7},
        {"id": "bath", "name": "Shower room", "type": "bathroom", "target_area": 3.5},
    ],
})

NORTH_ENTRY_2BR = Brief.model_validate({
    "title": "Two-bedroom, north entry",
    "lot": {"width": 16, "depth": 24},
    "entrance_side": "north",
    "rooms": [
        {"id": "living", "name": "Living / dining", "type": "living", "target_area": 28},
        {"id": "kitchen", "name": "Kitchen", "type": "kitchen", "target_area": 9, "side": "west"},
        {"id": "laundry", "name": "Laundry", "type": "laundry", "target_area": 3},
        {"id": "bed1", "name": "Bedroom 1", "type": "bedroom", "target_area": 13},
        {"id": "bed2", "name": "Bedroom 2", "type": "bedroom", "target_area": 10},
        {"id": "bath", "name": "Bathroom", "type": "bathroom", "target_area": 5.5},
    ],
})

WEST_ENTRY_STUDY = Brief.model_validate({
    "title": "Courtyard house with study",
    "lot": {"width": 28, "depth": 18, "setbacks": {"front": 4, "back": 3, "sides": 1.2}},
    "entrance_side": "west",
    "rooms": [
        {"id": "entry", "name": "Entry", "type": "entry", "target_area": 4},
        {"id": "living", "name": "Living", "type": "living", "target_area": 22},
        {"id": "kitchen", "name": "Kitchen", "type": "kitchen", "target_area": 10, "adjacent_to": ["dining"]},
        {"id": "dining", "name": "Dining", "type": "dining", "target_area": 10},
        {"id": "study", "name": "Study", "type": "study", "target_area": 8},
        {"id": "bed1", "name": "Bedroom 1", "type": "bedroom", "target_area": 14},
        {"id": "bed2", "name": "Bedroom 2", "type": "bedroom", "target_area": 10},
        {"id": "bath", "name": "Bathroom", "type": "bathroom", "target_area": 6},
    ],
})

TOO_BIG = Brief.model_validate({
    "title": "Does not fit",
    "lot": {"width": 10, "depth": 12},
    "rooms": [
        {"id": "living", "name": "Living", "type": "living", "target_area": 60},
        {"id": "bed1", "name": "Bedroom", "type": "bedroom", "target_area": 30},
        {"id": "bed2", "name": "Bedroom 2", "type": "bedroom", "target_area": 30},
        {"id": "bed3", "name": "Bedroom 3", "type": "bedroom", "target_area": 30},
    ],
})

SHRINKABLE = Brief.model_validate({
    "title": "Too generous for the lot",
    "lot": {"width": 14, "depth": 18},
    "rooms": [
        {"id": "living", "name": "Living", "type": "living", "target_area": 55},
        {"id": "kitchen", "name": "Kitchen", "type": "kitchen", "target_area": 22},
        {"id": "bed1", "name": "Bedroom", "type": "bedroom", "target_area": 30},
        {"id": "bath", "name": "Bathroom", "type": "bathroom", "target_area": 10},
    ],
})

FEASIBLE = {
    "family_3br": FAMILY_3BR,
    "studio": STUDIO,
    "north_entry_2br": NORTH_ENTRY_2BR,
    "west_entry_study": WEST_ENTRY_STUDY,
}
