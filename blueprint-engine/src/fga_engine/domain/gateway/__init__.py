"""Interfaces the services need from file-format code; infrastructure implements them."""

from __future__ import annotations

from typing import Protocol

from ..files import ConvertReport, ImportResult, PatchReport, TitleBlock
from ..schemas import BuildingModel


class PlanDrawing(Protocol):
    """Draws a model into a DXF and reads a DXF back into plans (our own drawing round trip)."""

    def draw(self, model: BuildingModel, title: str) -> bytes:
        """The model drawn into a DXF file."""

    def read(self, data: bytes) -> list[BuildingModel]:
        """The plans found in a DXF file, as imported (with provenance)."""


class PlanRendering(Protocol):
    def render(self, model: BuildingModel, fmt: str, title_block: TitleBlock) -> bytes | str:
        """The model as an svg (str), dxf, pdf or ifc (bytes) file."""


class DxfReading(Protocol):
    def read_plans(self, data: bytes, unit: str | None) -> ImportResult:
        """The plans found in a CAD file; EditError when it is not a usable file."""


class PlanConversion(Protocol):
    def to_dxf(self, data: bytes, mime: str, scale: int | None, dpi: float | None,
               longest_m: float | None) -> tuple[bytes, ConvertReport]:
        """A PDF, PNG or JPEG floor plan converted into a DXF (metres)."""


class DxfPatching(Protocol):
    def patch(self, original: bytes, base: BuildingModel, current: BuildingModel) -> tuple[bytes, PatchReport]:
        """The original CAD file with the changes between base and current written into it."""
