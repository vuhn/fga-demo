# API architecture

The Go service (module `github.com/vuhn/fga-demo/api`, Gin, pgx, goose). It is the only authenticated edge of the system; see the [system overview](../ARCHITECTURE.md).

## Network

```text
web :5173 --HTTP /api (Bearer JWT + X-Tenant-ID)--> API :8088
API :8088 --pgx-------------> Postgres :5432   (db fgademo: display record)
API :8088 --HTTP Check/BatchCheck/ListObjects/Write--> OpenFGA :8080
API :8088 --gRPC (x-request-id)--> Agent :8091  (draft, revise, review, estimate chat)
API :8088 --gRPC (x-request-id)--> Engine :8092 (layout, plan ops, DXF, render, takeoff)
Agent :8091 --gRPC tools--> Engine :8092
OpenFGA :8080 --SQL--> Postgres :5432          (db openfga)
```

## Layers

Dependencies point inward. Paths are under `internal/`.

```text
handler/http  ->  usecase (interfaces)  ->  domain/model
service       ->  usecase, usecase/helper, domain/gateway, domain/model
infrastructure -> domain/gateway, domain/model
cmd/api/wire.go builds infrastructure, services, handlers and injects them
```

| Layer | Responsibility |
| --- | --- |
| `domain/model` | Business models (entities), value types, sentinel errors. Stdlib only. |
| `domain/gateway` | Data-access interfaces (Postgres repos, OpenFGA). |
| `usecase`, `usecase/helper` | One interface per operation (`FolderCreator`), plus shared rule interfaces (`WriteGuard`, `SharePolicy`). |
| `service` | Implementations, one file per operation (`folder_creator.go`). Business logic only. |
| `infrastructure/postgres` | pgx repos: the display record. |
| `infrastructure/fga` | OpenFGA client, model bootstrap, and pure tuple builders. The only place tuples are formatted. |
| `infrastructure/agent` | gRPC client of the Agent (`gateway.BlueprintAgent`: `BriefFromText` unary; `Revise`, `RevisePlan`, `Review` streams; the sync variants collect the same stream), wired by `wire.go` from `AGENT_ADDR` (default `localhost:8091`). `EstimateTurn` is the same client for `gateway.EstimateAgent`: each stream event becomes the SSE event the browser already reads (a fact's text value gets its JSON type back, a question always has `options`), the Done event is the turn, and a turn is bounded by a 31 minute ceiling. A status before the first message is a plain error, an in-band `Error` event after it is mapped the same way (4xx -> `ErrValidation`, else `ErrEngineUnavailable`); a caller that leaves cancels the stream and so the Agent's run. Only `x-request-id` is sent. |
| `infrastructure/engine` | gRPC client of the deterministic Engine (`gateway.BlueprintEngine`), wired by `wire.go` from `BLUEPRINT_ENGINE_ADDR` (default `localhost:8092`). `infrastructure/grpcutil` holds what gRPC clients share: dial options (32 MB messages), the default deadline, `x-request-id` metadata, and the status / in-band error mapping (`INVALID_ARGUMENT` -> `ErrValidation`; anything else -> `ErrEngineUnavailable`). `domain`, `usecase` and `service` never import gRPC, protobuf or the generated stubs (architecture script). |
| `infrastructure/{auth,idgen,config}` | JWT and bcrypt, UUIDs, environment config. |
| `handler/http` | Gin handlers (decode, call one usecase, map errors) and middleware. |
| `handler/http/mapper/<domain>_mapper.go` | The domain's JSON DTOs and the conversions model <-> DTO (views out, shared request bodies in). Handlers never hold a `...View` function or a `...JSON` type. |
| `seed` | The fixed demo world, shared by `cmd/seed` and the integration tests. |

Rules:

- `domain`, `usecase`, and `service` never import Gin, pgx, or the OpenFGA SDK.
- Constructors take interfaces; `cmd/api/wire.go` is the only place that names concrete types. A new operation is a usecase interface, a service file, and wiring.
- **Each domain is separate in every layer.** Folder, document, album, photo, blueprint (and user, tenant, product, member, role) own their model, gateways, repos, relation builder, usecases, services, handler, and routes. Do not merge them behind a shared abstraction. The only shared code is cross-domain rules in `service/helper.go` and SQL plumbing in `postgres/grant_rows.go`. Member removal also drops that person's custom-role assignee tuples.

## Request lifecycle

1. Middleware: recovery and logging, CORS, JWT authentication (everything except login and `/healthz`), tenant header parsing. The middleware does not check membership.
2. The handler reads the actor (JWT `sub`) and tenant (`X-Tenant-ID`), calls one usecase method, and maps errors to HTTP statuses.
3. The service validates, then authorizes through `usecase.Authorizer` (`Require` 403, `RequireVisible` 404, `PlatformOr`, `Permissions`).
4. Reads: ListObjects, then `WHERE tenant_id = $1 AND id = ANY($2)`. Never load a whole tenant and filter in Go.
5. Writes: Postgres rows and OpenFGA tuples, ordered by `WriteGuard`.
6. Blueprints: the engine (LLM and geometry) is called after the Check and before any write, so a slow or failed LLM call writes nothing. Versions are Postgres rows guarded by `base_version` (409 when stale); they never touch OpenFGA. Estimate sessions follow the same order (Check `can_edit`, engine, then one write guarded by the session's turn counter) and stream the agent's events as SSE; estimates are read with `can_view`. The estimate domain has its own entity, gateways, repos, usecases, services and handler.

"Platform admin OR tenant relation" is two Checks composed in the service, never a model relation that makes system admins tenant members.

## Authorization

OpenFGA holds the model ([`fga/model.fga`](fga/model.fga)) and tuples. The model has three object layers: `platform`, `tenant` (including custom `tenant_role` objects), `project` (including `project_role` objects), and content containers and items. Content `can_*` relations intersect tenant membership (`can_read_tenant`) and the product entitlement, so stale grants and disabled products fail closed. A custom role adds `grant_*` usersets on the tenant; it does not replace the built-in affiliation relations.

On startup `cmd/api` opens the existing store and its newest model; `cmd/seed` recreates the store and writes the model. Every call pins that model id and uses `HIGHER_CONSISTENCY` for reads.

References: [model](../docs/authorization-model.md), [tuples per command](../docs/tuple-catalog.md), [required check per action](../docs/decision-catalog.md).

## Consistency between stores

There is no transaction across Postgres and OpenFGA. Ordering keeps failures safe:

- **Grant** (create, share, add member, enable product): Postgres first, then OpenFGA. If OpenFGA fails, compensate Postgres.
- **Revoke** (delete, unshare, remove member, disable product): OpenFGA first, then Postgres. If Postgres fails, write back exactly the tuples that were deleted.

`WriteGuard.Grant` and `WriteGuard.Revoke` (`service/helper.go`) own this. Tuple writes are idempotent (`on_duplicate`/`on_missing: ignore`), so retrying is safe. Details: [write path](../docs/tuple-catalog.md).

## Data

Postgres tables: users, companies, platform_admins, tenants, tenant_members, tenant_roles, projects, project_members, project_roles, tenant_products, one container and one item table per product, `blueprints` with append-only `blueprint_versions`, `estimate_sessions`, `estimate_messages`, `blueprint_estimates`, and `resource_grants`. Postgres ids and OpenFGA object ids are the same UUIDs (the platform object is `main`). Schema: [database schema](../docs/database-schema.md). Migrations are embedded and applied on startup.

## Entry points

| Command | Purpose |
| --- | --- |
| `cmd/api` | Migrate, open the OpenFGA store, serve HTTP on `:8088`. Never seeds. |
| `cmd/seed` | Migrate, truncate app tables, recreate the store and model, write the demo world. Restart `cmd/api` afterward. |

Configuration is environment variables read in `infrastructure/config`; see [operations](../docs/operations.md).

## Testing

- Unit tests cover the pure tuple builders without a server (`make test`).
- Integration tests (`-tags=integration`, live OpenFGA) assert the model against the seed world and the README script (`make test-integration`).

See [tests](../docs/testing.md). HTTP contract: [HTTP API](../docs/http-api.md). Seed world: [seed data](../docs/seed-data.md).
