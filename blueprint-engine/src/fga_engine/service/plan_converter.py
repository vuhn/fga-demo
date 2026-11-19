from __future__ import annotations

import base64

from ..domain.edits import EditError
from ..domain.files import MAX_BYTES, ConvertOut
from ..domain.gateway import PlanConversion


class PlanConverterService:
    def __init__(self, conversion: PlanConversion):
        self._conversion = conversion

    def convert(self, data: bytes, mime: str, scale: int | None, dpi: float | None,
                longest_m: float | None) -> ConvertOut:
        """Zero or missing options mean 'not given'; `longest_m` beats `scale`."""
        if len(data) > MAX_BYTES:
            raise EditError("the file is larger than 20 MB")
        out, report = self._conversion.to_dxf(data, mime, scale or None, dpi or None, longest_m or None)
        return ConvertOut(dxf=base64.b64encode(out).decode("ascii"), report=report)
