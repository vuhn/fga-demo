"""Interfaces the services need from the outside world; infrastructure implements them."""

from __future__ import annotations

from collections.abc import Iterator, Mapping
from typing import Any, Protocol

from ..tools import Catalog, ToolList


class LLM(Protocol):
    """The language model. Output is checked by the Engine's tools (NormalizeDraft, ApplyBriefOps), so the model
    returns plain JSON for the schema it is given; invalid JSON is retried once with the parse error."""

    def structured(self, system: str, user: str, schema: dict, max_tokens: int | None = None) -> dict:
        """A JSON object for the JSON Schema; LLMError when the model cannot be reached or answers invalid JSON twice."""

    def stream_text(self, system: str, user: str) -> Iterator[str]:
        """A plain-text reply, yielded as it is generated."""


class ToolRegistry(Protocol):
    """The typed tools the LLM loops may call. One entry is a name, one Engine RPC and the mapping of its
    request and response; a call is exactly one Engine RPC."""

    def names(self) -> list[str]: ...

    def call(self, name: str, args: Mapping[str, Any]) -> Any:
        """Run one tool. EditError / UnsupportedEdit when the Engine refuses; EngineUnavailable when it cannot serve."""


class PlanTools(Protocol):
    """What the Engine publishes about itself."""

    def list_tools(self) -> ToolList: ...

    def get_catalog(self) -> Catalog: ...


class CatalogSource(Protocol):
    """The tool list and catalog, loaded from the Engine on first use and cached."""

    def ready(self) -> bool:
        """True once both are loaded."""

    def tools(self) -> ToolList:
        """EngineUnavailable while the Engine cannot be reached; the next call tries again."""

    def catalog(self) -> Catalog: ...
