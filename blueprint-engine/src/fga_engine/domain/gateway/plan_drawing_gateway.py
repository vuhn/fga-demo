from __future__ import annotations

from typing import Protocol

from ..schemas import BuildingModel


class PlanDrawing(Protocol):
    """Draws a model into a DXF and reads a DXF back into plans (our own drawing round trip)."""

    def draw(self, model: BuildingModel, title: str) -> bytes:
        """The model drawn into a DXF file."""

    def read(self, data: bytes) -> list[BuildingModel]:
        """The plans found in a DXF file, as imported (with provenance)."""
