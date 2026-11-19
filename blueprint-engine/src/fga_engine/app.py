"""Stateless HTTP engine. No auth and no storage: only the Go API calls it, after its own Checks.

Run: uv run uvicorn fga_engine.app:app --port 8090
"""

from __future__ import annotations

import base64
import json
import queue
import threading
from pathlib import Path
from typing import Literal

from fastapi import FastAPI, File, Form, Request, UploadFile
from fastapi.responses import FileResponse, JSONResponse, Response, StreamingResponse
from pydantic import BaseModel, Field

from . import convert, dxf_patch, dxf_plan, estimate_agent, llm_steps, pipeline, progress
from .domain import plan_edits
from .domain.checks import failed, run_imported_checks
from .draw import TitleBlock, build_dxf, to_dxf_bytes, to_pdf, to_svg
from .domain.edits import EditError
from .ifc import to_ifc
from .estimate_models import Message, SessionState
from .llm import LLMClient, LLMError, OllamaClient
from .price_research import LLMWebPriceSource
from .price_source import PriceSource
from .domain.schemas import Brief, BuildingModel, CheckResult

STATIC = Path(__file__).parent / "static"


class TextIn(BaseModel):
    prompt: str
    enhance: bool = False  # opt-in: on qwen3 8B the Enhancer adds rooms and lowers the eval (13/15 -> 8/15)


class BriefOut(BaseModel):
    brief: Brief
    notes: list[str]


class GenerateIn(BaseModel):
    brief: Brief
    previous: BuildingModel | None = None
    repair: bool = False


class ReviseIn(BaseModel):
    brief: Brief
    previous: BuildingModel | None = None
    message: str
    history: list[str] = Field(default_factory=list, max_length=20)  # earlier chat messages, oldest first


class PlanReviseIn(BaseModel):
    model: BuildingModel
    message: str
    history: list[str] = Field(default_factory=list, max_length=20)


class ReviewIn(BaseModel):
    brief: Brief
    previous: BuildingModel | None = None
    history: list[str] = Field(default_factory=list, max_length=20)
    max_rounds: int = Field(default=3, ge=1, le=3)


class TitleBlockIn(BaseModel):
    project: str = "Single-storey house"
    version: int = 1
    issued: str = ""
    author: str = ""


class RenderIn(BaseModel):
    model: BuildingModel
    format: Literal["svg", "dxf", "pdf", "ifc"] = "svg"
    title_block: TitleBlockIn = TitleBlockIn()


class EstimateTurnIn(BaseModel):
    model: BuildingModel
    state: SessionState = Field(default_factory=SessionState)
    message: str = Field(default="", max_length=2000)
    messages: list[Message] = Field(default_factory=list, max_length=12)  # recent chat, oldest first
    rate_overrides: dict[str, float] = Field(default_factory=dict)


class PlanDrawIn(BaseModel):
    model: BuildingModel
    title: str = Field(default="", max_length=200)


class PlanEditIn(BaseModel):
    model: BuildingModel
    ops: plan_edits.PlanOps


class PatchOut(BaseModel):
    dxf: str  # base64 of the patched file
    report: dxf_patch.PatchReport


class ImportPlanOut(dxf_plan.ImportedPlan):
    checks: list[CheckResult]
    ok: bool
    svg: str  # the plan as the engine draws it, for the review step


class ConvertOut(BaseModel):
    dxf: str  # base64 of the DXF (metres)
    report: convert.ConvertReport


class ImportOut(BaseModel):
    plans: list[ImportPlanOut]
    report: dxf_plan.ImportReport


MEDIA = {"svg": "image/svg+xml", "dxf": "image/vnd.dxf", "pdf": "application/pdf", "ifc": "application/x-step"}


def create_app(llm: LLMClient | None = None, prices: PriceSource | None = None) -> FastAPI:
    app = FastAPI(title="blueprint-engine", version="0.1.0")
    app.state.llm = llm or OllamaClient()
    app.state.prices = prices or LLMWebPriceSource(app.state.llm)

    @app.exception_handler(EditError)
    async def _edit_error(_: Request, e: EditError):
        return JSONResponse(status_code=422, content={"error": "invalid_edit", "message": str(e)})

    @app.exception_handler(LLMError)
    async def _llm_error(_: Request, e: LLMError):
        return JSONResponse(status_code=502, content={"error": "llm_unavailable", "message": str(e)})

    @app.get("/healthz")
    def healthz() -> dict:
        return {"ok": True}

    @app.post("/v1/brief/from-text", response_model=BriefOut)
    def from_text(body: TextIn) -> BriefOut:
        if not body.prompt.strip():
            raise EditError("describe the house first")
        brief, notes = llm_steps.brief_from_text(app.state.llm, body.prompt[:4000], use_enhancer=body.enhance)
        return BriefOut(brief=brief, notes=notes)

    @app.post("/v1/convert/dxf", response_model=ConvertOut)
    async def convert_to_dxf(file: UploadFile = File(...), scale: int = Form(0), dpi: float = Form(0),
                             longest_m: float = Form(0)) -> ConvertOut:
        """PDF / PNG / JPEG floor plan -> DXF, no LLM. The caller then imports the DXF like any CAD file.
        Zero means 'not given'; `longest_m` (real length of the drawing's longest side) beats `scale`."""
        data = await file.read(20 * 1024 * 1024 + 1)
        if len(data) > 20 * 1024 * 1024:
            raise EditError("the file is larger than 20 MB")
        mime = file.content_type or ""
        out, rep = convert.to_dxf(data, mime, scale=scale or None, dpi=dpi or None, longest_m=longest_m or None)
        return ConvertOut(dxf=base64.b64encode(out).decode("ascii"), report=rep)

    @app.post("/v1/import/dxf", response_model=ImportOut)
    async def import_dxf(file: UploadFile = File(...), unit: str = Form("")) -> ImportOut:
        """Read a CAD floor plan: no LLM, no storage. One plan per cluster of walls found in the file;
        the caller picks which to keep. Deterministic, so a second call returns the same plans."""
        data = await file.read(dxf_plan.MAX_BYTES + 1)
        result = dxf_plan.import_dxf(data, unit or None)
        plans = []
        for p in result.plans:
            checks = run_imported_checks(p.model)
            doc, sheet = build_dxf(p.model, TitleBlock(project=p.name))
            plans.append(ImportPlanOut(**p.model_dump(), checks=checks, ok=not failed(checks), svg=to_svg(doc, sheet)))
        return ImportOut(plans=plans, report=result.report)

    @app.post("/v1/generate", response_model=llm_steps.StepResult)
    def generate(body: GenerateIn) -> llm_steps.StepResult:
        """repair=true shrinks rooms toward their minimums when the brief cannot fit (create flows);
        form edits send repair=false so the user's own numbers are never changed silently."""
        if body.repair:
            return llm_steps.create(app.state.llm, body.brief, previous=body.previous)
        return llm_steps.StepResult(result=pipeline.generate(body.brief, body.previous))

    @app.post("/v1/generate/plan", response_model=llm_steps.PlanCreated)
    def generate_plan(body: GenerateIn) -> llm_steps.PlanCreated:
        """Create from a brief as an editable CAD plan: the model (as read back), its checks and the DXF it was
        drawn into. The brief is not part of the result."""
        return llm_steps.create_plan(app.state.llm, body.brief)

    @app.post("/v1/plan/draw", response_model=llm_steps.PlanCreated)
    def plan_draw(body: PlanDrawIn) -> llm_steps.PlanCreated:
        """Draw a generated plan's model into a DXF and read it back (converting a plan made from a brief into
        an editable CAD plan). No LLM."""
        if body.model.source != "generated":
            raise EditError("this plan is already a CAD plan")
        return llm_steps.draw_plan(body.model, body.title)

    @app.post("/v1/revise", response_model=llm_steps.StepResult)
    def revise(body: ReviseIn) -> llm_steps.StepResult:
        if not body.message.strip():
            raise EditError("say what to change")
        return llm_steps.revise(app.state.llm, body.brief, body.previous, body.message[:2000], body.history)

    def ndjson(run, final: str = "result") -> StreamingResponse:
        """Run `run()` on a thread and stream NDJSON: progress lines while it works, then one result or
        error line. Closing the connection cancels at the next step."""
        events: queue.Queue[dict | None] = queue.Queue()
        cancelled = threading.Event()

        def put(ev: dict) -> None:
            if cancelled.is_set():
                raise progress.Cancelled()
            events.put(ev)

        def work() -> None:
            try:
                with progress.sink(put):
                    step = run()
                put({"event": final, "data": step.model_dump(mode="json")})
            except progress.Cancelled:
                pass
            except EditError as e:
                events.put({"event": "error", "status": 422, "error": "invalid_edit", "message": str(e)})
            except LLMError as e:
                events.put({"event": "error", "status": 502, "error": "llm_unavailable", "message": str(e)})
            except Exception as e:  # the stream is already open: report instead of dropping it
                events.put({"event": "error", "status": 500, "error": "internal", "message": str(e)[:300]})
            finally:
                events.put(None)

        threading.Thread(target=work, daemon=True).start()

        def lines():
            try:
                while (ev := events.get()) is not None:
                    yield json.dumps(ev) + "\n"
            finally:
                cancelled.set()

        return StreamingResponse(lines(), media_type="application/x-ndjson")

    @app.post("/v1/estimate/agent/turn")
    def estimate_turn(body: EstimateTurnIn) -> StreamingResponse:
        """One turn of the estimate chat, as NDJSON: progress, token, fact, question, rate, estimate, then done."""
        return ndjson(lambda: estimate_agent.run_turn(app.state.llm, app.state.prices, body.model, body.state, body.message,
                                                      body.messages, body.rate_overrides), final="done")

    @app.post("/v1/revise/stream")
    def revise_stream(body: ReviseIn) -> StreamingResponse:
        """revise, as NDJSON. A bad request is the usual 4xx before any line."""
        if not body.message.strip():
            raise EditError("say what to change")
        return ndjson(lambda: llm_steps.revise(app.state.llm, body.brief, body.previous, body.message[:2000], body.history))

    @app.post("/v1/plan/revise", response_model=llm_steps.PlanStep)
    def plan_revise(body: PlanReviseIn) -> llm_steps.PlanStep:
        """Chat edit of an imported plan (no brief): ops on the model as drawn."""
        if not body.message.strip():
            raise EditError("say what to change")
        return llm_steps.revise_plan(app.state.llm, body.model, body.message[:2000], body.history)

    @app.post("/v1/plan/edit", response_model=llm_steps.PlanStep)
    def plan_edit(body: PlanEditIn) -> llm_steps.PlanStep:
        """Direct edit of an imported plan: explicit ops, no LLM. Deterministic and checked like a chat edit."""
        return llm_steps.edit_plan(body.model, body.ops)

    @app.post("/v1/export/dxf-patch", response_model=PatchOut)
    async def export_dxf_patch(file: UploadFile = File(...), base: str = Form(...), current: str = Form(...)) -> PatchOut:
        """The original CAD file with the changes between the imported plan (`base`) and now applied to it."""
        data = await file.read(dxf_plan.MAX_BYTES + 1)
        if len(data) > dxf_plan.MAX_BYTES:
            raise EditError("the original file is too large")
        try:
            b, c = BuildingModel.model_validate_json(base), BuildingModel.model_validate_json(current)
            out, report = dxf_patch.patch_dxf(data, b, c)
        except ValueError as e:  # includes pydantic's ValidationError
            raise EditError(str(e)) from e
        return PatchOut(dxf=base64.b64encode(out).decode("ascii"), report=report)

    @app.post("/v1/plan/revise/stream")
    def plan_revise_stream(body: PlanReviseIn) -> StreamingResponse:
        if not body.message.strip():
            raise EditError("say what to change")
        return ndjson(lambda: llm_steps.revise_plan(app.state.llm, body.model, body.message[:2000], body.history))

    @app.post("/v1/review", response_model=llm_steps.StepResult)
    def review(body: ReviewIn) -> llm_steps.StepResult:
        """Reviewer agent: up to max_rounds of LLM fixes for the failed checks, each kept only
        when it lowers the issue score. The result is the best plan found, never a worse one."""
        return llm_steps.review(app.state.llm, body.brief, body.previous, body.history, max_rounds=body.max_rounds)

    @app.post("/v1/render")
    def render(body: RenderIn) -> Response:
        if body.format == "ifc":
            return Response(content=to_ifc(body.model, body.title_block.project), media_type=MEDIA["ifc"])
        tb = TitleBlock(project=body.title_block.project, version=body.title_block.version,
                        issued=body.title_block.issued, author=body.title_block.author)
        doc, sheet = build_dxf(body.model, tb)
        if body.format == "dxf":
            content: bytes | str = to_dxf_bytes(doc)
        elif body.format == "pdf":
            content = to_pdf(doc, sheet)
        else:
            content = to_svg(doc, sheet)
        return Response(content=content, media_type=MEDIA[body.format])

    @app.get("/")
    def harness() -> FileResponse:
        return FileResponse(STATIC / "index.html")

    return app


app = create_app()
