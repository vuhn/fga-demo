# Engine architecture

The Engine (`blueprint-engine/`, gRPC :8092) is deterministic: no LLM, no network, no auth, no storage, no tenant or user ids. **Code does every piece of geometry.** The API and the Agent call it. Contracts: `proto/fga/demo/engine/v1/engine.proto`, summary in `docs/service-contracts.md`. What each piece does and the DXF/IFC conventions: `README.md`.

```text
API :8088 ----gRPC----> Engine :8092 <----gRPC tools---- Agent :8091
```

## Layers

`domain/` (pure geometry, rules, models: `layout`, `building`, `fixtures`, `checks`, `edits`, `plan_edits`, `schemas`, `estimate*`, `rate_catalog`, `tools`) with `domain/gateway/` (one Protocol per file for DXF read/draw/patch, rendering, conversion) -> `usecase/<op>.py` (Protocol) -> `service/<op>.py` (same file name) -> `infrastructure/` (ezdxf, ifcopenshell, OpenCV, pdfium, rapidocr live only here) -> `handler/grpc/` (`<domain>_handler.py` with its own use cases, `mapper/`, `common/`, `servicer.py`, `server.py`). `wire.py` is the only composition root; `serve.py` the entry point. Inner layers import no SDK. `scripts/check-architecture-python.sh` (with an empty allow-list) and `scripts/check-python-shape.py` enforce this; `.claude/skills/python-engine/SKILL.md` is the checklist.

## Operations

- **Brief and plan:** `Generate` (layout, shrink-to-fit with `repair`), `ApplyBriefOps`, `NormalizeDraft`, `CreatePlan` (brief drawn into our DXF and read back like an import), `DrawPlan`, `ApplyPlanOps`, `RunChecks`.
- **Files:** `ImportDXF`, `ConvertToDXF` (PDF or picture), `PatchDXF` (differences written into the original), `Render` (SVG, PDF, DXF, IFC).
- **Tools for the Agent:** `ListTools` (per-operation JSON Schema and signature), `GetCatalog` (minimum areas, priced fixtures, rate catalog).
- **Estimate:** `SummarizePlan`, `Takeoff`, `PriceEstimate`.

## Boundaries

A refusal is `INVALID_ARGUMENT` with an `ErrorDetail` in the `error-detail-bin` trailer. Messages up to 32 MB are accepted; a DXF is at most 20 MB. Same input, same output: layouts stay stable against the previous version. Uploads (PDF, picture) are converted and discarded.
