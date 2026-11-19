from __future__ import annotations

from collections.abc import Callable
from typing import Any, Protocol

from ...domain.model.call_context import CallContext
from ...domain.model.progress import Progress


class FeedbackAsker(Protocol):
    """Asks the model for JSON and applies it; when applying fails, sends the answer and the error back (Text2BIM Eq. 4)
    and asks again, up to `attempts` times. Shared by every step that applies model output."""

    def ask(self, ctx: CallContext, system: str, user: str, schema: dict, apply: Callable[[dict], Any],
            on_progress: Callable[[Progress], None], attempts: int = 3) -> tuple[dict, Any, int]:
        """The model's answer, what `apply` made of it, and the number of tries. UnsupportedEdit is not retried; after
        the last try the last InvalidEdit is raised."""
