# Plan: usecase-io-structs

Input: approved [requirements.md](requirements.md). System design and DB design are skipped (see README): nothing to design, no schema, no contract.

Base branch: `main`. **One task, one PR** (decision of the user, 2026-11-20, overriding the per-domain slicing of the first draft: the work is one mechanical change, "create the structs and apply them to usecases and gateways").

## Strategy
- **Delivery pattern: one atomic change.** No flag, no cutover, no soak gate, no allow-list ratchet: the PR lands the new shape and the checks that enforce it together, so `main` is either the old code or the finished new code. Behavior is the same code path with different parameter types.
- **Rollback:** revert the PR (no data, schema, contract or config involved).
- **Review aid inside the PR:** ordered commits, each building and passing `make check`: (1) spec-first docs, baselines and the domain replay script, (2) `Authorizer` moves to `usecase/helper`, (3) `AuthzGateway.Check` params, (4) one commit per domain, blueprint and estimate last, (5) the new checks with their planted-violation tests (introduced once the migration is complete, so no allow-list is ever committed), (6) docs reconcile.
- **Conventions** (written first in `CLAUDE.md`, `api/ARCHITECTURE.md`, the `go-api` skill and the evaluator rubric):
  - `usecase/input/<domain>.go` holds `type <Interface> struct{...}` (`input.FolderCreator`), `usecase/output/<domain>.go` holds `output.<Interface>`; one file per domain (`input/folder.go` holds every folder input). The old in-file `*Input` structs and usecase-local result structs are deleted from `usecase/*.go`.
  - `input.X` carries `Actor`, `TenantID` (when the method has one), ids and body fields; `ctx` is the only other param; a streaming callback stays a trailing param. A method with no result returns `error` only.
  - `output.X` has its own copied fields (no `model.*` field type, no embedded model). Where several usecases return the same shape (`BlueprintDetail`, `EstimateSessionDetail`, `TenantView`), the shared field set is one type in the same domain file and each usecase's output embeds it (`struct{ BlueprintDetail }`). Services build outputs through mapping functions in `service/<model>_output.go`; handler mappers convert `output.X` to the JSON view and request bodies to `input.X`.
  - `usecase/helper` is exempt; `Authorizer` moves there (a shared decision port, not an operation).
  - Gateways: more than 3 params after `ctx` becomes `model.<Action><Model>Params` in `domain/model`, one file per concept; an existing fitting struct (`model.CheckRequest` for `AuthzGateway.Check`) is reused, and the check accepts any single `model.*` struct.
- **Authz safety:** services keep every `Authorizer.Require*`, `PlatformOr` and `WriteGuard` call as is; a diff self-check (`git diff` shows these lines unchanged or only re-pointed at `in.Actor` / `in.TenantID`) runs before the PR, then `authz-reviewer` on the whole diff.
- **Baselines recorded before the first behavior-touching change** (commit 1): `scripts/smoke-replay.sh` covers only blueprint and estimate routes, so a new non-LLM `scripts/smoke-replay-domains.sh` replays every other domain's routes as seeded users (users, auth/me, companies, products, tenants, inspector, `/me/permissions`, members, roles, folders, albums, documents, photos, projects, project members and roles; allow, 403 `{"error","check"}`, 404 GET, platform-only product toggle, 400 `cannot_upload`), recording status and normalized JSON shape. Both baselines and the test inventory (`go test -list '.*' ./...`) are saved under `specs/usecase-io-structs/baseline/`; the script runs twice before the change to prove it is deterministic.

## Endpoint inventory
Deployable `api/` plus `infra` (scripts, docs). Every HTTP route is unchanged (`docs/http-api.md`); the whole set is touched as one slice.

| Deployable | Scope | Change | Task |
| --- | --- | --- | --- |
| api | all operation files in `usecase/*.go` (about 110 operation methods) -> `usecase/input`, `usecase/output`; services, handlers, handler mappers, tests | types only | T1 |
| api | every `domain/gateway` method with more than 3 params after `ctx` (about 15: Authz `Check`, membership and project `UpdateRole`/`AssignRole`, project and role `Removal`, document and photo `ListByIDs`, blueprint `CreateImported`, `BlueprintAgent` `Revise`/`ReviseStream`/`RevisePlanStream`, estimate `AppendTurn` and `EstimateTurn`) -> `model.*Params`; infrastructure implementations, `internal/seed/world.go`, fakes | types only | T1 |
| infra | new checks, planted-violation tests, `smoke-replay-domains.sh`, docs and skill rules | new | T1 |

## Infrastructure / 3rd-party changes
None: no env var, compose service, secret, OpenFGA model, provider or migration. Tooling only (`scripts/check-architecture.sh` plus a small `go/parser` program under `scripts/`, `scripts/test-architecture-go.sh` with fixtures under `scripts/testdata/`, `scripts/smoke-replay-domains.sh`).

## Tasks
| ID | Type | Deployable | Slice | Scope | Est (h) | Depends on | Merge safety | E2E test | Rollback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| T1 | E | api (with the infra scripts and docs it needs) | Usecase `input`/`output` structs and gateway `*Params` structs applied everywhere, with the checks that enforce them | Commits 1-6 above. Domains: user, tenant, auth (after tenant: `Me` returns the tenant view), company, product, inspector and permissions, member (gateway params and the `internal/seed/world.go`, `member_assignment.go`, `member_remover.go` callers), role, folder, album, document, photo, project, project members and roles (`project_member_helper.go`, `project_member_remover.go`, seed callers), blueprint (all at once: `CreateImported` has two callers, `blueprint_creator.go` and `blueprint_importer.go`), estimate. Checks: (a) every operation method is `ctx` + `in input.X` (+ trailing callback) returning `output.X` or only `error`, no `model.` in the signature, `input.X`/`output.X` exist; (b) no `model.` field types in `usecase/output`; (c) at most 3 params after `ctx` in every `domain/gateway` method; (d) `domain`, `usecase`, `service` import only the standard library and `api/internal/{domain,usecase}` (already true today; third-party allow-list empty); planted-violation fixture per rule | 90 | - | Atomic: the old shape disappears and the new one with its checks appears together; no intermediate state on `main` | `make check`; `make verify-live` (resets data); `smoke-replay-domains.sh` and `smoke-replay.sh` diffs vs baseline identical; `smoke-failures.sh`; planted-violation tests; `authz-reviewer` accepts | revert the PR |

Total estimate: 90 h, one PR. No critical path or parallelism: a single task.

## DB design coverage
Skipped: DB design is skipped (no table, column, index, constraint or data change). No task adds or edits a migration file, and `scripts/check-specs.sh` passes with DB design skipped.

## Merge sequence walk
One PR. Before it: old shape. After it: new shape, checks green, replays unchanged. No intermediate state on `main`, so no ordering to walk.

## Release order
Merge, then restart `cmd/api`. No config flip, migration or ordering beyond that.

## Test plan
| Step | Acceptance criteria | Gates which task |
| --- | --- | --- |
| `scripts/check-architecture.sh` rules (a)-(d) and `scripts/test-architecture-go.sh` planted violations | 1, 2, 3, 4 | T1 |
| `make check` (vet, unit tests, architecture rules, docs/model sync, web build), no allow-list | 5 | T1 |
| `make verify-live` and `make test-integration` | 6 | T1 |
| `smoke-replay-domains.sh` and `smoke-replay.sh` diffs vs baseline; `smoke-failures.sh`; test-inventory diff (no test lost) | 6 | T1 |
| `authz-reviewer` over the diff vs `main`, plus the diff self-check on Check/WriteGuard lines | 7 | T1 |
| `scripts/check-specs.sh`, docs/model sync, review of `CLAUDE.md`, `api/ARCHITECTURE.md`, go-api skill, rubric | 8 | T1 |

## Notes on requirements
- Criterion 5 ("no new allow-list entries") is met literally: with one atomic PR there is no ratchet.
- Layer independence rule (d) is already satisfied today; T1 locks it in rather than fixing anything.

## Risks
- **Review size.** One PR of roughly 400 files, mostly mechanical, far above a 15-30 minute review. Accepted by the user; mitigated by ordered commits that each build, a pure-rename commit for `Authorizer`, replay diffs, and checks proving the shape. A long-lived branch conflicts with other `api/` work: merge `main` into it at commit boundaries and finish in as few sittings as possible.
- **Output duplication.** Copying fields per usecase could balloon (`BlueprintDetail`): defined types over one shared field set per domain. If review decides that breaks "own struct, copied fields", blueprint falls back to explicit per-usecase structs.
- **Hidden behavior change in mapping.** A dropped or renamed field changes JSON: the domain replay diff, plus a handler mapper test per domain asserting the JSON view of a filled output.
- **Check-script false positives** on multi-line signatures: parsed with `go/parser`, not regexes; fixtures cover multi-line methods.
- **`Authorizer` move** is outside the PRD text: it is a shared rule interface, so `usecase/helper` is its place and the rule has no exceptions inside `usecase/*.go`.
- **Counts:** about 110 operation methods (119 minus the 9 `Authorizer` methods), 126 service files, 20 handlers, about 15 gateway methods over the limit.
