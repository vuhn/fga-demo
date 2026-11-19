from __future__ import annotations

from ..domain.tools import Catalog, ToolList, get_catalog, list_tools


class ToolCatalogService:
    def list_tools(self) -> ToolList:
        return list_tools()

    def get_catalog(self) -> Catalog:
        return get_catalog()
