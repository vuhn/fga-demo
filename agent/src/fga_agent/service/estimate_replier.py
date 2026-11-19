from __future__ import annotations

from collections.abc import Callable
from typing import Any

from ..domain.errors import LlmUnavailable
from ..domain.gateway import LanguageModel
from ..domain.model.call_context import CallContext
from ..domain.model.reply_token import ReplyToken
from .prompts import estimate_reply as prompt


class EstimateReplierService:
    def __init__(self, llm: LanguageModel) -> None:
        self._llm = llm

    def say(self, ctx: CallContext, instruction: str, data: dict, fallback: str, on_event: Callable[[Any], None]) -> str:
        parts: list[str] = []
        try:
            for piece in self._llm.answer_text(prompt.REPLY_SYSTEM, prompt.reply_user(instruction, data)):
                parts.append(piece)
                on_event(ReplyToken(piece))
        except LlmUnavailable:
            if parts:  # some text is already out: the reply cannot be finished from a fixed text
                raise
        text = "".join(parts)
        if not text.strip():
            on_event(ReplyToken(fallback))
            return fallback
        return text
