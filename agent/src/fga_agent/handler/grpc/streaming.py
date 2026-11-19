"""Server-streaming over a worker thread: the run reports into a queue through a ProgressReporter, the RPC yields what
arrives; when the client leaves, the CancelToken is set and the run stops at its next report."""

from __future__ import annotations

import queue
import threading
from collections.abc import Callable, Iterator
from typing import Any

import grpc

from ...domain.cancel import Cancelled, CancelToken
from ...domain.gateway import ProgressReporter

_DONE = object()


class QueueReporter:
    """Implements gateway.ProgressReporter: each report becomes a proto event on the queue, after a cancel check."""

    def __init__(self, q: queue.Queue, token: CancelToken, to_event: Callable[[dict], Any]) -> None:
        self._q, self._token, self._to_event = q, token, to_event

    def _put(self, ev: dict) -> None:
        self._token.check()
        self._q.put(self._to_event(ev))

    def step(self, stage: str, message: str) -> None:
        self._put({"event": "progress", "kind": "step", "stage": stage, "message": message})

    def thinking(self, text: str) -> None:
        if text:
            self._put({"event": "progress", "kind": "thinking", "stage": "thinking", "message": text})

    def event(self, name: str, data: dict) -> None:
        self._put({"event": name, "data": data})


def stream_run(context: grpc.ServicerContext, run: Callable[[ProgressReporter], Any],
               to_event: Callable[[dict], Any], on_error: Callable[[Exception], Any]) -> Iterator[Any]:
    """Run `run(reporter)` in a worker. Reports go through `to_event`; the return value of `run` is the final event
    (already a proto message); an exception becomes `on_error(exc)` (an in-band error event, because the stream is
    open). Cancelled ends the stream silently."""
    q: queue.Queue = queue.Queue()
    token = CancelToken()
    context.add_callback(token.cancel)

    def work() -> None:
        try:
            q.put(run(QueueReporter(q, token, to_event)))
        except Cancelled:
            pass
        except Exception as e:  # noqa: BLE001 - reported in-band
            q.put(on_error(e))
        finally:
            q.put(_DONE)

    threading.Thread(target=work, daemon=True).start()
    while True:
        item = q.get()
        if item is _DONE:
            return
        yield item
