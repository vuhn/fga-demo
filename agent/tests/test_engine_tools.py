"""The tool registry and the catalog loader against an in-process Engine."""

import pytest
from fake_engine import Running

from fga_agent.domain import request
from fga_agent.domain.errors import EditError, EngineUnavailable, UnsupportedEdit
from fga_agent.gen.fga.demo.common.v1 import common_pb2 as c
from fga_agent.gen.fga.demo.engine.v1 import engine_pb2
from fga_agent.infrastructure.engine_grpc import registry
from fga_agent.infrastructure.engine_grpc.catalog import CatalogLoader
from fga_agent.infrastructure.engine_grpc.client import EngineTools, open_channel
from fga_agent.infrastructure.engine_grpc.registry import Entry


@pytest.fixture
def eng():
    r = Running()
    r.tools = EngineTools(open_channel(r.addr), timeout=5)
    yield r
    r.stop()


def test_every_engine_rpc_is_a_tool_a_catalog_call_or_deliberately_left_out():
    methods = {m.name for m in engine_pb2.DESCRIPTOR.services_by_name["EngineService"].methods}
    tools = {x.rpc for x in registry.ENTRIES}
    covered = tools | registry.CATALOG | registry.NOT_TOOLS | registry.LATER
    assert methods == covered, f"decide where these go: {sorted(methods ^ covered)}"
    assert not tools & (registry.CATALOG | registry.NOT_TOOLS | registry.LATER)
    assert len({x.name for x in registry.ENTRIES}) == len(registry.ENTRIES)


def test_a_tool_call_is_exactly_one_engine_rpc(eng):
    out = eng.tools.call("generate", {"brief": b'{"a":1}', "previous_model": b"{}", "repair": True})
    assert eng.engine.calls == ["Generate"]
    assert out.ok and out.changed and out.attempts == 1 and out.issue_score == 2
    assert out.room_targets == {"r1": 12.5}
    assert out.checks[0].rule == "x" and out.checks[0].room_id == "r1" and not out.checks[0].ok
    eng.tools.call("apply_plan_ops", {"model": b"{}", "ops": b"[]"})
    eng.tools.call("run_checks", {"model": b"{}"})
    eng.tools.call("apply_brief_ops", {"brief": b"{}", "ops": b"[]"})
    eng.tools.call("normalize_draft", {"draft": b"{}"})
    assert eng.engine.calls == ["Generate", "ApplyPlanOps", "RunChecks", "ApplyBriefOps", "NormalizeDraft"]


def test_a_new_tool_needs_one_registry_entry(eng):
    """A throwaway tool: one entry over an existing RPC is all it takes."""
    extra = Entry("regenerate", "Generate", lambda a: engine_pb2.GenerateRequest(brief_json=a["brief"]), lambda r: r.ok)
    tools = EngineTools(open_channel(eng.addr), entries=registry.ENTRIES + (extra,))
    assert "regenerate" in tools.names()
    assert tools.call("regenerate", {"brief": b"{}"}) is True
    with pytest.raises(EngineUnavailable):
        eng.tools.call("regenerate", {"brief": b"{}"})  # not registered on the real registry


def test_refusals_and_outages_map_to_domain_errors(eng):
    eng.engine.refuse = (c.ERROR_CODE_EDIT_ERROR, "area too small")
    with pytest.raises(EditError, match="area too small") as ei:
        eng.tools.call("generate", {"brief": b"{}"})
    assert not isinstance(ei.value, UnsupportedEdit)
    eng.engine.refuse = (c.ERROR_CODE_UNSUPPORTED, "no such op")
    with pytest.raises(UnsupportedEdit):
        eng.tools.call("apply_brief_ops", {"brief": b"{}", "ops": b"[]"})
    eng.engine.refuse = None
    eng.engine.down = True
    with pytest.raises(EngineUnavailable, match="UNAVAILABLE"):
        eng.tools.call("generate", {"brief": b"{}"})


def test_the_request_id_is_forwarded_and_nothing_else(eng):
    with request.bind("abc123"):
        eng.tools.call("run_checks", {"model": b"{}"})
    eng.tools.call("run_checks", {"model": b"{}"})
    assert eng.engine.request_ids == ["abc123"]  # none when the call has no id


def test_catalog_loads_lazily_caches_and_retries_after_a_failure(eng):
    loader = CatalogLoader(eng.tools)
    eng.engine.down = True
    assert not loader.ready()
    with pytest.raises(EngineUnavailable):
        loader.tools()
    assert not loader.ready()
    eng.engine.down = False
    assert loader.tools().tools[0].name == "set_area"
    assert loader.catalog().min_area == {"bedroom": 9.0} and loader.catalog().rate_catalog[0].code == "R1"
    assert loader.ready()
    n = len(eng.engine.calls)
    loader.tools(), loader.catalog()
    assert len(eng.engine.calls) == n  # cached: no further Engine calls
