"""Handler helper for server-streaming RPCs (used by the `<domain>_handler.py` classes): a worker thread runs the use
case; its progress callback puts each step on a queue and the RPC yields what arrives. When the client leaves, the
token is cancelled and the run stops at its next progress step."""

from __future__ import annotations

import queue
import threading
from collections.abc import Callable, Iterator
from typing import Any

import grpc

from ....domain.errors import Cancelled
from ....domain.model.progress import Progress

_DONE = object()


class _Failed:
    """A run's exception, queued so the RPC thread decides whether the stream had opened."""

    def __init__(self, error: Exception) -> None:
        self.error = error


def ignore_progress(_: Progress) -> None:
    """The progress callback of a unary RPC: nobody is listening."""


class CancelToken:
    """Set when the client leaves; the progress callback checks it, so a run stops at its next step."""

    def __init__(self) -> None:
        self._ev = threading.Event()

    def cancel(self) -> None:
        self._ev.set()

    @property
    def cancelled(self) -> bool:
        return self._ev.is_set()

    def check(self) -> None:
        if self._ev.is_set():
            raise Cancelled()


def stream_run(context: grpc.ServicerContext, run: Callable[[Callable[[Progress], None]], Any],
               to_event: Callable[[Progress], Any], on_error: Callable[[Exception], Any],
               refuse: Callable[[Exception], None] | None = None) -> Iterator[Any]:
    """Run `run(on_progress)` in a worker. Each progress step goes through `to_event`; the return value of `run` is
    the final event (already a proto message). An exception after the first event becomes `on_error(exc)` (an in-band
    error event, because the stream is open). Before the first event `refuse(exc)` gets it first: it aborts the call
    with a status (it does not return) for an error the caller refuses, and returns for one it does not.
    Cancelled ends the stream silently."""
    q: queue.Queue = queue.Queue()
    token = CancelToken()
    context.add_callback(token.cancel)

    def on_progress(p: Progress) -> None:
        token.check()
        q.put(to_event(p))

    def work() -> None:
        try:
            q.put(run(on_progress))
        except Cancelled:
            pass
        except Exception as e:  # noqa: BLE001 - reported in-band, or refused before the stream opened
            q.put(_Failed(e))
        finally:
            q.put(_DONE)

    threading.Thread(target=work, daemon=True).start()
    opened = False
    while True:
        item = q.get()
        if item is _DONE:
            return
        if isinstance(item, _Failed):
            if not opened and refuse is not None:
                refuse(item.error)
            item = on_error(item.error)
        opened = True
        yield item
