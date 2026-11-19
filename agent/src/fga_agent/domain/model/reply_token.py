from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ReplyToken:
    """A piece of the reply as it is written."""

    text: str
