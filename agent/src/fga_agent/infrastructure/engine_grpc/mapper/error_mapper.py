"""Engine gRPC failure -> the Agent's domain errors (the ErrorDetail DTO in the trailer is read here)."""

from __future__ import annotations

import grpc

from ....domain.errors import EngineUnavailable, InvalidEdit, UnsupportedEdit
from fga.demo.common.v1 import common_pb2 as c

DETAIL_KEY = "error-detail-bin"


def detail_of(err: grpc.RpcError) -> c.ErrorDetail | None:
    d = c.ErrorDetail()
    for k, v in err.trailing_metadata() or ():
        if k == DETAIL_KEY:
            try:
                d.ParseFromString(v if isinstance(v, bytes) else v.encode("latin-1"))
                return d
            except Exception:  # noqa: BLE001 - a malformed detail falls back to the status message
                return None
    return None


def to_domain(rpc: str, err: grpc.RpcError) -> Exception:
    code = err.code()
    detail = detail_of(err)
    message = (detail.message if detail and detail.message else err.details()) or code.name
    if code == grpc.StatusCode.INVALID_ARGUMENT:
        if detail and detail.code == c.ERROR_CODE_UNSUPPORTED:
            return UnsupportedEdit(message)
        return InvalidEdit(message)
    return EngineUnavailable(f"engine {rpc}: {code.name}: {message}")
