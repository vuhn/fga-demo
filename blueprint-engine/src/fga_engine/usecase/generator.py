from __future__ import annotations

from typing import Protocol

from ..domain.results import BriefResult, StepResult
from ..domain.schemas import Brief, BuildingModel


class Generator(Protocol):
    def generate(self, brief: Brief, previous: BuildingModel | None = None, *, repair: bool = False) -> StepResult:
        """Brief -> layout -> model -> checks. `repair` shrinks rooms toward their minimums when the brief
        cannot fit (create flows); without it the user's numbers are never changed."""

    def generate_brief(self, brief: Brief, previous: BuildingModel | None = None, *, repair: bool = False) -> BriefResult:
        """`generate` with the extras an LLM loop reads (score, failed rules, targets)."""
