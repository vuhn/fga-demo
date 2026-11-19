from __future__ import annotations

from typing import Protocol

from ..domain.results import PlanCreated
from ..domain.schemas import BuildingModel


class PlanDrawer(Protocol):
    def draw_plan(self, model: BuildingModel, title: str) -> PlanCreated:
        """A generated plan drawn into a DXF and read back as a CAD plan; EditError when it is already one."""
