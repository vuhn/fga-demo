# Architecture

A multi-tenant authorization demo. A Go API owns users, tenants, documents, and photos; [OpenFGA](https://openfga.dev) makes every allow/deny decision; a React admin panel is the clickable proof. [README.md](README.md) tells the story and the click script.

This file is the system overview. Each part has its own:

- [api/ARCHITECTURE.md](api/ARCHITECTURE.md): the Go service, its layers, and how a request is authorized.
- [web/ARCHITECTURE.md](web/ARCHITECTURE.md): the React admin panel and how it shows permissions.
- [docs/](docs/): detailed reference (schema, model, tuples, routes, seed data, tests).

## System context

```text
browser :5173  (Vite dev server)
    |  /api proxied
    v
Go API :8088
    |                    \
    v                     v
Postgres :5432          OpenFGA :8080  (gRPC :8081, Playground :3000)
  db fgademo              store "fga-demo"
  db openfga  <-----------'  (OpenFGA's own datastore)
```

| Piece | Role |
| --- | --- |
| Web (`web/`) | Admin panel. Renders only what the API says the actor may do. |
| API (`api/`) | The only authenticated edge. Validates, asks OpenFGA, reads and writes Postgres. |
| Postgres `fgademo` | Display record: names, membership labels, file metadata, grant rows. Never decides access. |
| OpenFGA | Relationship tuples. Answers Check, BatchCheck, ListObjects. Allow-only model. |
| `deploy/` | Docker Compose for Postgres and OpenFGA. The API and Vite run on the host. |

## Core principles

1. **OpenFGA decides.** An action is allowed only after the OpenFGA check named in the [decision catalog](docs/decision-catalog.md) returns true. Collections use ListObjects, then a Postgres query for those ids only.
2. **Postgres only displays.** `memberships.role`, `affiliation`, and `platform_admins` choose which tuple to write and which label to show. No code, Go or React, branches on them to permit anything.
3. **Identity is a user id, tenancy is a header.** The JWT `sub` is the user; the active tenant is `X-Tenant-ID`, because one person belongs to many tenants.
4. **Allow-only model.** No `but not` or conditions. "Private wins" is an intersection over the ancestor chain; entitlements are `user:*` wildcard tuples intersected into content checks. Custom roles are tenant-scoped `role` objects: a platform admin assigns permissions from a fixed catalog, and those grants are `role#assignee` usersets OR'd into the existing `can_*` relations. The four built-in affiliation roles stay.
5. **Fail closed.** Grants write Postgres then OpenFGA; revocations delete in OpenFGA then Postgres. A crash leaves less access, never more. No outbox, no workers.
6. **The UI is a mirror.** The API computes permissions with Check and returns them; the UI hides controls from that. Hiding is UX only; the API always checks.

## Authorization at a glance

Three layers of objects: `platform` (system admins), `tenant` (internal/external roles, product entitlements), and content (`folder`/`document`, `album`/`photo`) that inherits through containers.

- Internal members see and edit all tenant content. External members see a container only when it and every ancestor are public, plus anything explicitly shared with them.
- The system admin manages tenants, users, products, and memberships but is not a tenant member and sees no content.
- Details: [authorization model](docs/authorization-model.md), [tuple catalog](docs/tuple-catalog.md).

## Request flow

```text
web  --Bearer JWT + X-Tenant-ID-->  handler  -->  usecase/service
                                                   |-- Authorizer.Check / ListObjects  --> OpenFGA
                                                   |-- repos (display rows)            --> Postgres
                                                   `-- WriteGuard (ordered tuple writes)
```

## Scope

In scope: ReBAC across platform, tenant, and resource layers; affiliation-based defaults; private/public containers; direct shares to external members; product entitlements; an admin panel that performs the README script.

Out of scope (do not scaffold): SSO or password flows beyond login, file bytes or object storage, cross-tenant shares, conditions or `but not`, an API to grant `system_admin`, deleting users or tenants, outbox or background workers, pagination, ownership transfer.

## Repository layout

```text
api/        Go service            -> api/ARCHITECTURE.md
web/        React admin panel     -> web/ARCHITECTURE.md
deploy/     Docker Compose, Postgres init
docs/       Detailed reference
Makefile    compose-up, seed, api, web, test, test-integration
go.work     Lets the language server resolve api/ from the repo root
```

## Reference docs

| Doc | Contents |
| --- | --- |
| [docs/operations.md](docs/operations.md) | Runtime, Compose, environment variables |
| [docs/database-schema.md](docs/database-schema.md) | Postgres DDL |
| [docs/authorization-model.md](docs/authorization-model.md) | OpenFGA DSL, id format, store bootstrap |
| [docs/tuple-catalog.md](docs/tuple-catalog.md) | Tuples per command, write ordering and compensation |
| [docs/decision-catalog.md](docs/decision-catalog.md) | Required Check per action |
| [docs/http-api.md](docs/http-api.md) | Routes, errors, payloads |
| [docs/seed-data.md](docs/seed-data.md) | The fixed demo world |
| [docs/web-ui.md](docs/web-ui.md) | Page behavior and UI permission mapping |
| [docs/testing.md](docs/testing.md) | What the tests assert |
