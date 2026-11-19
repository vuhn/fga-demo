"""What an estimate turn reports while it runs -> the Agent's stream message (`EstimateEvent`). Each part is converted by the mapper
of its own model; this one only chooses the slot."""

from __future__ import annotations

from typing import Any

from ....domain.model.estimate import Estimate
from ....domain.model.estimate_session import FactChange, Question
from ....domain.model.progress import Progress
from ....domain.model.rate import RateResult
from ....domain.model.reply_token import ReplyToken
from fga.demo.agent.v1 import agent_pb2 as a
from . import estimate_mapper, fact_change_mapper, progress_mapper, question_mapper, rate_mapper, reply_token_mapper


def to_event(event: Any) -> a.EstimateEvent:
    if isinstance(event, Progress):
        return a.EstimateEvent(progress=progress_mapper.to_pb(event))
    if isinstance(event, ReplyToken):
        return a.EstimateEvent(token=reply_token_mapper.to_pb(event))
    if isinstance(event, FactChange):
        return a.EstimateEvent(fact=fact_change_mapper.to_pb(event))
    if isinstance(event, Question):
        return a.EstimateEvent(question=question_mapper.to_pb(event))
    if isinstance(event, RateResult):
        return a.EstimateEvent(rate=rate_mapper.to_pb(event))
    if isinstance(event, Estimate):
        return a.EstimateEvent(estimate_json=estimate_mapper.to_json(event))
    raise TypeError(f"no stream message for {type(event).__name__}")
