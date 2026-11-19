"""Cancellation of a streaming call: the handler sets the token when the client leaves; the progress reporter raises
Cancelled at the next step so the run stops without finishing."""

from __future__ import annotations

import threading


class Cancelled(Exception):
    """The client left; stop quietly (nothing is written, nothing is reported)."""


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
