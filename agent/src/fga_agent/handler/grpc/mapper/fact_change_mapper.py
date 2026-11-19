"""A change a message made to the session <-> the Agent's DTO (`EstimateEvent.FactEvent`). The value travels as text."""

from __future__ import annotations

from ....domain.model.estimate_session import FactChange
from fga.demo.agent.v1 import agent_pb2 as a


def _text(value: str | float | bool) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, float):
        return f"{value:g}"
    return str(value)


def to_pb(c: FactChange) -> a.EstimateEvent.FactEvent:
    return a.EstimateEvent.FactEvent(name=c.name, value=_text(c.value), source=c.source)


def to_domain(f: a.EstimateEvent.FactEvent) -> FactChange:
    return FactChange(name=f.name, value=f.value, source=f.source)
