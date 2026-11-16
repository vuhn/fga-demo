"""Live progress for the streaming endpoints. Steps call emit(); with no sink installed it does
nothing, so the sync endpoints and the tests are unaffected. A sink may raise Cancelled (the
client went away) to stop the run at the next step."""

from __future__ import annotations

from collections.abc import Callable, Iterator
from contextlib import contextmanager
from contextvars import ContextVar

Sink = Callable[[dict], None]
_sink: ContextVar[Sink | None] = ContextVar("progress_sink", default=None)


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


_quiet: ContextVar[bool] = ContextVar("progress_quiet_thinking", default=False)


@contextmanager
def quiet_thinking() -> Iterator[None]:
    """Narrow, mechanical LLM calls (reading a message into fields) do not need to think out loud: the
    client skips the thinking stream for them, which is much faster on a local model."""
    token = _quiet.set(True)
    try:
        yield
    finally:
        _quiet.reset(token)


def thinking_wanted() -> bool:
    return _sink.get() is not None and not _quiet.get()


def active() -> bool:
    """True while a streaming endpoint is listening (the LLM client then streams the model's thinking)."""
    return _sink.get() is not None


def emit_thinking(text: str) -> None:
    fn = _sink.get()
    if fn is not None and text:
        fn({"event": "progress", "kind": "thinking", "stage": "thinking", "message": text})


@contextmanager
def sink(fn: Sink) -> Iterator[None]:
    token = _sink.set(fn)
    try:
        yield
    finally:
        _sink.reset(token)
