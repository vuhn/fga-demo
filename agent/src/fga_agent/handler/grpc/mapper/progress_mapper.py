"""Progress DTO <-> domain (`agent.v1.Progress`), both directions, and its place in the blueprint stream."""

from __future__ import annotations

from ....domain.model.progress import Progress
from ....gen.fga.demo.agent.v1 import agent_pb2 as a


def to_pb(p: Progress) -> a.Progress:
    return a.Progress(kind=p.kind, stage=p.stage, message=p.message)


def to_domain(x: a.Progress) -> Progress:
    return Progress(kind=x.kind or "step", stage=x.stage, message=x.message)


def to_blueprint_event(p: Progress) -> a.BlueprintEvent:
    """Only progress steps exist on the blueprint streams; the result and error events are built by their mappers."""
    return a.BlueprintEvent(progress=to_pb(p))
