from __future__ import annotations

import base64

from ..domain.edits import EditError
from ..domain.files import MAX_BYTES, PatchOut
from ..domain.gateway import DxfPatching
from ..domain.schemas import BuildingModel


class DxfPatcherService:
    def __init__(self, patching: DxfPatching):
        self._patching = patching

    def patch(self, original: bytes, base_json: str, current_json: str) -> PatchOut:
        if len(original) > MAX_BYTES:
            raise EditError("the original file is too large")
        try:
            base, current = BuildingModel.model_validate_json(base_json), BuildingModel.model_validate_json(current_json)
            out, report = self._patching.patch(original, base, current)
        except ValueError as e:  # includes pydantic's ValidationError
            raise EditError(str(e)) from e
        return PatchOut(dxf=base64.b64encode(out).decode("ascii"), report=report)
