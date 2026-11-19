"""Result and option shapes of the file operations (render, import, convert, patch): the JSON the HTTP handlers send."""

from __future__ import annotations

from dataclasses import dataclass

from pydantic import BaseModel, Field

from .schemas import BuildingModel, CheckResult

MAX_BYTES = 20 << 20  # the largest CAD file or picture the engine reads


@dataclass
class TitleBlock:
    project: str = "Single-storey house"
    sheet: str = "A-101"
    sheet_title: str = "Ground floor plan"
    version: int = 1
    issued: str = ""
    author: str = ""


class ImportReport(BaseModel):
    units: str  # "mm", "m", ...
    unit_source: str  # how the unit was decided
    layers_used: list[str] = Field(default_factory=list)
    layers_ignored: list[str] = Field(default_factory=list)
    plans_found: int = 0
    doors: int = 0
    windows: int = 0
    unlabeled_rooms: int = 0
    unmapped_texts: list[str] = Field(default_factory=list)
    fixtures: int = 0  # furniture and sanitary items kept
    skipped: list[str] = Field(default_factory=list)  # what the model cannot hold, with counts
    warnings: list[str] = Field(default_factory=list)


class ImportedPlan(BaseModel):
    index: int
    name: str
    model: BuildingModel
    room_count: int
    net_area: float


class ImportResult(BaseModel):
    plans: list[ImportedPlan]
    report: ImportReport


class ConvertReport(BaseModel):
    method: str  # "pdf-vector" | "raster-trace"
    scale_source: str  # how metres per source unit was decided
    metres_across: float  # the drawing's longest side, m
    walls: int = 0  # wall solids / outlines written
    openings: int = 0
    texts: int = 0
    skipped: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)


class PatchReport(BaseModel):
    applied: list[str] = Field(default_factory=list)  # changes written into existing entities
    added: list[str] = Field(default_factory=list)  # new entities drawn on FGA-ADDED (or copied from a template)
    skipped: list[str] = Field(default_factory=list)  # changes the file cannot take, with the reason


class ImportPlanOut(ImportedPlan):
    checks: list[CheckResult]
    ok: bool
    svg: str  # the plan as the engine draws it, for the review step


class ImportOut(BaseModel):
    plans: list[ImportPlanOut]
    report: ImportReport


class ConvertOut(BaseModel):
    dxf: str  # base64 of the DXF (metres)
    report: ConvertReport


class PatchOut(BaseModel):
    dxf: str  # base64 of the patched file
    report: PatchReport
