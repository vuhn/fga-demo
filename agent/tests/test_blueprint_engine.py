"""GrpcBlueprintEngine against an in-process Engine: one RPC per method, mapping, errors, request id."""

import pytest
from fake_engine import Running

from fga_agent.domain.model.call_context import CallContext
from fga_agent.domain.model.estimate_facts import EstimateFacts
from fga_agent.domain.model.estimate_session import EstimateFact, RoomScope
from fga_agent.domain.errors import EngineUnavailable, InvalidEdit, UnsupportedEdit
from fga.demo.common.v1 import common_pb2 as c
from fga.demo.engine.v1 import engine_pb2
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
    facts = EstimateFacts(facts={"quality": EstimateFact("good", "user"), "floor_height": EstimateFact(3.0, "assumed"),
                                 "mep_in_scope": EstimateFact(False, "user"), "storeys": EstimateFact(2, "user")},
                          scope={"r1": RoomScope("existing", "user")}, fixtures_existing=["f1"], excluded=["window"])
    summary = g.summarize_plan(CTX, b"{}", facts)
    assert [r.name for r in summary.rooms] == ["Living"] and summary.rooms[0].area_m2 == 22.5
    assert (summary.fixtures[0].id, summary.fixtures[0].kind, summary.fixtures[0].room_id) == ("f1", "toilet", "r1")
    assert summary.openings[0].room_ids == ["r1", "outside"] and summary.openings[0].width == 0.9
    assert summary.pending_fixtures == ["f1"] and summary.structure_in_scope
    sent = {f.name: f for f in eng.engine.facts_seen.facts}
    assert sent["quality"].text == "good" and sent["floor_height"].number == 3.0 and sent["mep_in_scope"].flag is False
    assert sent["storeys"].number == 2.0 and sent["floor_height"].source == c.FACT_SOURCE_ASSUMED
    assert [(x.room_id, x.state) for x in eng.engine.facts_seen.scope] == [("r1", c.ROOM_STATE_EXISTING)]
    assert list(eng.engine.facts_seen.fixtures_existing) == ["f1"] and list(eng.engine.facts_seen.excluded) == ["window"]
    assert eng.engine.calls == ["Generate", "ApplyPlanOps", "RunChecks", "ApplyBriefOps", "NormalizeDraft", "SummarizePlan"]


def test_the_catalog_calls_map_to_domain_types(eng):
    ops = eng.gateway.operations(CTX)
    assert ops.operations[0].name == "set_area" and ops.operations[0].schema == {} and ops.brief_ops_schema == {}
    limits = eng.gateway.design_limits(CTX)
    assert limits.min_area == {"bedroom": 9.0} and limits.rate_catalog[0].code == "R1" and limits.rate_catalog[0].quality_sensitive


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
