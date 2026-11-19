from __future__ import annotations

from collections.abc import Callable
from typing import Any

from ...domain.errors import InvalidEdit, UnsupportedEdit
from ...domain.gateway import LanguageModel
from ...domain.model.call_context import CallContext
from ...domain.model.progress import Progress
from ..prompts import conversation


class FeedbackAskerService:
    def __init__(self, llm: LanguageModel) -> None:
        self._llm = llm

    def ask(self, ctx: CallContext, system: str, user: str, schema: dict, apply: Callable[[dict], Any],
            on_progress: Callable[[Progress], None], attempts: int = 3) -> tuple[dict, Any, int]:
        prompt = user
        last: InvalidEdit | None = None
        for n in range(1, attempts + 1):
            on_progress(Progress("step", "ask_model", f"Asking the model… attempt {n}/{attempts}"))
            out = self._llm.answer_json(system, prompt, schema, on_progress=on_progress)
            try:
                return out, apply(out), n
            except UnsupportedEdit:
                raise
            except InvalidEdit as e:
                last = e
                if n < attempts:
                    on_progress(Progress("step", "retry",
                                         f"Attempt {n} could not be applied ({str(e)[:160]}); sending the error back"))
                prompt = user + conversation.feedback(out, str(e))
        raise InvalidEdit(f"{last} (after {attempts} attempts)")
