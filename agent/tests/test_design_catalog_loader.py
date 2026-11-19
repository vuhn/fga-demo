"""DesignCatalogLoaderService is a service: tested against a fake BlueprintEngine gateway, no gRPC."""

import pytest

from fga_agent.domain.call_context import CallContext
from fga_agent.domain.design_limits import DesignLimits
from fga_agent.domain.errors import EngineUnavailable
from fga_agent.domain.operation_catalog import OperationCatalog
from fga_agent.service.design_catalog_loader import DesignCatalogLoaderService

CTX = CallContext()


class FakeEngine:
    def __init__(self):
        self.down = False
        self.calls = []

    def operations(self, ctx):
        self.calls.append("operations")
        if self.down:
            raise EngineUnavailable("down")
        return OperationCatalog([], {}, {}, {})

    def design_limits(self, ctx):
        self.calls.append("design_limits")
        if self.down:
            raise EngineUnavailable("down")
        return DesignLimits({}, {}, [], 2, {}, [], {}, [], [])


def test_loads_lazily_caches_and_retries_after_a_failure():
    engine = FakeEngine()
    loader = DesignCatalogLoaderService(engine)
    engine.down = True
    assert not loader.ready()
    with pytest.raises(EngineUnavailable):
        loader.operations(CTX)
    assert not loader.ready()
    engine.down = False
    assert loader.limits(CTX).max_nested == 2
    assert loader.ready()
    n = len(engine.calls)
    loader.operations(CTX), loader.limits(CTX)
    assert len(engine.calls) == n  # cached: no further Engine calls
