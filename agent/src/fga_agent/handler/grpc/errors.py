"""Status mapping for the Agent's handlers: refusals are INVALID_ARGUMENT with an ErrorDetail in the
`error-detail-bin` trailer; a dependency that is down is UNAVAILABLE with `dependency` set."""

from __future__ import annotations

import functools
import logging
import uuid

import grpc

from ...domain.call_context import CallContext
from ...domain.errors import EngineUnavailable, InvalidEdit, LlmUnavailable
from ...gen.fga.demo.common.v1 import common_pb2 as c

log = logging.getLogger("fga_agent.grpc")
DETAIL_KEY = "error-detail-bin"
REQUEST_ID = "x-request-id"


def call_context(context: grpc.ServicerContext) -> CallContext:
    """The request id the caller sent, or a fresh one."""
    for k, v in context.invocation_metadata() or ():
        if k == REQUEST_ID:
            return CallContext(request_id=v)
    return CallContext(request_id=uuid.uuid4().hex[:12])


def abort(context: grpc.ServicerContext, status: grpc.StatusCode, code: int, message: str, dependency: str = ""):
    detail = c.ErrorDetail(code=code, message=message[:600], dependency=dependency)
    context.set_trailing_metadata(((DETAIL_KEY, detail.SerializeToString()),))
    context.abort(status, message[:600])


def map_exception(context: grpc.ServicerContext, err: Exception) -> None:
    """Abort the call with the status for err (does not return); re-raise what is not a known domain error."""
    if isinstance(err, InvalidEdit):
        code = c.ERROR_CODE_UNSUPPORTED if err.code == "unsupported" else c.ERROR_CODE_EDIT_ERROR
        abort(context, grpc.StatusCode.INVALID_ARGUMENT, code, str(err))
    if isinstance(err, EngineUnavailable):
        abort(context, grpc.StatusCode.UNAVAILABLE, c.ERROR_CODE_DEPENDENCY_UNAVAILABLE, str(err), "engine")
    if isinstance(err, LlmUnavailable):
        abort(context, grpc.StatusCode.UNAVAILABLE, c.ERROR_CODE_DEPENDENCY_UNAVAILABLE, str(err), "llm")
    raise err


def handled(fn):
    """Wrap a unary method: log with the request id and map domain errors to gRPC statuses."""

    @functools.wraps(fn)
    def wrapper(self, req, context):
        rid = call_context(context).request_id
        try:
            out = fn(self, req, context)
            log.info("rid=%s rpc=%s ok", rid, fn.__name__)
            return out
        except (InvalidEdit, EngineUnavailable, LlmUnavailable) as e:
            log.info("rid=%s rpc=%s failed: %s", rid, fn.__name__, e)
            map_exception(context, e)

    return wrapper
