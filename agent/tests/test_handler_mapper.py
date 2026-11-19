"""The handler's DTO mappers: domain -> the Agent's protobuf messages."""

import grpc

from fga_agent.domain.errors import EngineUnavailable, InvalidEdit, LlmUnavailable, UnsupportedEdit
from fga_agent.domain.model.plan_check import PlanCheck
from fga_agent.domain.model.progress import Progress
from fga_agent.gen.fga.demo.common.v1 import common_pb2 as c
from fga_agent.handler.grpc.mapper import error_mapper, plan_check_mapper, progress_mapper


def test_plan_check_to_pb_turns_an_absent_room_into_an_empty_string():
    pb = plan_check_mapper.to_pb(PlanCheck("r", "error", False, "m", None, "geometry", "r:x", ["a"], "h"))
    assert (pb.rule, pb.room_id, pb.category, list(pb.elements), pb.hint) == ("r", "", "geometry", ["a"], "h")
    assert plan_check_mapper.to_pb(PlanCheck("r", "warning", True, "m", "room1")).room_id == "room1"


def test_plan_check_survives_a_round_trip_through_the_dto():
    for room in ("room1", None):
        check = PlanCheck("r", "error", False, "m", room, "geometry", "r:x", ["a", "b"], "h")
        assert plan_check_mapper.to_domain(plan_check_mapper.to_pb(check)) == check


def test_progress_survives_a_round_trip_and_becomes_a_blueprint_event():
    for p in (Progress("step", "layout", "laying out"), Progress("thinking", "thinking", "hm")):
        assert progress_mapper.to_domain(progress_mapper.to_pb(p)) == p
    ev = progress_mapper.to_blueprint_event(Progress("thinking", "thinking", "hm"))
    assert ev.WhichOneof("event") == "progress" and ev.progress.kind == "thinking" and ev.progress.message == "hm"


def test_domain_errors_map_to_a_status_with_detail():
    status, d = error_mapper.to_status(InvalidEdit("bad"))
    assert status == grpc.StatusCode.INVALID_ARGUMENT and d.code == c.ERROR_CODE_EDIT_ERROR
    assert error_mapper.to_status(UnsupportedEdit("no"))[1].code == c.ERROR_CODE_UNSUPPORTED
    status, d = error_mapper.to_status(EngineUnavailable("down"))
    assert status == grpc.StatusCode.UNAVAILABLE and d.dependency == "engine"
    assert error_mapper.to_status(LlmUnavailable("down"))[1].dependency == "llm"
    assert error_mapper.to_status(RuntimeError("x")) is None


def test_domain_errors_map_to_in_band_events_without_leaking_internals():
    assert (error_mapper.to_error(InvalidEdit("bad")).status, error_mapper.to_error(InvalidEdit("bad")).error) == (422, "invalid_edit")
    assert error_mapper.to_error(UnsupportedEdit("no")).error == "invalid_edit"
    assert error_mapper.to_error(EngineUnavailable("d")).error == "engine_unavailable"
    assert error_mapper.to_error(LlmUnavailable("d")).status == 502
    internal = error_mapper.to_error(RuntimeError("secret path /etc/x"))
    assert (internal.status, internal.error, internal.message) == (500, "internal", "internal error")
    assert error_mapper.to_blueprint_error_event(InvalidEdit("bad")).error.message == "bad"
