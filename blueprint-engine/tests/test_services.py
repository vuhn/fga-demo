"""Unit tests for the brief and plan services (T4a): no HTTP, drawing behind a fake gateway where it can be."""

import pytest

from fga_engine.domain.edits import EditError
from fga_engine.domain.plan_edits import PlanOps, PlanRenameRoom
from fga_engine.domain.schemas import Brief
from fga_engine.pipeline import generate
from fga_engine.service.checks_runner import ChecksRunnerService
from fga_engine.service.generator import GeneratorService
from fga_engine.service.plan_creator import PlanCreatorService
from fga_engine.service.plan_drawer import PlanDrawerService
from fga_engine.service.plan_ops_applier import PlanOpsApplierService
from fga_engine.wire import build_engine

from briefs import FAMILY_3BR, TOO_BIG


@pytest.fixture(scope="module")
def imported():
    eng = build_engine()
    return eng.plan_drawer.draw_plan(generate(FAMILY_3BR).model, "House").result.model


def test_generator_without_repair_reports_a_brief_that_does_not_fit():
    step = GeneratorService().generate(TOO_BIG)
    assert step.result.model is None and step.result.issues and step.attempts == 1 and step.notes == []


def test_generator_repair_shrinks_rooms_to_fit():
    brief = FAMILY_3BR.model_copy(update={"lot": FAMILY_3BR.lot.model_copy(update={"width": 11, "depth": 16})})
    plain = GeneratorService().generate(brief)
    fitted = GeneratorService().generate(brief, repair=True)
    assert plain.result.model is None
    assert fitted.result.model is not None and fitted.attempts > 1
    assert any(n.startswith("shrank rooms") for n in fitted.notes)


def test_checks_runner_uses_the_brief_only_when_given():
    res = generate(FAMILY_3BR)
    svc = ChecksRunnerService()
    assert [c.rule for c in svc.run_checks(res.model, FAMILY_3BR)] == [c.rule for c in res.checks]
    imported_rules = {c.rule for c in svc.run_checks(res.model)}
    assert imported_rules != {c.rule for c in res.checks}


class FakeDrawing:
    def __init__(self, plans):
        self.plans, self.drawn = plans, []

    def draw(self, model, title):
        self.drawn.append(title)
        return b"  0\nEOF\n"

    def read(self, data):
        return self.plans


def test_plan_drawer_rejects_a_plan_that_is_already_cad(imported):
    svc = PlanDrawerService(FakeDrawing([imported]), ChecksRunnerService())
    with pytest.raises(EditError, match="already a CAD plan"):
        svc.draw_plan(imported, "x")


def test_plan_drawer_needs_exactly_one_plan_back():
    gen = generate(FAMILY_3BR).model
    svc = PlanDrawerService(FakeDrawing([]), ChecksRunnerService())
    with pytest.raises(EditError, match="one plan"):
        svc.draw_plan(gen, "x")


def test_plan_drawer_refuses_a_drawing_that_loses_area(imported):
    gen = generate(FAMILY_3BR).model
    smaller = imported.model_copy(update={"rooms": imported.rooms[:-1]})
    fake = FakeDrawing([smaller])
    with pytest.raises(EditError, match="cannot be converted"):
        PlanDrawerService(fake, ChecksRunnerService()).draw_plan(gen, "x")
    assert fake.drawn == ["x"]


def test_plan_drawer_titles_an_untitled_plan_and_returns_the_dxf(imported):
    gen = generate(FAMILY_3BR).model
    fake = FakeDrawing([imported])
    out = PlanDrawerService(fake, ChecksRunnerService()).draw_plan(gen, "")
    assert fake.drawn == ["Floor plan"] and out.dxf and out.result.model is imported


def test_plan_creator_returns_issues_when_the_brief_cannot_be_built():
    class NeverDraws:
        def draw_plan(self, *a):
            raise AssertionError("must not draw")

    out = PlanCreatorService(GeneratorService(), NeverDraws()).create_plan(TOO_BIG)
    assert out.result.model is None and out.result.issues and out.dxf is None


def test_plan_creator_draws_the_fitted_layout():
    out = build_engine().plan_creator.create_plan(FAMILY_3BR)
    assert out.result.model.source == "imported" and out.dxf and out.result.ok in (True, False)


def test_plan_ops_applier_edits_and_checks(imported):
    room = imported.rooms[0]
    ops = PlanOps(ops=[PlanRenameRoom(op="rename_room", room=room.id, name="Den")])
    step = PlanOpsApplierService(ChecksRunnerService()).apply_plan_ops(imported, ops)
    assert step.result.model.rooms[0].name == "Den"
    assert step.notes[0].startswith("edited: ") and step.llm_attempts == 0


class FakeRendering:
    def __init__(self):
        self.calls = []

    def render(self, model, fmt, title_block):
        self.calls.append((fmt, title_block.project))
        return "<svg/>"


def test_dxf_importer_checks_and_draws_each_plan_through_the_gateways(imported):
    from fga_engine.domain.files import ImportedPlan, ImportReport, ImportResult
    from fga_engine.service.dxf_importer import DxfImporterService

    class Reading:
        def read_plans(self, data, unit):
            plan = ImportedPlan(index=0, name="Flat", model=imported, room_count=len(imported.rooms), net_area=1.0)
            return ImportResult(plans=[plan], report=ImportReport(units="m", unit_source="test"))

    fake = FakeRendering()
    out = DxfImporterService(Reading(), fake).import_dxf(b"x", None)
    assert fake.calls == [("svg", "Flat")] and out.plans[0].svg == "<svg/>" and out.plans[0].checks and out.report.units == "m"


def test_plan_converter_rejects_a_file_over_20_mb_and_treats_zero_as_not_given():
    from fga_engine.domain.files import MAX_BYTES, ConvertReport
    from fga_engine.service.plan_converter import PlanConverterService

    class Conversion:
        def to_dxf(self, data, mime, scale, dpi, longest_m):
            self.args = (scale, dpi, longest_m)
            return b"  0\nEOF\n", ConvertReport(method="m", scale_source="s", metres_across=1.0)

    conv = Conversion()
    svc = PlanConverterService(conv)
    with pytest.raises(EditError):
        svc.convert(b"x" * (MAX_BYTES + 1), "image/png", 0, 0, 0)
    out = svc.convert(b"x", "image/png", 0, 0.0, 12.5)
    assert conv.args == (None, None, 12.5) and out.dxf


def test_dxf_patcher_turns_bad_json_into_an_edit_error():
    from fga_engine.service.dxf_patcher import DxfPatcherService

    class Patching:
        def patch(self, *a):
            raise AssertionError("must not patch")

    with pytest.raises(EditError):
        DxfPatcherService(Patching()).patch(b"x", "not json", "{}")
