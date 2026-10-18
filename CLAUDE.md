# CLAUDE.md

OpenFGA multi-tenant authorization demo: Go API (Gin, pgx, goose) + OpenFGA + React admin panel. Postgres is the display record; OpenFGA makes every allow/deny decision.

## Source of truth

- `ARCHITECTURE.md` is the implementation contract (model, tuple catalog, decision catalog, routes, seed, tests). `README.md` is the story and the 11-step click script. **Keep code, `ARCHITECTURE.md`, and `README.md` aligned in the same change.** If behavior changes, update the spec first.
- `fga/model.fga` must equal the DSL block in `ARCHITECTURE.md` exactly.
- Seed ids/tuples (with each container's private/public flag) live once in `internal/seed/world.go`; `cmd/seed` writes them and the integration tests assert against them. Do not duplicate ids elsewhere.

## Commands

```bash
make compose-up         # Postgres :5432, OpenFGA :8080 (gRPC :8081, Playground :3000)
make seed               # RESETS everything: truncates app tables, recreates the fga-demo store, writes the seed
make api                # :8088 — migrates, opens existing store + newest model; never seeds
make web                # Vite :5173, proxies /api -> :8088
make test               # go test ./... (pure tuple builders)
make test-integration   # live OpenFGA: model vs README script, internal_admin ⊇ internal_user
cd web && npm run build # tsc --noEmit + vite build
```

Restart `cmd/api` after every reseed (the store id changes). Seed accounts: ada, bob, cara, dan, eve, fay `@example.com`, password `password`.

## Architecture rules

- Dependency direction: `cmd -> usecase -> domain`, adapters implement usecase ports (`internal/usecase/ports.go`). `domain` imports stdlib only; `usecase` never imports Gin, pgx, or the OpenFGA SDK.
- Tuple builders are pure (`internal/adapter/openfga/tuples.go`, `Catalog`) and reached via the `TupleCatalog` port.
- Documents and photos share one code path parameterized by `domain.Kind` (`usecase/content.go`, `http/content_handler.go`, `web/src/components/ContentPage.tsx`). Never fork them.
- Handlers: decode, read actor (JWT `sub`) and tenant (`X-Tenant-ID` header), call one usecase method, map errors via `writeErr`. Tenant is never a JWT claim.

## Authorization invariants (do not break)

- Every action is allowed only after the decision-catalog OpenFGA Check; collections use ListObjects then `WHERE tenant_id = $1 AND id = ANY($2)`. Never select a whole tenant and filter in Go.
- Never branch on `memberships.role`/`affiliation` strings to permit anything (Go or React). They are display labels.
- "Platform admin OR tenant relation" is two Checks composed in the usecase (`platformOr`), never a model relation that makes system admins tenant members.
- Model is allow-only: no `but not`, no conditions, no custom roles. Products are `user:*` wildcard tuples intersected into content checks; disabling deletes only that wildcard.
- Content `can_view` intersects `can_read_tenant from tenant` (stale grants on removed members fail closed). `can_delete`/`can_share` = `can_manage` (owner or internal admin); folder/album rename uses `can_manage`, document/photo rename uses `can_edit`.
- `internal_admin` implies `internal_user`, `external_admin` implies `external_user` via relations; never write the implied tuple. External users are never in `can_contribute`; editor grants to them are 400 `cannot_upload`.
- Visibility: internal members see and edit all content via `internal_user from tenant`; externals see a container only when it and every ancestor carry `public@tenant` (`external_visible` chain anchored by `root@tenant` on top-level containers). "Private wins" is intersection, not `but not`. External admins contribute only in externally visible content; only internal members create private containers or change visibility (`can_change_visibility`); subfolders need `can_edit` on the parent.
- Tenant administration (Members page, `GET /tenants/:id/members`, `GET /tenants/:id/products`) is internal admin (`can_manage_members` / `can_view_products`) or platform only. Toggling a product is platform only (`can_manage_platform`); tenant admins only see product status. Never reopen these to all members; features that need people use narrower endpoints (e.g. `…/share-candidates`, gated by `can_share`).
- Shares are for external members only (`user`, `external_users` viewer, `external_admins` viewer/editor). There is no `internal_users` subject.
- Inspector derives the tenant from the object, never from the request body.
- Write ordering is fail-closed: grants = Postgres then OpenFGA (compensate Postgres); revocations = OpenFGA delete then Postgres (restore *exactly* the tuples that existed). See `App.grant` / `App.revoke`.
- No raw tuple route; no outbox/workers.

## OpenFGA gotchas (learned the hard way)

- Writes must send `on_duplicate: ignore` and `on_missing: ignore` (already in `Client.Write`); role changes delete non-existent tuples by design. Max 100 keys per atomic write — error, don't chunk (seed is the only chunked writer).
- Convert the DSL with `transformer.TransformDSLToJSON` and unmarshal into the SDK request. `protojson` with `UseProtoNames` produces an invalid model.
- Do not set `OPENFGA_LIST_OBJECTS_DEADLINE` above the 6s request timeout; v1.22 panics on start.
- Object ids must be real UUIDs (hex only). Seed photo ids use the `e` prefix, not `p`.
- Raw `viewer`/`editor` relations record grants, not permissions; an internal admin reaches content through `internal_admin from tenant` in `can_*`. Compare/check only `can_*` relations.
- Check/ListObjects/BatchCheck use `HIGHER_CONSISTENCY` and the pinned model id.

## Frontend permissions

- `web/src/permissions.tsx` is the only place mapping permissions to UI: `FEATURES` (feature -> rule `{ all?, any? }` over `platform:`/`tenant:` permissions; product menus require membership AND `<product>_enabled`) and `ACTIONS` (button -> relation in a row's `permissions`). Components use `useCan`, `useAllows`, `<Can>`, `RequireFeature`, `canAct`; they never name OpenFGA relations.
- Data comes from `GET /api/v1/me/permissions?tenant_id=` and the `permissions` object on content list/get rows (one server-side BatchCheck). Deny while loading. Hiding is UX only; the API still checks.
- `/tenants/:id` is guarded against the URL tenant (`usePermissionSet(id)`), not the active tenant. Menu entries are a config array filtered by feature.
- Active tenant: `TenantSwitcher` (`GET /tenants/mine`), stored in `sessionStorage.tenant_id`, sent as `X-Tenant-ID`.

## Conventions

- Versions: Go 1.26 (current goose/x/* require it), `openfga/openfga:v1.22.0`, `go-sdk v0.8.3`, React 19.3 (Mantine 9 requires React 19.2+), Mantine 9.7.3, `@tabler/icons-react` 3.49, react-router-dom 6.30.6, Vite 8, TypeScript 5.9. Pin exact versions (`npm install --save-exact`).
- JSON snake_case, RFC3339; creates 201, deletes/revokes 204. GET-by-id with a failed Check is 404; writes are 403 with `{"error","check":{relation,object}}`.
- Non-goals stay out (SSO, file bytes, cross-tenant shares, pagination, ownership transfer, system_admin API). Don't scaffold them.
- UI kit is Mantine (plain CSS, no CSS-in-JS runtime). Use Mantine components and props before writing CSS; `web/src/styles.css` is only for small tweaks using `--mantine-*` variables. Don't add a second UI library.
- Shared UI helpers live in `web/src/ui.tsx`: `run(fn, successMsg)` for mutations (toast on success or error), `notifyError`, `confirmDanger`, `PageHeader`, `RoleBadge`/`AffiliationBadge` (display only). Use them instead of inline error state.
- Pages are `React.lazy` routes; vendor chunks are split in `vite.config.ts`. Keep icon imports named (`import { IconX } from "@tabler/icons-react"`) so they tree-shake.
- The user's `~/.npm` cache has root-owned files; run npm with `npm_config_cache=$(mktemp -d)` instead of sudo.

## Verifying a change

1. `go vet ./... && go test ./...`
2. `make compose-up && make seed && make test-integration`
3. Restart the API and walk the README script (each step maps to a Check in `ARCHITECTURE.md`'s test tables).
4. `cd web && npm run build`.
