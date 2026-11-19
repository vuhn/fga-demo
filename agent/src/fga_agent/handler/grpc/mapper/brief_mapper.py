"""The Agent's DTOs <-> domain for drafting a brief from text (`BriefFromText`), both directions."""

from __future__ import annotations

from ....domain.model.brief import BriefDraft
from ....gen.fga.demo.agent.v1 import agent_pb2 as a


def draft_to_domain(req: a.BriefFromTextRequest) -> tuple[str, bool]:
    return req.prompt, req.enhance


def draft_to_pb(draft: BriefDraft) -> a.BriefFromTextResponse:
    return a.BriefFromTextResponse(brief_json=draft.brief, notes=draft.notes)
