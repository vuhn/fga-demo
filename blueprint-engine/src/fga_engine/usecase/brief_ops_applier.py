from __future__ import annotations

from typing import Protocol

from ..domain.results import BriefResult
from ..domain.schemas import Brief, BuildingModel, EditOps


class BriefOpsApplier(Protocol):
    def apply_brief_ops(self, brief: Brief, ops: EditOps, previous: BuildingModel | None = None) -> BriefResult:
        """Edit ops -> new brief -> layout (rooms shrink toward their minimums when it cannot fit) -> checks.
        Raises EditError, or UnsupportedEdit for an `unsupported` op."""
