"""Tool list and catalog, loaded from the Engine on first use and cached for the life of the process.
A failed load is not cached: the next call tries again."""

from __future__ import annotations

import threading

from ...domain.gateway import PlanTools
from ...domain.tools import Catalog, ToolList


class CatalogLoader:
    def __init__(self, tools: PlanTools) -> None:
        self._tools = tools
        self._lock = threading.Lock()
        self._list: ToolList | None = None
        self._catalog: Catalog | None = None

    def ready(self) -> bool:
        return self._list is not None and self._catalog is not None

    def load(self) -> None:
        with self._lock:
            if self._list is None:
                self._list = self._tools.list_tools()
            if self._catalog is None:
                self._catalog = self._tools.get_catalog()

    def tools(self) -> ToolList:
        self.load()
        assert self._list is not None
        return self._list

    def catalog(self) -> Catalog:
        self.load()
        assert self._catalog is not None
        return self._catalog
