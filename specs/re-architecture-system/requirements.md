# Requirements: re-architecture-system

## Problem
`blueprint-engine` has grown into three jobs in one Python process (~8.5k lines, flat modules): (1) deterministic blueprint processing (layout, DXF/PDF/picture conversion, import, checks, draw, IFC, patch, takeoff, pricing math), (2) LLM orchestration (`llm.py`, `llm_steps.py`, `prompts.py`, the estimate interview `estimate_agent.py`, `estimate_reading.py`, `price_research.py` with web search/fetch), and (3) an HTTP/NDJSON edge. The Go API calls it as one sidecar. Roles are unclear: the "engine" is also an agent, it is not covered by the layered rules the Go API follows, and an LLM or search outage and a geometry bug live in the same process. The engine's source is flat modules with no enforced dependency direction.

## Outcome
Three services, each with one role and the same Clean Architecture + DDD layout as `api/`:

1. **API** (Go, unchanged role): business logic, authorization (OpenFGA Checks), the only reader/writer of Postgres, the only authenticated edge.
2. **Agent** (new, Python, gRPC server): all LLM work. Talks to the LLM (Ollama), decides which steps and tools to run, does web price research, and calls the Engine as tools. Stateless; conversation/session state is passed in and returned by the API on every call, as the estimate state is today.
3. **Engine** (Python, gRPC server, no LLM, no DB, no network egress): deterministic blueprint tools only. Pure functions over a plan model or file bytes. No `httpx`, no Ollama, no search.

Behavior seen by the web panel and by the HTTP API does not change.

## Scope
- In:
  - Split LLM code out of the engine into the new Agent service (`agent/`), per flow: estimate interview and price research, create/revise/review (brief and plan chat), text-to-brief, plan draw/convert-by-LLM steps.
  - gRPC contracts for API->Agent, API->Engine (deterministic ops: convert/import DXF, edit ops, render, export, patch) and Agent->Engine (tools). Protos in a shared `proto/` dir, versioned (`v1`), linted and breaking-change checked.
  - Server-streaming RPCs replace NDJSON between API and sidecars (progress, thinking, tokens, facts, questions, estimates, results). The API's SSE to the browser stays as is.
  - Agent and Engine source follow the `api/internal` layering: `domain/entity` (stdlib/pure types only), `domain/gateway` (interfaces), `usecase`, `service`, `infrastructure/*` (Ollama, search, ezdxf/ifcopenshell/OpenCV/pdfium, config), `handler/grpc`, one composition root (`wire`). An executable architecture check for the Python services joins `make check`.
  - API: `infrastructure/blueprintengine` HTTP client replaced by gRPC clients behind the existing `usecase`/gateway interfaces (`BlueprintEngine`, estimate turn); services and handlers do not change shape.
  - Compose/Make/docs: `make agent`, `make agent-test`, `make engine` (gRPC), health checks, env vars, ARCHITECTURE/docs updates, golden evals move with the LLM code.
- Out (CLAUDE.md non-goals checked: none touched):
  - No new product features, no HTTP route or web UI change, no OpenFGA model change, no migration.
  - No service auth/mTLS, service mesh, message queue, or outbox. Services stay on the local network behind the API (extending the existing "engine has no auth" stance to the Agent).
  - No change to LLM model choice or prompt behavior beyond what moving the code requires.
  - Agent does not read Postgres or OpenFGA and never sees tenant or user ids.

## Acceptance criteria
1. `engine` has no import of `httpx`, `ollama`, `llm`, `prompts`, or any search/web-fetch code, and no outbound network calls; a CI/architecture check fails if it does. Its tests pass with the network disabled.
2. `agent` is the only component that calls Ollama or the web (search + page fetch); engine and API contain no LLM client code.
3. Python services follow the layer rules (checked by an executable script in `make check`): `domain`, `usecase`, `service` never import grpc, httpx, ezdxf, ifcopenshell, cv2 or the Ollama client; only `infrastructure/` and `handler/` do; one `wire` module builds the graph; one usecase interface per operation.
4. gRPC: `proto/` compiles with lint and breaking-change checks in `make check`; generated stubs are not hand-edited; unary calls carry deadlines; streaming calls cancel upstream work when the client disconnects (the estimate turn's 30-minute ceiling and 5-minute default are preserved).
5. Every HTTP route in `docs/http-api.md` returns the same status codes and JSON as before (the existing Go service/handler tests and `make test-integration` pass; the only test edits allowed are the sidecar client tests and service tests that swap their fake to the matching new gateway, with assertions unchanged).
6. LLM-down and Agent-down both surface as 502 `engine_unavailable` (or the existing code) from the API; Engine-down fails only deterministic ops, not an Agent call that does not need the Engine.
7. The Agent calls the Engine only through a typed tool registry; a test shows a tool call is one Engine RPC and that adding a tool needs a proto method plus one registry entry.
8. Behavior parity: engine pytest (non-LLM) suite moves to engine or agent tests with no deleted case (checked against the list of collected pytest node IDs recorded before the work: 210 at 2026-11-19); `make engine-eval`/golden eval moved to `agent/` runs and scores within one case of the T0 baseline recorded on the untouched stack (`specs/re-architecture-system/baseline/`; the model is not deterministic: 14/15 and 13/15 on 2026-11-19 versus 15/15 and 14/15 on 2026-11-13) and the estimate golden stays equal.
9. The README 11-step script plus step 12 (blueprints) and the estimate chat work end to end with the three services running (`make verify-live` + HTTP smoke).
10. Docs aligned in the same change: `ARCHITECTURE.md`, new `agent/ARCHITECTURE.md`, `blueprint-engine/` architecture/README, `docs/operations.md`, `docs/testing.md`, CLAUDE.md (service list, "only the API calls the engine" invariant), `docs/harness/progress.md`.
11. Each rollout step leaves `make check` green and the system runnable (incremental cut-over, one flow per step; old HTTP path removed only after its flow is switched and verified).

## Impact on the current system
- Tenancy / authorization: none. Checks stay in API services before any Agent/Engine call; a new invariant is added: neither Agent nor Engine receives tenant or user ids, and Agent never gets DB or OpenFGA access. No new action, no model change, `authz-reviewer` still runs on `service/` changes (the clients are infrastructure).
- Existing data: none. Sessions, estimates, versions and originals stay in Postgres written by the API. Opaque state/model JSON crosses the gRPC boundary as `bytes` or `google.protobuf.Struct`; formats are unchanged.
- Existing clients: web unchanged. The old engine HTTP API is removed at the end (internal only).
- Infra: two host processes instead of one (Agent :8091 gRPC, Engine :8092 gRPC, both on the host; Ollama stays on the host for Metal). New tooling: grpcio, protobuf/buf for Python and Go (protoc-gen-go), uv workspace or two uv projects. `go.work` gains nothing new for the Agent (Python); API gains `google.golang.org/grpc`.

## Proposals beyond the ask (answers to "Open thought")
Adopted into scope unless you strike them:
- **Contracts first**: one `proto/` directory is the single source of truth for service contracts (like `model.fga` and the docs for authorization). `buf lint` + `buf breaking` in `make check`.
- **Agent stays stateless**; API owns session state (already how estimate works). This keeps scaling and fail-over trivial and avoids a second datastore.
- **Tools are the engine's public surface**: engine RPCs are narrow, typed, side-effect-free tools (`LayoutPlan`, `RunChecks`, `ApplyPlanOps`, `Takeoff`, `PriceLines`, `Render`...). The Agent never imports engine code.
- **Deterministic orchestration with LLM at guarded steps** stays the default (qwen3 8B is unreliable as a free tool-calling loop; the estimate agent's code guards exist for that reason). The Agent gets a tool-calling loop abstraction so a stronger model can be adopted later without changing contracts.
- **Cross-cutting basics**: request-id propagation in gRPC metadata to logs in all three services, gRPC health service, standard status-code mapping in one place per client, deadlines on every call.
- **Price web research belongs to the Agent** (it is egress + LLM); quantity takeoff and pricing math stay in the Engine (deterministic, testable).
- **Engine internals are repackaged by layer** (domain model/geometry rules, usecases per operation, infrastructure for ezdxf/IFC/OpenCV/pdfium) without behavior change; big modules (`dxf_plan.py` 1k lines, `convert.py` 768) are split only along existing seams.
- Not proposed (deliberately): an API gateway/mesh, event bus, separate DB per service, service auth. Revisit when a second consumer or network exposure appears.

## Decisions taken with PO / biz
- Agent language -> Python -> user, 2026-11-19.
- API<->Agent and Agent<->Engine protocol -> gRPC (API->Engine deterministic ops also gRPC, so the engine speaks one protocol) -> user, 2026-11-19.
- Non-LLM engine calls go API -> Engine directly; Agent calls the Engine only as tools -> user, 2026-11-19.
- Rollout -> incremental, one flow at a time -> user, 2026-11-19.
- Feature name `re-architecture-system`; stop after `plan` -> user, 2026-11-19.

## Open questions
None blocking. Defaults to confirm at approval: (a) deterministic orchestration with a tool-calling loop abstraction rather than a free LLM tool loop now; (b) price research (search + fetch + robots) moves to the Agent; (c) uv: separate `agent/` and `blueprint-engine/` projects (no shared Python library; shared types come only from the protos).

## Changes after approval
- 2026-11-19 (plan review): criterion 5 and 8 wording tightened as above (service-test fake swap allowed; baseline is 210 collected node IDs, not 157 test functions). Engine gRPC port is :8092 permanently (the legacy HTTP :8090 is deleted), so no port flip.
- 2026-11-19 (T0): criterion 8's eval gate is relative to the baseline measured on the untouched stack at T0, within one case, because the model is not deterministic.
