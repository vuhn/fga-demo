from __future__ import annotations

from typing import Protocol

from ..domain.estimate import EstimateFacts, Takeoff
from ..domain.schemas import BuildingModel


class QuantityTakeoff(Protocol):
    def takeoff(self, model: BuildingModel, facts: EstimateFacts) -> Takeoff:
        """Quantities per rate code from the plan and the interview facts; status "unsupported" for a plan with no rooms."""
