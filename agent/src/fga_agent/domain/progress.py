"""Live progress for the streaming RPCs. Steps call emit(); with no sink installed it does nothing, so unary calls
and tests are unaffected. A sink may raise Cancelled (the client went away) to stop the run at the next step."""

from __future__ import annotations

from collections.abc import Callable, Iterator
from contextlib import contextmanager
from contextvars import ContextVar

Sink = Callable[[dict], None]
_sink: ContextVar[Sink | None] = ContextVar("progress_sink", default=None)
_quiet: ContextVar[bool] = ContextVar("progress_quiet_thinking", default=False)


class Cancelled(Exception):
    pass


def emit(stage: str, message: str) -> None:
    fn = _sink.get()
    if fn is not None:
        fn({"event": "progress", "kind": "step", "stage": stage, "message": message})


def emit_event(name: str, data: dict) -> None:
    """A non-progress event for the chat streams (token, fact, question, rate, estimate)."""
    fn = _sink.get()
    if fn is not None:
        fn({"event": name, "data": data})


def emit_thinking(text: str) -> None:
    fn = _sink.get()
    if fn is not None and text:
        fn({"event": "progress", "kind": "thinking", "stage": "thinking", "message": text})


@contextmanager
def quiet_thinking() -> Iterator[None]:
    """Narrow, mechanical LLM calls do not need to think out loud: the client skips the thinking stream for them."""
    token = _quiet.set(True)
    try:
        yield
    finally:
        _quiet.reset(token)


def thinking_wanted() -> bool:
    return _sink.get() is not None and not _quiet.get()


@contextmanager
def sink(fn: Sink) -> Iterator[None]:
    token = _sink.set(fn)
    try:
        yield
    finally:
        _sink.reset(token)
