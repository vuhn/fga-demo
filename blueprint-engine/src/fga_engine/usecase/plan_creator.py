from __future__ import annotations

from typing import Protocol

from ..domain.results import PlanCreated
from ..domain.schemas import Brief


class PlanCreator(Protocol):
    def create_plan(self, brief: Brief) -> PlanCreated:
        """A brief drawn into our own DXF and read back as an editable CAD plan (no brief afterwards)."""
