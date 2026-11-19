from __future__ import annotations

from typing import Protocol

from ...domain.model.estimate_session import SessionState
from ...domain.model.extracted import Extracted
from ...domain.model.plan_summary import PlanSummary


class KeywordReader(Protocol):
    """What plain code can read from a message with certainty (a keyword for the kind or quality, a direct answer to the
    question just asked, the agent's own chips) and the words that gate what the model may claim. No model call, no language
    beyond a few English and Vietnamese keywords; the model's reading stays the main path and is verified separately."""

    def read(self, state: SessionState, summary: PlanSummary, message: str) -> Extracted:
        """The fields the message states with certainty."""

    def exception_items(self, message: str) -> list[str]:
        """Opening rate codes the message says already exist by naming them as the exception or the only ones."""

    def says_unknown(self, message: str) -> bool:
        """The person gives up on the question ("I don't know", "you decide")."""

    def says_typical(self, message: str) -> bool:
        """The person asks for the typical percentages."""

    def names_removal(self, message: str) -> bool:
        """The words take work out of the estimate (remove, exclude, already exists, an exception)."""

    def names_addition(self, message: str) -> bool:
        """The words put work back (add, include, restore)."""

    def names_condition(self, message: str) -> bool:
        """The words talk about the condition of rooms (empty, bare, finished, replace); "exist" alone does not count."""

    def mentions_existing(self, message: str) -> bool:
        """The weak word "exist" is in the message."""

    def mentions_fixtures(self, message: str) -> bool:
        """The message talks about fixtures."""

    def referred_rooms(self, summary: PlanSummary, message: str) -> set[str]:
        """Rooms the message refers to: by their own name, else by their type ("the toilets")."""

    def referred_fixtures(self, summary: PlanSummary, priced_kinds: list[str], message: str) -> set[str]:
        """Priced fixture ids the message refers to: by kind, narrowed to a named room when it names one."""
