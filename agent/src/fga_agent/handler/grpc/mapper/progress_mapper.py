"""Progress reports -> the Agent's stream events. A report is the dict a ProgressReporter produces."""

from __future__ import annotations

from ....gen.fga.demo.agent.v1 import agent_pb2 as a


def to_progress(ev: dict) -> a.Progress:
    return a.Progress(kind=ev.get("kind", "step"), stage=ev.get("stage", ""), message=ev.get("message", ""))


def to_blueprint_event(ev: dict) -> a.BlueprintEvent:
    """Only progress reports exist on the blueprint streams; the result and error events are built by their mappers."""
    return a.BlueprintEvent(progress=to_progress(ev))
