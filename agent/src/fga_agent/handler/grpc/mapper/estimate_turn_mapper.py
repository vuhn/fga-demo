"""An estimate turn <-> the Agent's DTOs: the request in, the final `Done` message out."""

from __future__ import annotations

from ....domain.model.estimate_turn import EstimateTurnRequest, EstimateTurnResult
from fga.demo.agent.v1 import agent_pb2 as a
from . import estimate_mapper, message_mapper, session_state_mapper


def to_domain(r: a.EstimateTurnRequest) -> EstimateTurnRequest:
    return EstimateTurnRequest(model=r.model_json, state=session_state_mapper.from_json(r.state_json), message=r.message,
                               messages=[message_mapper.to_domain(m) for m in r.messages], rate_overrides=dict(r.rate_overrides))


def to_done_event(result: EstimateTurnResult) -> a.EstimateEvent:
    return a.EstimateEvent(done=a.EstimateEvent.Done(
        state_json=session_state_mapper.to_json(result.state), status=result.state.status, reply=result.reply,
        estimate_json=estimate_mapper.to_json(result.estimate) if result.estimate else b""))
