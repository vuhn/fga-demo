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

The Agent calls the Engine only as tools. Nothing else is allowed: the Engine never calls out, and neither the Agent nor the Engine reads Postgres or OpenFGA.

## Rules

1. **Identity never crosses.** No message or field name carries the words tenant, user, project or actor, and no metadata carries them. The only metadata key is `x-request-id` (opaque, logged by all three services). `scripts/check-proto-identity.py` checks the compiled descriptors with whole-word matching of message, field, enum, service and rpc names (not enum values: `FACT_SOURCE_USER` says who stated a fact). It runs in `make proto`, which `make check` runs.
2. **The API Checks first.** Every OpenFGA Check happens in an API service before any Agent or Engine call; the Agent and the Engine are never reachable from the browser.
3. **Opaque JSON where the API stores it.** Plans (`model_json`), briefs and edit operations cross as JSON `bytes` in the Engine's schema, published by `EngineService.ListTools`. The Agent builds LLM prompts and structured-output schemas from `ListTools` and `GetCatalog` and never duplicates them. The Agent's conversation state (`state_json`) is opaque to the API and the Engine.
4. **Typed where a service must interpret.** `EstimateFacts`, rates, takeoff lines, `PlanSummary`, `Check`, uploads.
5. **v1 is additive.** `buf breaking` (FILE rules) runs against `main` once `main` has `proto/`; a breaking change needs a new `v2` package.
6. **Generated stubs are shared, committed, and never hand edited.** `make proto-gen` (`scripts/gen-proto.sh`) writes them once, outside every service: Go into `proto/go` (module `github.com/vuhn/fga-demo/proto/go`, mirroring the proto packages, e.g. `proto/go/fga/demo/engine/v1`) and Python into `proto/python` (package `fga`, e.g. `from fga.demo.engine.v1 import engine_pb2`). A service only imports them (the API through `require` + `replace ../proto/go` in `api/go.mod` and `go.work`; the Agent and Engine through the `fga-proto` path dependency on `../proto/python` in their `pyproject.toml`) and never generates code into its own tree. `make proto` fails on drift. Pinned generators: `protoc-gen-go` v1.36.12, `protoc-gen-go-grpc` 1.5.1, `grpcio-tools` from `proto/python/uv.lock`.

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

On a stream, an error before the first message is a status as above; after the first message it is an `Error` event `{status, error, message}` (the API's SSE is already open): 422 `invalid_edit`, 502 `llm_unavailable` or `engine_unavailable`, 504 `timeout` (an estimate turn ran past the Agent's own ceiling, 30 minutes, `ESTIMATE_TURN_SECONDS`), 500 `internal`. Every unary call has a deadline from the API client (5 minutes; `EstimateTurn` up to 30). Max message size is 32 MB (uploads are capped at 20 MB). Both servers expose `grpc.health.v1.Health`.

## Calls

`AgentService`: `BriefFromText` (unary), `Revise`, `RevisePlan`, `Review` (server streams of `BlueprintEvent`: progress, then one result or error), `EstimateTurn` (server stream of `EstimateEvent`: progress, then in order `fact` for each change the message made, `token` as the reply is written, `question` when the turn asks one, `rate` for each researched rate and `estimate_json` when it priced the plan, then `done` or `error`). The request carries the plan, the opaque session (`state_json`, the shape the Engine-era agent wrote, so an open session carries on), the message, the chat so far and the person's own rates; `done` carries the new session, its status, the whole reply and, only when the turn priced the plan, the estimate. The Agent keeps nothing between turns. `GetCatalog`'s `RateSpec` says which rates depend on quality (`quality_sensitive`).

`EngineService` (all unary): `ListTools`, `GetCatalog`; `NormalizeDraft`, `Generate`, `ApplyBriefOps`, `CreatePlan`, `DrawPlan`, `ApplyPlanOps`, `RunChecks`, `SummarizePlan`; `Takeoff`, `PriceEstimate`; `ConvertToDXF`, `ImportDXF`, `PatchDXF`, `Render`.

Which API service calls which: Agent: `blueprint_drafter`, `blueprint_reviser`, `blueprint_revise_streamer`, `blueprint_reviewer`, `estimate_session_messenger`. Engine: `blueprint_creator`, `blueprint_brief_updater`, `blueprint_importer`, `blueprint_dxf_previewer`, `blueprint_drawing_renderer`, `blueprint_original_exporter`, `blueprint_plan_converter`, `blueprint_plan_editor`.

## Status

The API's `infrastructure/agent` client (T14) calls `BriefFromText`, `Revise`, `RevisePlan` and `Review` when `AGENT_ADDR` is set (`Review` asks for at most 3 rounds) and `EstimateTurn` (T15, mapped onto the browser's SSE events); the sync variants read the stream to its result. The Agent scaffold exists in `agent/` (T10): `serve` on :8091, health tied to the Engine's catalog (the shared `DesignCatalogLoader` helper in `usecase/helper` + `service/helper`), the typed `BlueprintEngine` gateway with its gRPC adapter (`infrastructure/engine_grpc/blueprint_engine.py`, one `@rpc` method per Engine RPC; a test fails until every `EngineService` method is used or deliberately left out), the Ollama adapter behind `LanguageModel`, and the stream helper that cancels a run when the client leaves. `BriefFromText`, `Revise`, `RevisePlan` and `Review` are served (T11a, T11b); `EstimateTurn` is served (T13b): the Agent reads the message (`EstimateReader`), keeps the interview (`EstimateInterview`), researches prices (`PriceResearcher` through `EstimatePricer`) and writes the reply (`EstimateReplier`); `EstimateTurner` orchestrates one turn and the Engine only measures (`SummarizePlan`, `Takeoff`) and prices (`PriceEstimate`).

Both services are live. The Engine (`blueprint-engine/`, `python -m fga_engine.serve`, :8092) serves every `EngineService` RPC; the Agent (`agent/`, :8091) serves `BriefFromText`, `Revise`, `RevisePlan`, `Review` and `EstimateTurn`. The API calls the Agent for every LLM operation and the Engine for every deterministic one; the Agent calls the Engine only as tools. There is no HTTP engine any more.
