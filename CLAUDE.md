# CLAUDE.md

OpenFGA multi-tenant authorization demo: Go API in `api/` (its own module, Gin, pgx, goose) + OpenFGA + React admin panel in `web/` + `blueprint-engine/` (Python FastAPI sidecar, uv, local Ollama; stateless, no auth; only the API calls it, plus the Agent that will call its deterministic tools once the Agent exists; it never sees a tenant or user id). Postgres is the display record; OpenFGA makes every allow/deny decision.

## Source of truth

- Architecture overviews: `ARCHITECTURE.md` (system; read first), `api/ARCHITECTURE.md` (read when touching Go layers), `web/ARCHITECTURE.md` (read when touching the panel). Detailed reference lives in `docs/`: `authorization-model.md` and `tuple-catalog.md` (model or relation builders), `decision-catalog.md` (any new action or Check), `http-api.md` (routes), `database-schema.md` and `seed-data.md` (migrations, seed), `web-ui.md` (UI), `testing.md` (tests), `operations.md` (compose, env). `README.md` is the story and the 11-step click script. **Keep code, the architecture files, `docs/`, and `README.md` aligned in the same change.** If behavior changes, update the spec first.
- Session state lives in `docs/harness/`: `progress.md` (where we are), `decisions.md`, `sdlc.md`, `lessons.md`, `clean-state-checklist.md`, `evaluator-rubric.md`, `quality.md`.
- `api/fga/model.fga` must equal the DSL block in `docs/authorization-model.md` exactly.
- Seed ids/tuples (with each container's private/public flag) live once in `api/internal/seed/world.go` (one slice per domain); `cmd/seed` writes them and the integration tests assert against them. Do not duplicate ids elsewhere.

## Commands

```bash
make init               # tools, .env, npm ci, make check
make check             # one command: vet, unit tests, architecture rules, docs/model sync, web build
make verify-live       # compose-up + seed + integration tests (RESETS data)
make compose-up         # Postgres :5432, OpenFGA :8080 (gRPC :8081, Playground :3000)
make seed               # RESETS everything: truncates app tables, recreates the fga-demo store, writes the seed
make api                # :8088 — migrates, opens existing store + newest model; never seeds
make web                # Vite :5173, proxies /api -> :8088
make engine             # blueprint-engine :8090 (needs Ollama: qwen3:latest, only for describe/chat; uploads need no LLM)
make engine-test        # engine pytest (no LLM); make engine-eval runs golden prompts on the live LLM
make test               # cd api && go test ./... (pure tuple builders)
make test-integration   # live OpenFGA: model vs README script, internal_admin ⊇ internal_user
cd web && npm run build # tsc --noEmit + vite build
```

Restart `cmd/api` after every reseed (the store id changes). Seed accounts: ada, bob, cara, dan, eve, fay `@example.com`, password `password`.

## Architecture rules

- Layers under `api/internal`: `domain/model` (stdlib only) and `domain/gateway` (data-access interfaces) -> `usecase` (one interface per operation, e.g. `FolderCreator`) and `usecase/helper` (shared rule interfaces) -> `service` (implementations, one file per operation: `folder_creator.go`) -> `infrastructure/{postgres,fga,auth,idgen,config}` implements gateways -> `handler/http`. `domain`, `usecase`, and `service` never import Gin, pgx, or the OpenFGA SDK.
- DI everywhere: constructors take interfaces; services hold no concrete types. `api/cmd/api/wire.go` is the only place that builds the graph. Add a new operation as usecase interface + service file + constructor wiring.
- Go and Python use the same words: `domain/model` is business data in every service (Go's `domain/entity` was renamed), `entity` only names a technology's own row or wire shape.
- Python services (`agent/`, `blueprint-engine/`) follow the same Clean Architecture and name things in the business domain, not by mechanism (`scripts/check-architecture-python.sh` enforces what can be checked):
  - The Engine's delivery side already follows this (per-domain `handler/grpc/<domain>_handler.py` with their own `<Domain>Usecases`, `mapper/`, `common/*_helper.py`, one gateway per file, no layer imports `wire.py`) and the same checks run on it; its flat `domain/` and root modules are refactored in a later PR, so the layout, model, usecase and service checks run on the Agent only.
  - Layout (checked by the architecture script): `domain/{model,gateway}` + `errors.py`; `usecase/<op>.py` (a Protocol named after the file, signatures only) + `usecase/helper/`; `service/<op>.py` (`<Op>Service`) + `service/helper/` + `service/prompts/` (functions only); `infrastructure/<technology>/` + `infrastructure/entity/` (no loose modules); `handler/<transport>/`; the package root holds only `wire.py` and `serve.py`.
  - `domain/model/`: the business data models, one file per concept (`brief.py`, `plan.py`, `plan_check.py`, `design_limits.py`), stdlib only, data only (dataclasses and enums with fields and no methods, like Go's `domain/model`; a token, an exception, a factory or a callback type is not a model); every dataclass in `domain/` lives there; `domain/gateway/<collaborator>_gateway.py` holds exactly one Protocol named for what the domain asks of it (`BlueprintEngine`, `LanguageModel`; a progress sink is a plain callback, not a gateway), with business-named methods, not RPC names (`__init__.py` only re-exports, like Go's `<kind>_gateway.go`).
  - `usecase/<operation>.py` is one interface with verb methods (`BriefDrafter.draft`); `service/<operation>.py` is its implementation `<Operation>Service` and holds the business logic. Same file name in both folders.
  - Prompts are business behavior: they live in `service/prompts/` (pure functions: inputs in, text out). `infrastructure/` only adapts a technology (`ollama`, `engine_grpc`), including model-specific tuning; the technology's own wire shapes are `infrastructure/entity/<technology>_<shape>.py` (e.g. `ollama_chat.py`), not dicts in the adapter.
  - DTOs (protobuf messages) are converted only in `mapper/` folders, one `<model>_mapper.py` per domain model, each converting BOTH ways (request/DTO -> domain and domain -> DTO/request): `handler/grpc/mapper/` (the Agent's own API: decode requests, encode responses/events, errors -> status or in-band error) and `infrastructure/engine_grpc/mapper/` (build Engine requests, read Engine responses and errors). No `*_pb2` message module is imported outside `mapper/`.
  - Folder names are singular like the Go layers (`model`, `entity`, `mapper`, `gateway`; `model` is the business data in every service, Go included, and `entity` is only a technology's own row or wire shape).
  - `handler/` is the delivery layer (like Go's `handler/http`): request -> handler -> use case (implemented by a service) -> gateway (implemented by infrastructure) -> dependencies (the model, the Engine, third parties). `handler/grpc/` holds the `<domain>_handler.py` endpoint classes (one per domain with its own `<Domain>Usecases`, thin methods: decode, call ONE use case, encode through `mapper/`) and the delivery plumbing around them: a delegating `servicer.py`, `server.py` (server and health), `mapper/`, and `common/<kind>_helper.py` for what the handlers share (`streaming_helper.py` runs a streaming RPC and cancels it when the client leaves; `error_helper.py` maps domain errors to statuses). Business rules (scores, defaults, summaries) are computed in services, never in the delivery layer.
  - Dependencies are explicit: `CallContext` (request id) first argument, a progress callback parameter (`Callable[[Progress], None]`, it may raise `Cancelled`); never ambient `ContextVar`s. Only `wire.py` builds the graph and no layer imports it (a handler declares its own `<Domain>Usecases` and `wire.py` builds them, like Go); handlers decode, call one use case and encode.
  - Shape is checked on the syntax tree (`scripts/check-python-shape.py`): a public handler method has at most 15 statements, no loops and calls ONE use case (`self._uc.<field>.<method>`); a mapper function has at most 25 statements; no Agent function exceeds 60. What a script cannot judge (is it thin? is the name the domain's word?) is reviewed with the Python layering checklist in `docs/harness/evaluator-rubric.md`.
  - Common code stays in its own layer (Go: `usecase/helper`, `service/helper.go`): handler helpers in `handler/grpc/common/<kind>_helper.py`, shared business rules as an interface in `usecase/helper/<name>.py` implemented in `service/helper/<name>.py`, infrastructure helpers inside `infrastructure/`; none in `domain/`.
  - No catch-all modules (`utils`, `helpers`, `results`, a growing `__init__`).
- Services read as business logic: validate, Check via `usecase.Authorizer`, read/write via gateways, order stores via `helper.WriteGuard`. Tuple formatting lives only in `infrastructure/fga` (pure `*Relations` builders behind `*RelationGateway`); services pass `model.Tuple` through.
- Each domain is fully separate in every backend layer: folder, document, album, photo, blueprint (and user, tenant, product, member) each own their model, gateways, repos (literal table names), relation builder, usecases, services, handler, and routes. Never merge them back behind a shared `Kind`-style abstraction or a generic service/handler; keep their behavior identical unless the spec says otherwise. Shared code is only cross-domain rules in `service/helper.go` and SQL plumbing in `postgres/grant_rows.go`. The web UI still uses one `ContentPage.tsx` for documents and photos, rendered as tabs inside the project page (no top-level Documents/Photos routes); Blueprints is its own tab page (`Blueprints.tsx`) because it has no containers.
- Handlers: decode, read actor (JWT `sub`) and tenant (`X-Tenant-ID` header), call one usecase method, map errors via `writeErr`. Each handler registers its own routes (`Register(Routes)`). Tenant is never a JWT claim. A domain's JSON shapes and the model <-> DTO conversions live in `handler/http/mapper/<domain>_mapper.go` (views out, shared request bodies in), never in the handler file (`scripts/check-architecture.sh`).

## Authorization invariants (do not break)

- Every action is allowed only after the decision-catalog OpenFGA Check; collections use ListObjects then `WHERE tenant_id = $1 AND id = ANY($2)`. Never select a whole tenant and filter in Go.
- Affiliation is derived from the member's company (internal iff it is the tenant's own company); `memberships` is now `tenant_members`. Content belongs to a project; effective permissions are the union of tenant-role and project-role grants; internal members reach every project, external members need project membership; `can_create_*` live on the project. `company`, `project`, `project_role`, `project_member` are separate domains.
- Never branch on `tenant_members.role`/`affiliation` strings to permit anything (Go or React). They are display labels.
- "Platform admin OR tenant relation" is two Checks composed in the service (`Authorizer.PlatformOr`), never a model relation that makes system admins tenant members.
- Model is allow-only: no `but not`, no conditions. Products are `user:*` wildcard tuples intersected into content checks; disabling deletes only that wildcard. Custom roles are tenant-scoped `role` objects with a fixed permission catalog (`grant_*` usersets of `role#assignee`). Platform admins create them per tenant. The four built-in roles stay; custom grants are OR'd into `can_*` and still require `can_read_tenant`.
- Content `can_view` intersects `can_read_tenant from tenant` (stale grants on removed members fail closed). `can_delete`/`can_share` = `can_manage` (owner or internal admin); folder/album rename uses `can_manage`, document/photo rename uses `can_edit`.
- `internal_admin` implies `internal_user`, `external_admin` implies `external_user` via relations; never write the implied tuple. External users are never in `can_contribute`; editor grants to them are 400 `cannot_upload`.
- Visibility: internal members see and edit all content via `internal_user from tenant`; externals see a container only when it and every ancestor carry `public@tenant` (`external_visible` chain anchored by `root@tenant` on top-level containers). "Private wins" is intersection, not `but not`. External admins contribute only in externally visible content; only internal members create private containers or change visibility (`can_change_visibility`); subfolders need `can_edit` on the parent.
- Tenant administration (Members page, `GET /tenants/:id/members`, `GET /tenants/:id/products`) is internal admin (`can_manage_members` / `can_view_products`) or platform only. Toggling a product is platform only (`can_manage_platform`); tenant admins only see product status. Never reopen these to all members; features that need people use narrower endpoints (e.g. `…/share-candidates`, gated by `can_share`).
- Shares are for external members only (`user`, `external_users` viewer, `external_admins` viewer/editor). There is no `internal_users` subject.
- Inspector derives the tenant from the object, never from the request body.
- Write ordering is fail-closed: grants = Postgres then OpenFGA (compensate Postgres); revocations = OpenFGA delete then Postgres (restore *exactly* the tuples that existed). See `WriteGuard.Grant` / `WriteGuard.Revoke` in `api/internal/service/helper.go`.
- No raw tuple route; no outbox/workers.
- Blueprints: no public chain; externals reach one only via a share or a custom-role `grant_*`. Deliberate exception to "external admins contribute only in externally visible content": an external admin who is a project member may create blueprints (`can_create_blueprint = can_contribute …`); they own and see their own, nobody else's unless shared; the brief is only the creation input (the LLM drafts it, the engine lays it out and draws it into a DXF); after that a blueprint is a DXF plan whose LLM chat and viewer edits are small explicit ops the engine applies and checks (old blueprints that still have a brief keep editing through it), and the engine does all geometry. Services Check before calling the engine and call it before any write; versions are Postgres-only and guarded by `base_version` (409). The engine never sees tenant or user ids.

## OpenFGA gotchas (learned the hard way)

- Writes must send `on_duplicate: ignore` and `on_missing: ignore` (already in `Client.Write`); role changes delete non-existent tuples by design. Max 100 keys per atomic write — error, don't chunk (seed is the only chunked writer).
- Convert the DSL with `transformer.TransformDSLToJSON` and unmarshal into the SDK request. `protojson` with `UseProtoNames` produces an invalid model.
- Do not set `OPENFGA_LIST_OBJECTS_DEADLINE` above the 6s request timeout; v1.22 panics on start.
- Object ids must be real UUIDs (hex only). Seed photo ids use the `e` prefix, not `p`.
- Raw `viewer`/`editor` relations record grants, not permissions; an internal admin reaches content through `internal_admin from tenant` in `can_*`. Compare/check only `can_*` relations.
- Check/ListObjects/BatchCheck use `HIGHER_CONSISTENCY` and the pinned model id.

## Frontend permissions

- `web/src/permissions.tsx` is the only place mapping permissions to UI: `FEATURES` (feature -> rule `{ all?, any? }` over `platform:`/`tenant:` permissions; product tabs inside a project require membership AND `<product>_enabled`) and `ACTIONS` (button -> relation in a row's `permissions`). Components use `useCan`, `useAllows`, `<Can>`, `RequireFeature`, `canAct`; they never name OpenFGA relations.
- Data comes from `GET /api/v1/me/permissions?tenant_id=` and the `permissions` object on content list/get rows (one server-side BatchCheck). Deny while loading. Hiding is UX only; the API still checks.
- `/tenants/:id` is guarded against the URL tenant (`usePermissionSet(id)`), not the active tenant. Menu entries are a config array filtered by feature.
- Active tenant: `TenantSwitcher` (`GET /tenants/mine`), stored in `sessionStorage.tenant_id`, sent as `X-Tenant-ID`.

## Conventions

- Versions: Go 1.26 (current goose/x/* require it), `openfga/openfga:v1.22.0`, `go-sdk v0.8.3`, React 19.3 (Mantine 9 requires React 19.2+), Mantine 9.7.3, `@tabler/icons-react` 3.49, react-router-dom 6.30.6, Vite 8, TypeScript 5.9; engine: Python 3.13, FastAPI 0.162.5, ezdxf 1.4.5, Shapely 2.2.0, Pydantic 2.14.3, ifcopenshell 0.9.0.post1, opencv-python-headless 5.0.0.93, rapidocr-onnxruntime 1.4.4, pypdfium2 5.15.2 (exact pins in `blueprint-engine/pyproject.toml`, locked by `uv.lock`). Pin exact versions (`npm install --save-exact`).
- JSON snake_case, RFC3339; creates 201, deletes/revokes 204. GET-by-id with a failed Check is 404; writes are 403 with `{"error","check":{relation,object}}`.
- Non-goals stay out (SSO, object storage and file serving (an uploaded PDF or picture is passed to the engine and discarded; the one stored file is a blueprint's original DXF (the uploaded DXF, the DXF the engine converted from an uploaded PDF or picture, or the drawing the engine made for a created plan), a bytea row read only with `can_edit`), cross-tenant shares, pagination, ownership transfer, system_admin API). Don't scaffold them.
- UI kit is Mantine (plain CSS, no CSS-in-JS runtime). Use Mantine components and props before writing CSS; `web/src/styles.css` is only for small tweaks using `--mantine-*` variables. Don't add a second UI library.
- Shared UI helpers live in `web/src/ui.tsx`: `run(fn, successMsg)` for mutations (toast on success or error), `notifyError`, `confirmDanger`, `PageHeader`, `RoleBadge`/`AffiliationBadge` (display only). Use them instead of inline error state.
- Pages are `React.lazy` routes; vendor chunks are split in `vite.config.ts`. Keep icon imports named (`import { IconX } from "@tabler/icons-react"`) so they tree-shake.
- The user's `~/.npm` cache has root-owned files; run npm with `npm_config_cache=$(mktemp -d)` instead of sudo.

## AI-native SDLC (features, not small fixes)

`/feature <name>` runs clarify requirements -> system design -> DB design -> plan -> build -> test -> release docs. Each stage has a skill in `.claude/skills/` and writes a committed artifact in `specs/<feature>/`; a person approves each artifact before the next stage. `design-reviewer` checks artifacts, `authz-reviewer` checks code. DB changes follow the `db-design` skill (never breaking the running system; `scripts/check-migrations.sh` and a hook enforce it; committed migrations are never edited). Go work follows the `go-api` skill. How to use it: `docs/harness/using-the-harness.md`. Full map: `docs/harness/sdlc.md`.

## Session protocol

- Clock-in: first `scripts/sync-base.sh [integration-branch]` (fetch and fast-forward the branch the task's PR targets, e.g. `feat/re-architecture` or `main`; on a task branch, rebase or merge the updated base before starting), then read `docs/harness/progress.md`, run `make check`.
- One task at a time; call it done only after `make check` (and `make verify-live` for authorization changes) pass.
- After touching `service/`, `fga/`, or `model.fga`, run the `authz-reviewer` subagent (rubric: `docs/harness/evaluator-rubric.md`).
- Clock-out: walk `docs/harness/clean-state-checklist.md`, update `progress.md`.

## Git rules (`scripts/hook-git-guard.sh` enforces these)

- Commit and push only when asked. Commits are authored by the user only: no Co-Authored-By or "Generated with Claude" lines, in commits or PR bodies.
- PR descriptions follow `.github/pull_request_template.md`: Summary, Changes, Impact, Test.
- Never commit or push to `main`. Branch from the integration branch (e.g. `feat/re-architecture`) and open the PR against it.
- Temp branches (`wt-*`, `worktree-agent-*`, `working-tree-*`) and their worktrees under `.claude/worktrees/` are local only: never push them (push work as `git push origin HEAD:<task-branch>`). After the PR is created run `scripts/clean-temp-branches.sh`.
- The hook reads the command text: run commit and push as separate commands.

## Verifying a change

`make check` runs step 1 and 4 plus the architecture rules (`scripts/check-architecture.sh`); `make verify-live` runs step 2.

1. `cd api && go vet ./... && go vet -tags=integration ./... && go test ./...`
2. `make compose-up && make seed && make test-integration`
3. Restart the API and walk the README script (each step maps to a Check in `docs/decision-catalog.md` and `docs/testing.md`).
4. `cd web && npm run build`.
