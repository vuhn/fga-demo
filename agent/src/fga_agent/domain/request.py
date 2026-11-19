"""The opaque request id of the call being served (`x-request-id`); forwarded to the Engine and logged."""

from __future__ import annotations

from contextlib import contextmanager
from contextvars import ContextVar
from collections.abc import Iterator

_id: ContextVar[str] = ContextVar("request_id", default="")


def current() -> str:
    return _id.get()


@contextmanager
def bind(request_id: str) -> Iterator[None]:
    token = _id.set(request_id)
    try:
        yield
    finally:
        _id.reset(token)
