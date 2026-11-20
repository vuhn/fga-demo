# Requirements: usecase-io-structs

Source: `prds/standardline-input-output-model.md` (gitignored raw ask).

## Problem
In the Go API (`api/internal`):
1. Usecase inputs and outputs are loose: inputs are a mix of positional params and ad-hoc `Create*Input` structs declared in the same file as the interfaces (10 of them; 119 methods in 18 files).
2. 77 of 119 usecase methods return `model.*` (or a usecase-local struct such as `BlueprintDetail`), so callers (handlers) depend on domain models and the business layer has no output contract of its own.
3. Gateways with many params (14 methods with more than 3 params after `ctx`, e.g. `RoleRelationGateway.Removal` with 5, `EstimateAgent.EstimateTurn` with 6) are unreadable and easy to call with swapped strings.
4. Nothing in the harness stops this from coming back, and the "domain, usecase and service are the business layer and must not depend on infrastructure, third parties or delivery" rule is only partly enforced (SDK imports are checked, imports of `infrastructure/` or `handler/` and arbitrary third-party packages are not).

## Outcome
- `api/internal/usecase/input` and `api/internal/usecase/output` packages hold one struct per usecase interface, named after it: `input.FolderCreator`, `output.FolderCreator`.
- Every usecase method has the form `Method(ctx context.Context, in input.X) (output.X, error)`; `ctx` is the only param outside the struct (actor, tenant id and ids move into `input.X`). A method with no result returns `error` only (no empty output struct). A streaming callback (`onProgress`) stays a trailing param.
- Usecases never expose `domain/model`; services map model -> `output.X` (own struct, copied fields); handler mappers map `output.X` -> JSON view, and request body -> `input.X`.
- Any gateway method with more than 3 params after `ctx` takes one `model.<Action><Model>Params` struct (e.g. `NewFolderParams`, `UpdateFolderParams`, `RemoveRoleParams`), defined in `domain/model`.
- `make check` fails when either rule or the layer-independence rule is broken, and `CLAUDE.md`, `api/ARCHITECTURE.md`, the `go-api` skill and the evaluator rubric state the rules.
- Observable behavior is unchanged: same routes, JSON, status codes, Checks, write ordering.

## Scope
- In:
  - All usecase interfaces in `api/internal/usecase/*.go` (not `usecase/helper`): move input/output into `usecase/input` and `usecase/output`, one file per usecase (`folder_creator.go` in each), remove the old in-file `*Input` structs and usecase-local result structs.
  - Services (`service/`), handlers and handler mappers (`handler/http`, `handler/http/mapper`), `cmd/api/wire.go` and tests updated to the new types.
  - All gateway interfaces in `domain/gateway` (persistence, relation, Authz, Agent, Engine): params structs for methods with more than 3 params after `ctx`; infrastructure implementations and fakes updated.
  - Harness: new checks in `scripts/check-architecture.sh` (with planted-violation tests next to the existing ones), updated `CLAUDE.md`, `api/ARCHITECTURE.md`, `docs/harness/evaluator-rubric.md`, `.claude/skills/go-api`, `docs/testing.md` if test layout changes.
  - Layer independence: `domain`, `usecase` (incl. `input`, `output`, `helper`) and `service` must not import `infrastructure/*`, `handler/*`, `cmd/*`, or third-party packages outside a short allow-list kept in the check script (the Go standard library and this module's business-layer packages are always allowed).
- Out:
  - Python services (`agent/`, `blueprint-engine/`): decided Go API only.
  - Any behavior, route, JSON, authorization model, schema or seed change. No new Checks. `web/` is untouched.
  - `usecase/helper` interfaces (`Authorizer`, `WriteGuard`, `SharePolicy`, ...): they are shared rules, not operations, so no input/output structs (their gateway-style params follow the gateway rule only if they are in `domain/gateway`).
  - Renaming operations or merging domains (per-domain separation in CLAUDE.md stays).
  - CLAUDE.md non-goals checked: nothing scaffolded.

## Acceptance criteria
1. `usecase/input` and `usecase/output` exist; for every operation interface `X` in `usecase/*.go` there is `input.X` (even when it has only an id) and, when the method returns data, `output.X`; no `usecase/*.go` method signature has a param other than `ctx` and `in input.X` (plus a trailing callback) and none mentions `model.` in its params or results. Verified by `scripts/check-architecture.sh`.
2. No struct in `usecase/output` embeds or aliases a `domain/model` type as its result (fields are copied); verified by review (evaluator rubric) and a check for `model.` in field types where it is mechanically decidable.
3. Every method of every interface in `domain/gateway` has at most 3 params after `ctx`; a method that would exceed it takes `model.<Action><Model>Params`. Verified by the check script.
4. A check fails (planted-violation test for each) when: a usecase signature breaks rule 1; a gateway method has more than 3 params after `ctx`; `domain`, `usecase` or `service` imports `infrastructure/`, `handler/`, `cmd/` or a third-party package not on the allow-list.
5. `make check` passes with no new allow-list entries beyond the third-party allow-list in criterion 4's check.
6. `make verify-live` passes unchanged (integration tests assert behavior, so a passing run shows no behavior change), and the README click script steps 1-11 still behave the same (spot check through `scripts/smoke-replay.sh` against `baseline/replay-before.txt`: identical statuses and JSON shapes).
7. `authz-reviewer` accepts the diff against `main` (every Check, write ordering and `base_version` handling is unchanged).
8. `CLAUDE.md`, `api/ARCHITECTURE.md`, the `go-api` skill and `docs/harness/evaluator-rubric.md` describe the rules and the layout; `scripts/check-specs.sh` passes.

## Impact on the current system
- Tenancy / authorization: none. Services keep Checking via `usecase.Authorizer` before any read/write; the actor and tenant id only change where they travel (inside `input.X`). Handlers still read the actor from the JWT and the tenant from `X-Tenant-ID`.
- Existing data / migrations: none.
- Existing clients: HTTP and gRPC contracts unchanged.
- Size: about 119 usecase methods, 126 service files, 20 handlers and mappers, 14 gateway methods, plus every test that calls them. Large but mechanical; it must be sliced into several mergeable PRs (plan stage), each leaving `make check` green, probably per domain.
- Infra / third party: none.

## Decisions taken with PO / biz
- Output shape: own struct with copied fields, not a wrapper of the model -> user, 2026-11-20.
- Input scope: every method; only `ctx` stays a separate param, actor and tenant id move into the input struct (the PRD example kept `actor, tenantID` outside; the user corrected it) -> user, 2026-11-20.
- Rules cover operations plus all gateways (Agent, Engine and Authz included); `usecase/helper` exempt; streaming callbacks stay a trailing param -> user, 2026-11-20.
- Harness scope: Go API only -> user, 2026-11-20.
- Skip system design and DB design (nothing to design) -> user, 2026-11-20.

## Open questions
None.
