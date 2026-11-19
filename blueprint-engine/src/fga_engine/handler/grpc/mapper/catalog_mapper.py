"""Tool list and catalog DTOs <-> domain (ListTools, GetCatalog), domain -> DTO (the requests carry nothing)."""

from __future__ import annotations

import json

from ....domain.tools import Catalog, ToolList
from fga.demo.engine.v1 import engine_pb2 as e


def tools_to_pb(tl: ToolList) -> e.ListToolsResponse:
    return e.ListToolsResponse(
        tools=[e.Tool(name=t.name, kind=t.kind, description=t.description, json_schema=json.dumps(t.json_schema),
                      signature=t.signature) for t in tl.tools],
        ops_json_schema=json.dumps(tl.ops_json_schema), plan_ops_json_schema=json.dumps(tl.plan_ops_json_schema),
        draft_json_schema=json.dumps(tl.draft_json_schema))


def catalog_to_pb(cat: Catalog) -> e.GetCatalogResponse:
    return e.GetCatalogResponse(
        min_area=cat.min_area, min_width=cat.min_width, nestable_types=cat.nestable_types, max_nested=cat.max_nested,
        default_fixtures=cat.default_fixtures, fixture_kinds=cat.fixture_kinds, default_markups=cat.default_markups,
        priced_fixtures=cat.priced_fixtures,
        rate_catalog=[e.RateSpec(code=r.code, package=r.package, description=r.description, unit=r.unit,
                                  quality_sensitive=r.quality_sensitive)
                      for r in cat.rate_catalog])
