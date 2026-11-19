---
name: go-api
description: Implement or change Go API code in api/ (Gin, pgx, OpenFGA, goose) following the repo's layered architecture and authorization invariants. Use for any task touching api/internal, api/cmd, or an HTTP route.
---

# Implement Go API changes

Source of truth: `api/ARCHITECTURE.md` (layers, lifecycle, consistency) and CLAUDE.md "Architecture rules" + "Authorization invariants". Read them when unsure; this skill is the working checklist.

## Layers (dependencies point inward)
`domain/entity` (stdlib only) and `domain/gateway` (interfaces) -> `usecase` (one interface per operation, e.g. `FolderCreator`) + `usecase/helper` -> `service` (one file per operation, `folder_creator.go`) -> `infrastructure/{postgres,fga,blueprintengine,auth,idgen,config}` implements gateways -> `handler/http`.
`domain`, `usecase`, `service` never import Gin, pgx, or the OpenFGA SDK (`scripts/check-architecture.sh` enforces).

## Adding an operation: touch exactly these
1. Spec first: `docs/decision-catalog.md` (the Check), `docs/http-api.md` (route), plus `tuple-catalog`/`authorization-model` if tuples or model change.
2. `domain/entity` types/errors; `domain/gateway` method if new data access.
3. `usecase/<domain>_<op>.go`: one interface.
4. `service/<domain>_<op>.go`: validate -> authorize via `usecase.Authorizer` (`Require` 403, `RequireVisible` 404, `PlatformOr`, `Permissions`) -> gateways -> `helper.WriteGuard` for stores. No tuple strings here; pass `entity.Tuple`.
5. `infrastructure/postgres/<domain>_repo.go` (literal table names) and `infrastructure/fga/<domain>_relations.go` (pure `*Relations` builders, unit-tested); migration via `db-design` if schema changes.
6. `handler/http/<domain>_handler.go`: decode, read actor (JWT `sub`) and tenant (`X-Tenant-ID`), call ONE usecase method, map errors with `writeErr`; register in the handler's own `Register(Routes)`.
7. `cmd/api/wire.go`: the only place that builds the graph; constructors take interfaces.
8. Tests: service unit tests with fakes, relation builder tests; integration test (`-tags=integration`) when authorization changes. Web: `web/src/permissions.tsx` only if a new action/feature appears.

## Rules that bite
- Domains stay separate in every layer (folder, document, album, photo, blueprint, estimate...); never add a `Kind` abstraction or generic service. Shared code only in `service/helper.go` and `postgres/grant_rows.go`.
- Every action needs its decision-catalog Check before any work; collections: ListObjects then `WHERE tenant_id = $1 AND id = ANY($2)`, never filter a tenant in Go.
- Never branch on `tenant_members.role`/`affiliation`. "Platform admin OR tenant relation" = `Authorizer.PlatformOr` (two Checks).
- Write order is fail-closed: grants Postgres then OpenFGA (compensate Postgres); revokes OpenFGA delete then Postgres (restore exactly the deleted tuples) via `WriteGuard.Grant/Revoke`. Max 100 tuples per atomic write.
- Engine calls (blueprints/estimates): Check, then engine, then one write; never send tenant/user ids to the engine; versions guarded by `base_version` (409).
- Statuses: creates 201, deletes/revokes 204, failed GET-by-id Check 404, failed write 403 with `{"error","check":{relation,object}}`. JSON snake_case, RFC3339. Tenant is never a JWT claim.
- No raw tuple route, outbox, workers, pagination, or other CLAUDE.md non-goals.

## Done means
`make check` passes; `make verify-live` too if authorization changed; run `authz-reviewer` after touching `service/`, `fga/`, `model.fga`; docs and `ARCHITECTURE.md` aligned; `api/fga/model.fga` equals the DSL in `docs/authorization-model.md`; restart `cmd/api` after any reseed.
