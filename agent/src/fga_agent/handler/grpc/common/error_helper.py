"""Status mapping for the Agent's handlers: refusals are INVALID_ARGUMENT with an ErrorDetail in the
`error-detail-bin` trailer; a dependency that is down is UNAVAILABLE with `dependency` set."""

from __future__ import annotations

import functools
import logging
import uuid

import grpc

from ....domain.model.call_context import CallContext
from ....domain.errors import EngineUnavailable, InvalidEdit, LlmUnavailable
from ..mapper import error_mapper

log = logging.getLogger("fga_agent.grpc")
DETAIL_KEY = "error-detail-bin"
REQUEST_ID = "x-request-id"


def call_context(context: grpc.ServicerContext) -> CallContext:
    """The request id the caller sent, or a fresh one."""
    for k, v in context.invocation_metadata() or ():
        if k == REQUEST_ID:
            return CallContext(request_id=v)
    return CallContext(request_id=uuid.uuid4().hex[:12])


def map_exception(context: grpc.ServicerContext, err: Exception) -> None:
    """Abort the call with the status for err (does not return); re-raise what is not a known domain error."""
    mapped = error_mapper.to_status(err)
    if mapped is None:
        raise err
    status, detail = mapped
    context.set_trailing_metadata(((DETAIL_KEY, detail.SerializeToString()),))
    context.abort(status, detail.message)


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
