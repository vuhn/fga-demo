"""Stateless HTTP engine. No auth and no storage: only the Go API calls it, after its own Checks.

Run: uv run uvicorn blueprint_engine.app:app --port 8090
"""

from __future__ import annotations

from pathlib import Path
from typing import Literal

from fastapi import FastAPI, File, Form, Request, UploadFile
from fastapi.responses import FileResponse, JSONResponse, Response
from pydantic import BaseModel, Field

from . import llm_steps, pipeline
from .draw import TitleBlock, build_dxf, to_dxf_bytes, to_pdf, to_svg
from .edits import EditError
from .ifc import to_ifc
from .llm import LLMClient, LLMError, OllamaClient
from .schemas import Brief, BuildingModel

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


MEDIA = {"svg": "image/svg+xml", "dxf": "image/vnd.dxf", "pdf": "application/pdf", "ifc": "application/x-step"}


def create_app(llm: LLMClient | None = None) -> FastAPI:
    app = FastAPI(title="blueprint-engine", version="0.1.0")
    app.state.llm = llm or OllamaClient()

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

    @app.post("/v1/brief/from-sketch", response_model=BriefOut)
    async def from_sketch(file: UploadFile = File(...), note: str = Form("")) -> BriefOut:
        data = await file.read(llm_steps.MAX_UPLOAD + 1)
        brief, notes = llm_steps.brief_from_sketch(app.state.llm, data, file.content_type or "", note[:1000])
        return BriefOut(brief=brief, notes=notes)

    @app.post("/v1/generate", response_model=llm_steps.StepResult)
    def generate(body: GenerateIn) -> llm_steps.StepResult:
        """repair=true shrinks rooms toward their minimums when the brief cannot fit (create flows);
        form edits send repair=false so the user's own numbers are never changed silently."""
        if body.repair:
            return llm_steps.create(app.state.llm, body.brief, previous=body.previous)
        return llm_steps.StepResult(result=pipeline.generate(body.brief, body.previous))

    @app.post("/v1/revise", response_model=llm_steps.StepResult)
    def revise(body: ReviseIn) -> llm_steps.StepResult:
        if not body.message.strip():
            raise EditError("say what to change")
        return llm_steps.revise(app.state.llm, body.brief, body.previous, body.message[:2000], body.history)

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
