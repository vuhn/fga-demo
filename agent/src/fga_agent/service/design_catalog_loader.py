from __future__ import annotations

import threading

from ..domain.model.call_context import CallContext
from ..domain.model.design_limits import DesignLimits
from ..domain.gateway import BlueprintEngine
from ..domain.model.operation_catalog import OperationCatalog


class DesignCatalogLoaderService:
    """Asks the Engine once and caches the answer; a failed load is not cached."""

    def __init__(self, engine: BlueprintEngine) -> None:
        self._engine = engine
        self._lock = threading.Lock()
        self._operations: OperationCatalog | None = None
        self._limits: DesignLimits | None = None

    def ready(self) -> bool:
        return self._operations is not None and self._limits is not None

    def load(self, ctx: CallContext) -> None:
        with self._lock:
            if self._operations is None:
                self._operations = self._engine.operations(ctx)
            if self._limits is None:
                self._limits = self._engine.design_limits(ctx)

    def operations(self, ctx: CallContext) -> OperationCatalog:
        self.load(ctx)
        assert self._operations is not None
        return self._operations

    def limits(self, ctx: CallContext) -> DesignLimits:
        self.load(ctx)
        assert self._limits is not None
        return self._limits
