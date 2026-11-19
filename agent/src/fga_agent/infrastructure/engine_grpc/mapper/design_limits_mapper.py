"""Engine DTOs <-> domain for the design limits: the GetCatalog request out, the limits in."""

from __future__ import annotations

from ....domain.model.design_limits import DesignLimits, RateSpec
from ....gen.fga.demo.engine.v1 import engine_pb2 as e


def request() -> e.GetCatalogRequest:
    return e.GetCatalogRequest()


def to_domain(r: e.GetCatalogResponse) -> DesignLimits:
    return DesignLimits(min_area=dict(r.min_area), min_width=dict(r.min_width), nestable_types=list(r.nestable_types),
                        max_nested=r.max_nested, default_fixtures=dict(r.default_fixtures),
                        fixture_kinds=list(r.fixture_kinds), default_markups=dict(r.default_markups),
                        priced_fixtures=list(r.priced_fixtures),
                        rate_catalog=[RateSpec(x.code, x.package, x.description, x.unit) for x in r.rate_catalog])
