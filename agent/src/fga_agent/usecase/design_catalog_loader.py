from __future__ import annotations

from typing import Protocol

from ..domain.call_context import CallContext
from ..domain.design_limits import DesignLimits
from ..domain.operation_catalog import OperationCatalog


class DesignCatalogLoader(Protocol):
    """The Engine's edit operations and design limits, loaded on first use and kept for the life of the process."""

    def ready(self) -> bool:
        """True once both are loaded."""

    def load(self, ctx: CallContext) -> None:
        """Load whatever is missing. EngineUnavailable while the Engine cannot be reached; the next call tries again."""

    def operations(self, ctx: CallContext) -> OperationCatalog: ...

    def limits(self, ctx: CallContext) -> DesignLimits: ...
