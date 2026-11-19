"""Cancellation of a streaming call: the handler sets the token when the client leaves; the progress sink raises
Cancelled at the next step so the run stops without finishing."""

from __future__ import annotations

import threading

from .progress import Cancelled


class CancelToken:
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
