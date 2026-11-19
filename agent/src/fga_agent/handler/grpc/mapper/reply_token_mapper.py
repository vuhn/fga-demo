"""A piece of the reply <-> the Agent's DTO (`EstimateEvent.Token`)."""

from __future__ import annotations

from ....domain.model.reply_token import ReplyToken
from fga.demo.agent.v1 import agent_pb2 as a


def to_pb(t: ReplyToken) -> a.EstimateEvent.Token:
    return a.EstimateEvent.Token(text=t.text)


def to_domain(t: a.EstimateEvent.Token) -> ReplyToken:
    return ReplyToken(t.text)
