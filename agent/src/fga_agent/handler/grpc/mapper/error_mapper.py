"""Domain errors -> the Agent's DTOs: a gRPC status with an ErrorDetail (before a stream opens) or an in-band
`Error` event (after the first message)."""

from __future__ import annotations

import grpc

from ....domain.errors import EngineUnavailable, InvalidEdit, LlmUnavailable
from fga.demo.agent.v1 import agent_pb2 as a
from fga.demo.common.v1 import common_pb2 as c

_INTERNAL = "internal error"


def to_status(err: Exception) -> tuple[grpc.StatusCode, c.ErrorDetail] | None:
    """The gRPC status and detail for a domain error, or None when it is not one (the caller re-raises)."""
    if isinstance(err, InvalidEdit):
        code = c.ERROR_CODE_UNSUPPORTED if err.code == "unsupported" else c.ERROR_CODE_EDIT_ERROR
        return grpc.StatusCode.INVALID_ARGUMENT, c.ErrorDetail(code=code, message=str(err)[:600])
    if isinstance(err, EngineUnavailable):
        return grpc.StatusCode.UNAVAILABLE, c.ErrorDetail(code=c.ERROR_CODE_DEPENDENCY_UNAVAILABLE,
                                                          message=str(err)[:600], dependency="engine")
    if isinstance(err, LlmUnavailable):
        return grpc.StatusCode.UNAVAILABLE, c.ErrorDetail(code=c.ERROR_CODE_DEPENDENCY_UNAVAILABLE,
                                                          message=str(err)[:600], dependency="llm")
    return None


def to_error(err: Exception) -> a.Error:
    """The in-band error the API relays: 422 invalid_edit, 502 llm_unavailable / engine_unavailable, 500 internal."""
    if isinstance(err, InvalidEdit):
        return a.Error(status=422, error="invalid_edit", message=str(err)[:600])
    if isinstance(err, EngineUnavailable):
        return a.Error(status=502, error="engine_unavailable", message=str(err)[:600])
    if isinstance(err, LlmUnavailable):
        return a.Error(status=502, error="llm_unavailable", message=str(err)[:600])
    return a.Error(status=500, error="internal", message=_INTERNAL)


def to_blueprint_error_event(err: Exception) -> a.BlueprintEvent:
    return a.BlueprintEvent(error=to_error(err))
