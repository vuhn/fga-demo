"""GrpcBlueprintEngine against an in-process Engine: one RPC per method, mapping, errors, request id."""

import pytest
from fake_engine import Running

from fga_agent.domain.model.call_context import CallContext
from fga_agent.domain.errors import EngineUnavailable, InvalidEdit, UnsupportedEdit
from fga_agent.gen.fga.demo.common.v1 import common_pb2 as c
from fga_agent.gen.fga.demo.engine.v1 import engine_pb2
from fga_agent.infrastructure.engine_grpc import blueprint_engine as be
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel

CTX = CallContext(request_id="rid-1")


@pytest.fixture
def eng():
    r = Running()
    r.gateway = GrpcBlueprintEngine(open_channel(r.addr), timeout=5)
    yield r
    r.stop()


def test_every_engine_rpc_is_used_or_deliberately_left_out():
    methods = {m.name for m in engine_pb2.DESCRIPTOR.services_by_name["EngineService"].methods}
    used = be.used_rpcs()
    assert methods == used | be.NOT_USED | be.LATER, f"decide where these go: {sorted(methods ^ (used | be.NOT_USED | be.LATER))}"
    assert not used & (be.NOT_USED | be.LATER)


def test_each_method_is_exactly_one_engine_rpc(eng):
    g = eng.gateway
    build = g.generate(CTX, b'{"a":1}', b"{}", True)
    assert build.ok and build.changed and build.attempts == 1 and build.issue_score == 2
    assert build.room_targets == {"r1": 12.5}
    assert build.checks[0].rule == "x" and build.checks[0].room_id == "r1" and not build.checks[0].ok
    assert g.apply_plan_ops(CTX, b"{}", b"[]").descriptions == ["moved"]
    assert g.run_checks(CTX, b"{}").ok
    assert g.apply_brief_ops(CTX, b"{}", b"[]").changed
    assert g.normalize_draft(CTX, b"{}").notes == ["n"]
    assert eng.engine.calls == ["Generate", "ApplyPlanOps", "RunChecks", "ApplyBriefOps", "NormalizeDraft"]


def test_the_catalog_calls_map_to_domain_types(eng):
    ops = eng.gateway.operations(CTX)
    assert ops.operations[0].name == "set_area" and ops.operations[0].schema == {} and ops.brief_ops_schema == {}
    limits = eng.gateway.design_limits(CTX)
    assert limits.min_area == {"bedroom": 9.0} and limits.rate_catalog[0].code == "R1"


def test_an_absent_room_id_is_none(eng):
    build = eng.gateway.generate(CTX, b"{}")
    assert build.checks[0].room_id == "r1"
    assert eng.gateway.run_checks(CTX, b"{}").checks == []


def test_refusals_and_outages_map_to_domain_errors(eng):
    eng.engine.refuse = (c.ERROR_CODE_EDIT_ERROR, "area too small")
    with pytest.raises(InvalidEdit, match="area too small") as ei:
        eng.gateway.generate(CTX, b"{}")
    assert not isinstance(ei.value, UnsupportedEdit)
    eng.engine.refuse = (c.ERROR_CODE_UNSUPPORTED, "no such op")
    with pytest.raises(UnsupportedEdit):
        eng.gateway.apply_brief_ops(CTX, b"{}", b"[]")
    eng.engine.refuse = None
    eng.engine.down = True
    with pytest.raises(EngineUnavailable, match="UNAVAILABLE"):
        eng.gateway.generate(CTX, b"{}")


def test_only_the_request_id_is_forwarded(eng):
    eng.gateway.run_checks(CTX, b"{}")
    eng.gateway.run_checks(CallContext(), b"{}")
    assert eng.engine.request_ids == ["rid-1"]  # none when the context has no id
