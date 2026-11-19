"""A chat message <-> the Agent's DTO (`ChatMessage`)."""

from __future__ import annotations

from ....domain.model.estimate_session import Message
from fga.demo.agent.v1 import agent_pb2 as a


def to_domain(m: a.ChatMessage) -> Message:
    return Message(role=m.role, content=m.content)


def to_pb(m: Message) -> a.ChatMessage:
    return a.ChatMessage(role=m.role, content=m.content)
