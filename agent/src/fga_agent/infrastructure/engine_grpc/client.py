"""gRPC client of the Engine: PlanTools (tool list, catalog) and the ToolRegistry."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

import grpc

from ...domain import request
from ...domain.errors import EngineUnavailable
from ...domain.tools import Catalog, ToolList
from ...gen.fga.demo.engine.v1 import engine_pb2 as e
from ...gen.fga.demo.engine.v1 import engine_pb2_grpc as rpc
from . import mapping as m
from .errors import map_error
from .registry import ENTRIES, Entry

MAX_MESSAGE = 32 << 20
REQUEST_ID = "x-request-id"


def open_channel(addr: str) -> grpc.Channel:
    return grpc.insecure_channel(addr, options=[("grpc.max_receive_message_length", MAX_MESSAGE),
                                                ("grpc.max_send_message_length", MAX_MESSAGE)])


class EngineTools:
    """Implements gateway.PlanTools and gateway.ToolRegistry. Every method is one Engine RPC with a deadline."""

    def __init__(self, channel: grpc.Channel, timeout: float = 120.0, entries: tuple[Entry, ...] = ENTRIES) -> None:
        self._stub = rpc.EngineServiceStub(channel)
        self._timeout = timeout
        self._entries = {x.name: x for x in entries}

    def _call(self, rpc_name: str, req: Any) -> Any:
        rid = request.current()
        md = ((REQUEST_ID, rid),) if rid else None
        try:
            return getattr(self._stub, rpc_name)(req, timeout=self._timeout, metadata=md)
        except grpc.RpcError as err:
            raise map_error(rpc_name, err) from err

    # --- PlanTools -----------------------------------------------------------------------------------------
    def list_tools(self) -> ToolList:
        return m.tool_list_from_pb(self._call("ListTools", e.ListToolsRequest()))

    def get_catalog(self) -> Catalog:
        return m.catalog_from_pb(self._call("GetCatalog", e.GetCatalogRequest()))

    # --- ToolRegistry --------------------------------------------------------------------------------------
    def names(self) -> list[str]:
        return sorted(self._entries)

    def call(self, name: str, args: Mapping[str, Any]) -> Any:
        entry = self._entries.get(name)
        if entry is None:
            raise EngineUnavailable(f"unknown tool {name!r}")
        return entry.response(self._call(entry.rpc, entry.request(args)))
