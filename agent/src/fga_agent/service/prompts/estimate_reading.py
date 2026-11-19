"""What the estimate reader asks the model and shows it: four narrow readings of one message (the facts, the settings, the
condition of rooms, what already exists), each answered as short claims {quote, field, value}."""

from __future__ import annotations

from ...domain.model.estimate_session import SessionState
from ...domain.model.plan_summary import PlanSummary


def claims_schema(fields: list[str], limit: int) -> dict:
    """The answer's JSON schema. `quote` comes first so the value is decided from the words; `field` may only name this
    reading's fields."""
    claim = {"type": "object", "additionalProperties": False, "required": ["quote", "field", "value"], "properties": {
        "quote": {"type": "string", "maxLength": 160}, "field": {"type": "string", "enum": fields},
        "value": {"type": "string", "maxLength": 80}}}
    return {"type": "object", "additionalProperties": False, "required": ["claims"],
            "properties": {"claims": {"type": "array", "maxItems": limit, "items": claim}}}


def _common() -> str:
    return (
        "Read ONE message from a person who is getting a construction cost estimate for a floor plan. The message may be in any language (English, Vietnamese, Japanese, ...): understand it in that language, but write each value in the fixed English form below.\n"
        "Return `claims`, one for each thing the message states: {quote, field, value}. quote = the exact words of the message that say it, copied word for word in the message's own language. If the message states nothing about these fields, return no claims. Never guess and never add a claim the words do not support.\n"
        ""
    )


def core_system() -> str:
    return _common() + (
        "Fields and values:\n"
        "- kind: apartment | house. work_type: new_build | fit_out (finishing an empty shell) | renovation.\n"
        "- city: only the city, in Latin letters ('Ho Chi Minh City', 'Da Nang'). district: the district or ward ('District 7', 'Thu Duc'), never in city.\n"
        "- quality: typical | normal | good | luxury. floor_height: metres. storeys: a whole number of floors.\n"
        "Examples (message -> claims):\n"
        "'My flat in Hue needs a standard finish' -> [{quote:'flat', field:kind, value:apartment}, {quote:'Hue', field:city, value:Hue}, {quote:'standard finish', field:quality, value:normal}]\n"
        "'Nhà phố ở Thủ Đức, TP HCM, 3 tầng' -> [{quote:'Nhà phố', field:kind, value:house}, {quote:'TP HCM', field:city, value:Ho Chi Minh City}, {quote:'Thủ Đức', field:district, value:Thu Duc}, {quote:'3 tầng', field:storeys, value:3}]\n"
        "'ハノイの一戸建て、高品質で' -> [{quote:'一戸建て', field:kind, value:house}, {quote:'ハノイ', field:city, value:Hanoi}, {quote:'高品質', field:quality, value:good}]\n"
        "'Ceilings are 2.9 m high' -> [{quote:'2.9 m', field:floor_height, value:2.9}]\n"
        "'The rooms already exist' / 'Hello' -> []"
    )


def settings_system() -> str:
    return _common() + (
        "Fields and values:\n"
        "- structure_in_scope: true | false (foundation, columns, slabs are part of the work). mep_in_scope: true | false (electrical and plumbing).\n"
        "- vat_percent, overhead_percent, profit_percent, design_percent, contingency_percent: a number, only when the message gives one.\n"
        "- use_typical_markups: true, ONLY when the person says to use typical or usual percentages.\n"
        "- decide_for_me: the person gives up on the question just asked ('I don't know', 'you decide', 'tùy bạn', 'お任せします'); value = the slot named in 'Question just asked'.\n"
        "- explain: the cost item the person asks about.\n"
        "Examples (message -> claims):\n"
        "'Overhead 6%, VAT 8%' -> [{quote:'Overhead 6%', field:overhead_percent, value:6}, {quote:'VAT 8%', field:vat_percent, value:8}]\n"
        "'Include the wiring and pipes' / 'Tính cả đường điện nước' / '電気と配管も含める' -> one claim: mep_in_scope true.\n"
        "'Only the finishing, structure is not needed' / 'Chỉ phần hoàn thiện thôi' / '仕上げのみで構造は不要' -> one claim: structure_in_scope false.\n"
        "'Not sure' (question just asked, slot quality) -> [{quote:'Not sure', field:decide_for_me, value:quality}]\n"
        "'Why does the ceiling cost that much?' -> [{quote:'the ceiling', field:explain, value:ceiling}]\n"
        "'Use the usual percentages' -> [{quote:'usual percentages', field:use_typical_markups, value:true}]\n"
        "'Hello' / 'normal quality' -> []"
    )


def scope_system() -> str:
    return _common() + (
        "Fields and values: the CONDITION of rooms.\n"
        "- scope_default: the condition of all rooms: empty (bare shell, nothing finished) | existing (already finished, no work) | to_replace (strip and redo) | new (to be built).\n"
        "- scope_room: one room whose condition differs from the rest, as 'room_id=state' with the room_id from the room list. A room kind ('the toilets', 'nhà vệ sinh', 'トイレ') means EVERY room of that kind: one claim per room.\n"
        "A message that does not say whether rooms are empty, finished, to be replaced or new gives NO claim: 'the rooms exist' only says they are on the plan, and 'finishes only' or 'no structure' is about the scope of work.\n"
        "Examples (the toilets are r7 and r9):\n"
        "'The place is a bare shell' / 'Nhà còn thô, chưa hoàn thiện gì' / '全室スケルトンです' -> one claim: scope_default empty.\n"
        "'Everything is bare except the toilets' -> [{quote:'Everything is bare', field:scope_default, value:empty}, {quote:'except the toilets', field:scope_room, value:r7=existing}, {quote:'except the toilets', field:scope_room, value:r9=existing}]\n"
        "'Chỉ có nhà vệ sinh là đã xong, còn lại để trống' -> [{quote:'còn lại để trống', field:scope_default, value:empty}, {quote:'nhà vệ sinh là đã xong', field:scope_room, value:r7=existing}, {quote:'nhà vệ sinh là đã xong', field:scope_room, value:r9=existing}]\n"
        "'トイレ以外は何も仕上がっていません' -> [{quote:'何も仕上がっていません', field:scope_default, value:empty}, {quote:'トイレ以外', field:scope_room, value:r7=existing}, {quote:'トイレ以外', field:scope_room, value:r9=existing}]\n"
        "'Finishes only, no structure' / 'The rooms already exist' / 'Các phòng có sẵn rồi' / 'Hello' -> []"
    )


def items_system(rate_codes: list[str]) -> str:
    return _common() + (
        "Fields and values: what is already installed, and what is out of or back in the estimate.\n"
        "- fixture_exists: the id of a fixture from the fixture list that the person says is ALREADY installed (one claim per fixture). A floor plan only shows where fixtures go, so they are assumed NOT installed. fixtures_all_exist / fixtures_none_exist: true, only when the person says all of them are installed / none of them is.\n"
        "- exclude_item: a rate code the person wants OUT of the estimate because that work already exists or is not wanted. include_item: a code they want back. One claim per code. Codes: {codes}. A door that is not called entrance, front or main means two claims: door_interior and door_entrance. An exception ('except the entrance door') or an 'only X' names what exists.\n"
        "Installed fixtures are fixture_exists with their ids, never exclude_item.\n"
        "Examples (the toilets' fixtures are f8 and f9):\n"
        "'Nothing is installed yet except the front door' -> [{quote:'Nothing is installed yet', field:fixtures_none_exist, value:true}, {quote:'except the front door', field:exclude_item, value:door_entrance}]\n"
        "'Thiết bị chưa lắp cái nào, trừ cửa ra vào' -> [{quote:'Thiết bị chưa lắp cái nào', field:fixtures_none_exist, value:true}, {quote:'trừ cửa ra vào', field:exclude_item, value:door_entrance}]\n"
        "'設備は未設置です。玄関ドアだけは取り付け済み' -> [{quote:'設備は未設置', field:fixtures_none_exist, value:true}, {quote:'玄関ドアだけは取り付け済み', field:exclude_item, value:door_entrance}]\n"
        "'The toilets already have everything' -> [{quote:'The toilets already have everything', field:fixture_exists, value:f8}, {quote:'The toilets already have everything', field:fixture_exists, value:f9}]\n"
        "'Nothing yet' (question just asked: which fixtures already exist?) -> [{quote:'Nothing yet', field:fixtures_none_exist, value:true}]\n"
        "'Skip the cost of the glazing, it is done' -> [{quote:'Skip the cost of the glazing', field:exclude_item, value:window}]\n"
        "'Count the front door again' -> [{quote:'Count the front door again', field:include_item, value:door_entrance}]\n"
        "'Rooms are bare' / 'Hello' / 'normal quality' -> []"
    ).replace("{codes}", ", ".join(rate_codes))


def facts_text(state: SessionState) -> str:
    return "\n".join(f"- {k} = {v.value} ({v.source})" for k, v in state.facts.items()) or "(none yet)"


def room_list(summary: PlanSummary) -> str:
    return "\n".join(f"- {r.id}: {r.name} ({r.type}, {r.area_m2:.1f} m²)" for r in summary.rooms) or "(no rooms)"


def fixture_list(summary: PlanSummary, priced_kinds: list[str]) -> str:
    return "\n".join(f"- {f.id}: {f.kind} in {f.room_id}" for f in summary.fixtures if f.kind in priced_kinds)


def question_asked(state: SessionState, last_assistant_message: str) -> str:
    if not state.pending:
        return ""
    return f"\nQuestion just asked (slot {state.pending[0]}): " + last_assistant_message[:600]


def message_block(message: str) -> str:
    return f"\n\nThe person's message:\n<message>\n{message}\n</message>"
