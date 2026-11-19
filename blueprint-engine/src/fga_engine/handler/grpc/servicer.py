"""EngineService for the catalog, brief, plan and estimate RPCs. Handlers decode, call one usecase, encode."""

from __future__ import annotations

import base64
import json

from ...domain import plan_edits
from ...domain.checks import failed
from ...domain.draft import DraftBrief
from ...domain.edits import EditError
from ...domain.files import TitleBlock
from ...domain.results import PlanCreated, PlanSummary
from ...domain.schemas import Brief, BuildingModel, EditOps
from ...gen.fga.demo.engine.v1 import engine_pb2 as e
from ...gen.fga.demo.engine.v1 import engine_pb2_grpc as rpc
from ...wire import Engine
from . import mapping as m
from .errors import handled


_FORMATS = {e.RENDER_FORMAT_SVG: "svg", e.RENDER_FORMAT_PDF: "pdf", e.RENDER_FORMAT_DXF: "dxf", e.RENDER_FORMAT_IFC: "ifc"}
_MEDIA = {"svg": "image/svg+xml", "dxf": "image/vnd.dxf", "pdf": "application/pdf", "ifc": "application/x-step"}


def _opt(model_cls, data: bytes, what: str):
    return m.load_json(model_cls, data, what) if data else None


def _outcome(created: PlanCreated) -> e.PlanOutcome:
    r = created.result
    return e.PlanOutcome(model_json=m.dump_json(r.model) if r.model is not None else b"", checks=m.checks_to_pb(r.checks),
                         issues=r.issues, ok=r.ok, notes=created.notes or r.notes,
                         dxf=base64.b64decode(created.dxf) if created.dxf else b"")


class EngineServicer(rpc.EngineServiceServicer):
    def __init__(self, engine: Engine):
        self._e = engine

    # --- catalog ---------------------------------------------------------------------------------------------
    @handled
    def ListTools(self, request, context):
        tl = self._e.tool_catalog.list_tools()
        return e.ListToolsResponse(
            tools=[e.Tool(name=t.name, kind=t.kind, description=t.description, json_schema=json.dumps(t.json_schema),
                          signature=t.signature) for t in tl.tools],
            ops_json_schema=json.dumps(tl.ops_json_schema), plan_ops_json_schema=json.dumps(tl.plan_ops_json_schema),
            draft_json_schema=json.dumps(tl.draft_json_schema))

    @handled
    def GetCatalog(self, request, context):
        cat = self._e.tool_catalog.get_catalog()
        return e.GetCatalogResponse(
            min_area=cat.min_area, min_width=cat.min_width, nestable_types=cat.nestable_types, max_nested=cat.max_nested,
            default_fixtures=cat.default_fixtures, fixture_kinds=cat.fixture_kinds, default_markups=cat.default_markups,
            priced_fixtures=cat.priced_fixtures,
            rate_catalog=[e.RateSpec(code=r.code, package=r.package, description=r.description, unit=r.unit)
                          for r in cat.rate_catalog])

    # --- brief and plan operations ---------------------------------------------------------------------------
    @handled
    def NormalizeDraft(self, request, context):
        brief, notes = self._e.draft_normalizer.normalize_draft(m.load_json(DraftBrief, request.draft_json, "draft_json"))
        return e.NormalizeDraftResponse(brief_json=m.dump_json(brief), notes=notes)

    @handled
    def Generate(self, request, context):
        brief = m.load_json(Brief, request.brief_json, "brief_json")
        prev = _opt(BuildingModel, request.previous_model_json, "previous_model_json")
        return m.brief_result_to_pb(self._e.generator.generate_brief(brief, prev, repair=request.repair))

    @handled
    def ApplyBriefOps(self, request, context):
        brief = m.load_json(Brief, request.brief_json, "brief_json")
        ops = m.load_json(EditOps, request.ops_json, "ops_json")
        prev = _opt(BuildingModel, request.previous_model_json, "previous_model_json")
        return m.brief_result_to_pb(self._e.brief_ops_applier.apply_brief_ops(brief, ops, prev))

    @handled
    def CreatePlan(self, request, context):
        return _outcome(self._e.plan_creator.create_plan(m.load_json(Brief, request.brief_json, "brief_json")))

    @handled
    def DrawPlan(self, request, context):
        return _outcome(self._e.plan_drawer.draw_plan(m.load_json(BuildingModel, request.model_json, "model_json"),
                                                      request.title))

    @handled
    def ApplyPlanOps(self, request, context):
        model = m.load_json(BuildingModel, request.model_json, "model_json")
        ops = m.load_json(plan_edits.PlanOps, request.ops_json, "ops_json")
        step = self._e.plan_ops_applier.apply_plan_ops(model, ops)
        new = step.result.model
        return e.PlanStep(model_json=m.dump_json(new), checks=m.checks_to_pb(step.result.checks), ok=step.result.ok,
                          notes=step.notes, summary=plan_edits.summary(new),
                          descriptions=[plan_edits.describe(o) for o in ops.ops], changed=new != model)

    @handled
    def RunChecks(self, request, context):
        model = m.load_json(BuildingModel, request.model_json, "model_json")
        brief = _opt(Brief, request.brief_json, "brief_json")
        checks = self._e.checks_runner.run_checks(model, brief)
        score = 10 * len(failed(checks)) + len(failed(checks, "warning"))
        return e.RunChecksResponse(checks=m.checks_to_pb(checks), ok=not failed(checks), issue_score=score)

    @handled
    def SummarizePlan(self, request, context):
        model = m.load_json(BuildingModel, request.model_json, "model_json")
        s: PlanSummary = self._e.plan_summarizer.summarize_plan(model, m.facts_from_pb(request.facts))
        return m.summary_to_pb(s)

    # --- estimate --------------------------------------------------------------------------------------------
    @handled
    def Takeoff(self, request, context):
        model = m.load_json(BuildingModel, request.model_json, "model_json")
        return m.takeoff_to_pb(self._e.takeoff.takeoff(model, m.facts_from_pb(request.facts)))

    @handled
    def PriceEstimate(self, request, context):
        rates = {r.code: m.rate_from_pb(r) for r in request.rates}
        est = self._e.estimate_pricer.price_estimate(m.takeoff_from_pb(request.takeoff), rates,
                                                     m.facts_from_pb(request.facts))
        return m.estimate_to_pb(est)

    # --- files -----------------------------------------------------------------------------------------------
    @handled
    def ConvertToDXF(self, request, context):
        o = request.options
        out = self._e.plan_converter.convert(request.file.data, request.file.mime, o.scale or None, o.dpi or None,
                                             o.longest_m or None)
        return e.ConvertToDXFResponse(dxf=base64.b64decode(out.dxf), report_json=out.report.model_dump_json().encode())

    @handled
    def ImportDXF(self, request, context):
        out = self._e.dxf_importer.import_dxf(request.file.data, request.unit or None)
        plans = [p.model_dump(mode="json") for p in out.plans]
        return e.ImportDXFResponse(plans_json=json.dumps(plans).encode(), report_json=out.report.model_dump_json().encode())

    @handled
    def PatchDXF(self, request, context):
        out = self._e.dxf_patcher.patch(request.original.data, request.base_model_json.decode("utf-8", "replace"),
                                        request.current_model_json.decode("utf-8", "replace"))
        return e.PatchDXFResponse(dxf=base64.b64decode(out.dxf), report_json=out.report.model_dump_json().encode())

    @handled
    def Render(self, request, context):
        fmt = _FORMATS.get(request.format)
        if fmt is None:
            raise EditError("unknown render format")
        model = m.load_json(BuildingModel, request.model_json, "model_json")
        t = request.title_block
        tb = TitleBlock(project=t.title or TitleBlock.project, version=t.version or 1, issued=t.issued, author=t.author)
        data = self._e.plan_renderer.render(model, fmt, tb)
        return e.RenderResponse(data=data.encode() if isinstance(data, str) else data, media_type=_MEDIA[fmt])
