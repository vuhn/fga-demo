from __future__ import annotations

from typing import Protocol

from ..domain.tools import Catalog, ToolList


class ToolCatalog(Protocol):
    def list_tools(self) -> ToolList:
        """Every operation the LLM may produce, with its JSON Schema and prompt signature."""

    def get_catalog(self) -> Catalog:
        """The limits and defaults (minimum sizes, fixtures, rate codes) the LLM prompts quote."""
