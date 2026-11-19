from __future__ import annotations

from collections.abc import Callable
from typing import Any, Protocol

from ..domain.model.call_context import CallContext


class EstimateReplier(Protocol):
    """Writes the next chat message of the estimate conversation, streamed as it is generated."""

    def say(self, ctx: CallContext, instruction: str, data: dict, fallback: str, on_event: Callable[[Any], None]) -> str:
        """The reply to the person's last message, in their language, from the instruction and only the figures in `data`;
        each piece is given to `on_event` as a `ReplyToken`. When the model fails before any text, `fallback` is the reply
        (and is sent as one token); after some text, LlmUnavailable."""
