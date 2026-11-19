"""Interfaces the services need from the outside world; infrastructure implements them.

One file per collaborator (`<kind>_gateway.py`), like `api/internal/domain/gateway/<kind>_gateway.go`. This file only
re-exports: a new gateway is a new file plus one line here."""

from .dxf_patching_gateway import DxfPatching
from .dxf_reading_gateway import DxfReading
from .plan_conversion_gateway import PlanConversion
from .plan_drawing_gateway import PlanDrawing
from .plan_rendering_gateway import PlanRendering

__all__ = ["DxfPatching", "DxfReading", "PlanConversion", "PlanDrawing", "PlanRendering"]
