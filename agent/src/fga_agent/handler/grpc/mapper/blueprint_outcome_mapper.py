"""The Agent's DTOs <-> domain for a chat revise: the request in, the outcome out as the stream's result event."""

from __future__ import annotations

from ....domain.model.blueprint_outcome import BlueprintOutcome
from fga.demo.agent.v1 import agent_pb2 as a
from . import plan_check_mapper


def revise_to_domain(req: a.ReviseRequest) -> tuple[bytes, bytes, str, list[str]]:
    return req.brief_json, req.previous_model_json, req.message, list(req.history)


def to_pb(x: BlueprintOutcome) -> a.BlueprintOutcome:
    return a.BlueprintOutcome(brief_json=x.brief, model_json=x.model, checks=[plan_check_mapper.to_pb(c) for c in x.checks],
                              issues=x.issues, ok=x.ok, notes=x.notes, attempts=x.llm_attempts, history=x.history)


def to_event(x: BlueprintOutcome) -> a.BlueprintEvent:
    return a.BlueprintEvent(result=to_pb(x))
