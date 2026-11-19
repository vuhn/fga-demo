"""File DTOs <-> domain (ConvertToDXF, ImportDXF, PatchDXF, Render), both directions. A proto3 zero value means "not given"."""

from __future__ import annotations

import base64
import json

from ....domain.edits import EditError
from ....domain.files import ConvertOut, ImportOut, PatchOut, TitleBlock
from ....domain.schemas import BuildingModel
from fga.demo.engine.v1 import engine_pb2 as e
from ..common.json_helper import load_json

_FORMATS = {e.RENDER_FORMAT_SVG: "svg", e.RENDER_FORMAT_PDF: "pdf", e.RENDER_FORMAT_DXF: "dxf", e.RENDER_FORMAT_IFC: "ifc"}
_MEDIA = {"svg": "image/svg+xml", "dxf": "image/vnd.dxf", "pdf": "application/pdf", "ifc": "application/x-step"}


def convert_to_domain(request: e.ConvertToDXFRequest) -> tuple[bytes, str, int | None, float | None, float | None]:
    o = request.options
    return request.file.data, request.file.mime, o.scale or None, o.dpi or None, o.longest_m or None


def converted_to_pb(out: ConvertOut) -> e.ConvertToDXFResponse:
    return e.ConvertToDXFResponse(dxf=base64.b64decode(out.dxf), report_json=out.report.model_dump_json().encode())


def import_to_domain(request: e.ImportDXFRequest) -> tuple[bytes, str | None]:
    return request.file.data, request.unit or None


def imported_to_pb(out: ImportOut) -> e.ImportDXFResponse:
    plans = [p.model_dump(mode="json") for p in out.plans]
    return e.ImportDXFResponse(plans_json=json.dumps(plans).encode(), report_json=out.report.model_dump_json().encode())


def patch_to_domain(request: e.PatchDXFRequest) -> tuple[bytes, str, str]:
    return (request.original.data, request.base_model_json.decode("utf-8", "replace"),
            request.current_model_json.decode("utf-8", "replace"))


def patched_to_pb(out: PatchOut) -> e.PatchDXFResponse:
    return e.PatchDXFResponse(dxf=base64.b64decode(out.dxf), report_json=out.report.model_dump_json().encode())


def render_to_domain(request: e.RenderRequest) -> tuple[BuildingModel, str, TitleBlock]:
    fmt = _FORMATS.get(request.format)
    if fmt is None:
        raise EditError("unknown render format")
    t = request.title_block
    return (load_json(BuildingModel, request.model_json, "model_json"), fmt,
            TitleBlock(project=t.title or TitleBlock.project, version=t.version or 1, issued=t.issued, author=t.author))


def rendered_to_pb(data: bytes | str, fmt: str) -> e.RenderResponse:
    return e.RenderResponse(data=data.encode() if isinstance(data, str) else data, media_type=_MEDIA[fmt])
