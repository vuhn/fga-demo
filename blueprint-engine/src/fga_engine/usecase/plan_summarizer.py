from __future__ import annotations

from typing import Protocol

from ..domain.estimate import EstimateFacts
from ..domain.results import PlanSummary
from ..domain.schemas import BuildingModel


class PlanSummarizer(Protocol):
    def summarize_plan(self, model: BuildingModel, facts: EstimateFacts) -> PlanSummary:
        """What an estimate interview needs to know about a plan: rooms, fixtures, openings, and what the facts leave open."""
