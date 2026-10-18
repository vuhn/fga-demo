# API architecture

The Go service (module `github.com/vuhn/fga-demo/api`, Gin, pgx, goose). It is the only authenticated edge of the system; see the [system overview](../ARCHITECTURE.md).

## Layers

Dependencies point inward. Paths are under `internal/`.

```text
handler/http  ->  usecase (interfaces)  ->  domain/entity
service       ->  usecase, usecase/helper, domain/gateway, domain/entity
infrastructure -> domain/gateway, domain/entity
cmd/api/wire.go builds infrastructure, services, handlers and injects them
```

| Layer | Responsibility |
| --- | --- |
| `domain/entity` | Entities, value types, sentinel errors. Stdlib only. |
| `domain/gateway` | Data-access interfaces (Postgres repos, OpenFGA). |
| `usecase`, `usecase/helper` | One interface per operation (`FolderCreator`), plus shared rule interfaces (`WriteGuard`, `SharePolicy`). |
| `service` | Implementations, one file per operation (`folder_creator.go`). Business logic only. |
| `infrastructure/postgres` | pgx repos: the display record. |
| `infrastructure/fga` | OpenFGA client, model bootstrap, and pure tuple builders. The only place tuples are formatted. |
| `infrastructure/{auth,idgen,config}` | JWT and bcrypt, UUIDs, environment config. |
| `handler/http` | Gin handlers (decode, call one usecase, map errors) and middleware. |
| `seed` | The fixed demo world, shared by `cmd/seed` and the integration tests. |

Rules:

- `domain`, `usecase`, and `service` never import Gin, pgx, or the OpenFGA SDK.
- Constructors take interfaces; `cmd/api/wire.go` is the only place that names concrete types. A new operation is a usecase interface, a service file, and wiring.
- **Each domain is separate in every layer.** Folder, document, album, photo (and user, tenant, product, member) own their entity, gateways, repos, relation builder, usecases, services, handler, and routes. Do not merge them behind a shared abstraction. The only shared code is cross-domain rules in `service/helper.go` and SQL plumbing in `postgres/grant_rows.go`.

## Request lifecycle

1. Middleware: recovery and logging, CORS, JWT authentication (everything except login and `/healthz`), tenant header parsing. The middleware does not check membership.
2. The handler reads the actor (JWT `sub`) and tenant (`X-Tenant-ID`), calls one usecase method, and maps errors to HTTP statuses.
3. The service validates, then authorizes through `usecase.Authorizer` (`Require` 403, `RequireVisible` 404, `PlatformOr`, `Permissions`).
4. Reads: ListObjects, then `WHERE tenant_id = $1 AND id = ANY($2)`. Never load a whole tenant and filter in Go.
5. Writes: Postgres rows and OpenFGA tuples, ordered by `WriteGuard`.

"Platform admin OR tenant relation" is two Checks composed in the service, never a model relation that makes system admins tenant members.

## Authorization

OpenFGA holds the model ([`fga/model.fga`](fga/model.fga)) and tuples. The model has three object layers: `platform`, `tenant`, and content containers and items. Content `can_*` relations intersect tenant membership (`can_read_tenant`) and the product entitlement, so stale grants and disabled products fail closed.

On startup `cmd/api` opens the existing store and its newest model; `cmd/seed` recreates the store and writes the model. Every call pins that model id and uses `HIGHER_CONSISTENCY` for reads.

References: [model](../docs/authorization-model.md), [tuples per command](../docs/tuple-catalog.md), [required check per action](../docs/decision-catalog.md).

## Consistency between stores

There is no transaction across Postgres and OpenFGA. Ordering keeps failures safe:

- **Grant** (create, share, add member, enable product): Postgres first, then OpenFGA. If OpenFGA fails, compensate Postgres.
- **Revoke** (delete, unshare, remove member, disable product): OpenFGA first, then Postgres. If Postgres fails, write back exactly the tuples that were deleted.

`WriteGuard.Grant` and `WriteGuard.Revoke` (`service/helper.go`) own this. Tuple writes are idempotent (`on_duplicate`/`on_missing: ignore`), so retrying is safe. Details: [write path](../docs/tuple-catalog.md).

## Data

Postgres tables: users, platform_admins, tenants, memberships, tenant_products, one container and one item table per product, and `resource_grants`. Postgres ids and OpenFGA object ids are the same UUIDs (the platform object is `main`). Schema: [database schema](../docs/database-schema.md). Migrations are embedded and applied on startup.

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
