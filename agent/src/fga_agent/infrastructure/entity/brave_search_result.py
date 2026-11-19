"""Brave Search's `/res/v1/web/search` wire shape. It exists only inside the Brave adapter."""

from __future__ import annotations

import html
import re
from dataclasses import dataclass


@dataclass(frozen=True)
class BraveResult:
    url: str
    title: str
    description: str  # carries <strong> tags around the matched words

    @staticmethod
    def from_response(body: dict) -> list[BraveResult]:
        rows = body.get("web", {}).get("results", [])
        return [BraveResult(x["url"], x.get("title", ""), x.get("description", "")) for x in rows if x.get("url")]

    def snippet(self) -> str:
        return html.unescape(re.sub("<[^>]+>", "", self.description))
