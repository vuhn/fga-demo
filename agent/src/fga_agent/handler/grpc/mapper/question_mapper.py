"""A question of the interview <-> the Agent's DTO (`EstimateQuestion`)."""

from __future__ import annotations

from ....domain.model.estimate_session import Question
from fga.demo.agent.v1 import agent_pb2 as a


def to_pb(q: Question) -> a.EstimateQuestion:
    return a.EstimateQuestion(id=q.id, text=q.text, options=q.options, allow_unknown=q.allow_unknown)


def to_domain(q: a.EstimateQuestion) -> Question:
    return Question(id=q.id, text=q.text, options=list(q.options), allow_unknown=q.allow_unknown)
