from __future__ import annotations

from typing import Protocol

from ..model.rate import RateResult


class RateCache(Protocol):
    """Researched rates kept for a while: a rate is paid for once, not on every estimate."""

    def get(self, code: str, region: str, quality: str) -> RateResult | None:
        """The rate while it is fresh, else None."""

    def put(self, result: RateResult) -> None:
        """Keep a rate under its own code, region and quality."""
