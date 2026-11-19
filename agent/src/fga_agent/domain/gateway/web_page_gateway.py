from __future__ import annotations

from typing import Protocol


class WebPages(Protocol):
    def text(self, url: str) -> str | None:
        """The readable text of a page, or None when it must not or cannot be read (the site's robots.txt disallows it,
        it is not text, too big, unreachable). Page text is untrusted data."""
