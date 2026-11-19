"""Engine DTOs <-> domain for the edit operations: the ListTools request out, the catalog in (schema text becomes a dict)."""

from __future__ import annotations

import json

from ....domain.model.operation_catalog import EditOperation, OperationCatalog
from fga.demo.engine.v1 import engine_pb2 as e


def request() -> e.ListToolsRequest:
    return e.ListToolsRequest()


def to_domain(r: e.ListToolsResponse) -> OperationCatalog:
    return OperationCatalog(
        operations=[EditOperation(t.name, t.kind, t.description, json.loads(t.json_schema or "{}"), t.signature)
                    for t in r.tools],
        brief_ops_schema=json.loads(r.ops_json_schema or "{}"), plan_ops_schema=json.loads(r.plan_ops_json_schema or "{}"),
        draft_schema=json.loads(r.draft_json_schema or "{}"))
