# Service contracts

The three services of the blueprint product talk gRPC. The `.proto` files in [`proto/`](../proto) are the only source of truth; this page explains the rules around them. Spec: `specs/re-architecture-system/` (requirements, system design, plan).

```text
API :8088 --agent.v1--> Agent :8091 --engine.v1--> Engine :8092
API ----------------------engine.v1----------------^
```

| Service | Package | Role | Port |
| --- | --- | --- | --- |
| API (Go) | none | Business logic, authorization, the only reader/writer of Postgres | 8088 (HTTP) |
| Agent (Python) | `fga.demo.agent.v1` | All LLM work and web price research. Stateless | 8091 |
| Engine (Python) | `fga.demo.engine.v1` | Deterministic blueprint tools: layout, checks, DXF/PDF/picture/IFC, takeoff and pricing math. No LLM, no database, no network | 8092 |
| shared | `fga.demo.common.v1` | Error detail, checks, uploads, estimate vocabulary | n/a |

The Agent calls the Engine only as tools. Nothing else is allowed: the Engine never calls out, and neither sidecar reads Postgres or OpenFGA.

## Rules

1. **Identity never crosses.** No message or field name carries the words tenant, user, project or actor, and no metadata carries them. The only metadata key is `x-request-id` (opaque, logged by all three services). `scripts/check-proto-identity.py` checks the compiled descriptors with whole-word matching of message, field, enum, service and rpc names (not enum values: `FACT_SOURCE_USER` says who stated a fact). It runs in `make proto`, which `make check` runs.
2. **The API Checks first.** Every OpenFGA Check happens in an API service before any Agent or Engine call; a sidecar is never reachable from the browser.
3. **Opaque JSON where the API stores it.** Plans (`model_json`), briefs and edit operations cross as JSON `bytes` in the Engine's schema, published by `EngineService.ListTools`. The Agent builds LLM prompts and structured-output schemas from `ListTools` and `GetCatalog` and never duplicates them. The Agent's conversation state (`state_json`) is opaque to the API and the Engine.
4. **Typed where a service must interpret.** `EstimateFacts`, rates, takeoff lines, `PlanSummary`, `Check`, uploads.
5. **v1 is additive.** `buf breaking` (FILE rules) runs against `main` once `main` has `proto/`; a breaking change needs a new `v2` package.
6. **Generated stubs are committed, never hand edited.** `make proto-gen` writes them (`scripts/gen-proto.sh`); `make proto` fails on drift. Pinned generators: `protoc-gen-go` v1.36.12, `protoc-gen-go-grpc` 1.5.1, `grpcio-tools` from `blueprint-engine/uv.lock`. Locations: `api/internal/infrastructure/grpcgen`, `<python package>/gen`.

## Errors

Refusals are gRPC statuses with an `ErrorDetail` (`common.v1`), serialized into the trailing metadata key `error-detail-bin` (the status message repeats the text):

| Status + detail code | API error | HTTP |
| --- | --- | --- |
| `INVALID_ARGUMENT` + `EDIT_ERROR` / `BAD_FILE` | `ErrValidation` | 400 / 422 as today |
| `INVALID_ARGUMENT` + `UNSUPPORTED` | `ErrValidation` (stops the Agent's retry loop) | 422 |
| `UNAVAILABLE`, connection refused, `DEPENDENCY_UNAVAILABLE` | `ErrEngineUnavailable` | 502 `engine_unavailable` |
| `DEADLINE_EXCEEDED` | `ErrEngineUnavailable` | 502 |
| `CANCELLED` (caller left) | none, nothing is written | none |
| anything else | internal | 500 |

On a stream, an error before the first message is a status as above; after the first message it is an `Error` event `{status, error, message}` (the API's SSE is already open). Every unary call has a deadline from the API client (5 minutes; `EstimateTurn` up to 30). Max message size is 32 MB (uploads are capped at 20 MB). Both servers expose `grpc.health.v1.Health`.

## Calls

`AgentService`: `BriefFromText` (unary), `Revise`, `RevisePlan`, `Review` (server streams of `BlueprintEvent`: progress, then one result or error), `EstimateTurn` (server stream of `EstimateEvent`: progress, token, fact, question, rate, estimate, then done or error).

`EngineService` (all unary): `ListTools`, `GetCatalog`; `NormalizeDraft`, `Generate`, `ApplyBriefOps`, `CreatePlan`, `DrawPlan`, `ApplyPlanOps`, `RunChecks`, `SummarizePlan`; `Takeoff`, `PriceEstimate`; `ConvertToDXF`, `ImportDXF`, `PatchDXF`, `Render`.

Which API service calls which: Agent: `blueprint_drafter`, `blueprint_reviser`, `blueprint_revise_streamer`, `blueprint_reviewer`, `estimate_session_messenger`. Engine: `blueprint_creator`, `blueprint_brief_updater`, `blueprint_importer`, `blueprint_dxf_previewer`, `blueprint_drawing_renderer`, `blueprint_original_exporter`, `blueprint_plan_converter`, `blueprint_plan_editor`.

## Status

The contracts and generated stubs exist. The Engine serves the catalog, brief, plan and estimate RPCs in-process (`fga_engine/handler/grpc`: `EngineServicer`, `build_server` with health and 32 MB limits; T7a); the file RPCs and the `serve` entrypoint on :8092 come with T7b, so nothing listens yet. The HTTP engine on :8090 stays the live path until the plan's T9 and T14/T15 switch each flow (see the plan's strategy and release order).
