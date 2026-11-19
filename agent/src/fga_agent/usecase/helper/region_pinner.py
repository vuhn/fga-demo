from __future__ import annotations

from typing import Protocol


class RegionPinner(Protocol):
    """The region is a fact the person gave, so code owns it in every search query; the model is told not to name a place."""

    def pin(self, query: str, region: str) -> str | None:
        """The query with the region appended when absent, or None when it names another city."""
