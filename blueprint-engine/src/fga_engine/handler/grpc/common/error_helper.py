"""Handler helper: status mapping. Refusals are INVALID_ARGUMENT with an ErrorDetail in the `error-detail-bin` trailer."""

from __future__ import annotations

import functools
import logging
import uuid

import grpc
from pydantic import ValidationError

from ....domain.edits import EditError
from ....gen.fga.demo.common.v1 import common_pb2 as c

log = logging.getLogger("fga_engine.grpc")
DETAIL_KEY = "error-detail-bin"
REQUEST_ID = "x-request-id"


def request_id(context: grpc.ServicerContext) -> str:
    for k, v in context.invocation_metadata() or ():
        if k == REQUEST_ID:
            return v
    return uuid.uuid4().hex[:12]


def abort(context: grpc.ServicerContext, status: grpc.StatusCode, code: int, message: str):
    context.set_trailing_metadata(((DETAIL_KEY, c.ErrorDetail(code=code, message=message[:600]).SerializeToString()),))
    context.abort(status, message[:600])


def handled(fn):
    """Wrap a unary method: log with the request id and map errors to gRPC statuses."""

    @functools.wraps(fn)
    def wrapper(self, request, context):
        rid = request_id(context)
        try:
            out = fn(self, request, context)
            log.info("rid=%s rpc=%s ok", rid, fn.__name__)
            return out
        except EditError as e:  # also UnsupportedEdit
            log.info("rid=%s rpc=%s refused: %s", rid, fn.__name__, e)
            code = c.ERROR_CODE_UNSUPPORTED if e.code == "unsupported" else c.ERROR_CODE_EDIT_ERROR
            abort(context, grpc.StatusCode.INVALID_ARGUMENT, code, str(e))
        except (ValidationError, ValueError) as e:
            log.info("rid=%s rpc=%s bad input: %s", rid, fn.__name__, str(e)[:200])
            abort(context, grpc.StatusCode.INVALID_ARGUMENT, c.ERROR_CODE_EDIT_ERROR, str(e))

    return wrapper
