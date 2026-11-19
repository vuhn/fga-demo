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
               to_event: Callable[[Progress], Any], on_error: Callable[[Exception], Any]) -> Iterator[Any]:
    """Run `run(on_progress)` in a worker. Each progress step goes through `to_event`; the return value of `run` is
    the final event (already a proto message); an exception becomes `on_error(exc)` (an in-band error event,
    because the stream is open). Cancelled ends the stream silently."""
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
