"""Catalog endpoints (ListTools, GetCatalog). Decode, call one use case, encode through mapper/."""

from __future__ import annotations

from dataclasses import dataclass

from ...usecase.tool_catalog import ToolCatalog
from .common.error_helper import handled
from .mapper import catalog_mapper


@dataclass(frozen=True)
class CatalogUsecases:
    tool_catalog: ToolCatalog


class CatalogHandler:
    def __init__(self, usecases: CatalogUsecases) -> None:
        self._uc = usecases

    @handled
    def ListTools(self, request, context):
        return catalog_mapper.tools_to_pb(self._uc.tool_catalog.list_tools())

    @handled
    def GetCatalog(self, request, context):
        return catalog_mapper.catalog_to_pb(self._uc.tool_catalog.get_catalog())
