"""Server-streaming over a worker thread: the run emits events into a queue, the RPC yields them; when the client
leaves, the CancelToken is set and the run stops at its next progress step."""

from __future__ import annotations

import queue
import threading
from collections.abc import Callable, Iterator
from typing import Any

import grpc

from ...domain import progress, request
from ...domain.cancel import CancelToken

_DONE = object()


def stream_run(context: grpc.ServicerContext, run: Callable[[CancelToken], Any],
               to_event: Callable[[dict], Any], on_error: Callable[[Exception], Any], rid: str = "") -> Iterator[Any]:
    """Run `run(token)` in a worker with a progress sink installed. Each progress/other event dict goes through
    `to_event`; the return value of `run` is the final event (already a proto message); an exception becomes
    `on_error(exc)` (an in-band error event) because the stream is open. Cancelled ends the stream silently."""
    q: queue.Queue = queue.Queue()
    token = CancelToken()
    context.add_callback(token.cancel)

    def sink(ev: dict) -> None:
        token.check()
        q.put(to_event(ev))

    def work() -> None:
        try:
            with request.bind(rid), progress.sink(sink):
                q.put(run(token))
        except progress.Cancelled:
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
