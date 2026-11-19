from __future__ import annotations

import threading
import time
from collections.abc import Callable

from ...domain.model.rate import RateResult

SEVEN_DAYS = 7 * 24 * 3600


class InMemoryRateCache:
    """Kept in this process only: the Agent is stateless, so a restart researches again."""

    def __init__(self, ttl_seconds: float = SEVEN_DAYS, clock: Callable[[], float] = time.time) -> None:
        self._ttl, self._clock = ttl_seconds, clock
        self._rates: dict[tuple[str, str, str], tuple[float, RateResult]] = {}
        self._lock = threading.Lock()

    def get(self, code: str, region: str, quality: str) -> RateResult | None:
        with self._lock:
            hit = self._rates.get((code, region, quality))
            return hit[1] if hit and self._clock() - hit[0] < self._ttl else None

    def put(self, result: RateResult) -> None:
        with self._lock:
            self._rates[(result.code, result.region, result.quality)] = (self._clock(), result)
