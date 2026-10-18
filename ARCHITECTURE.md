# Architecture

This document is the implementation contract for the OpenFGA multi-tenant demo. Implement it literally. A later agent should be able to generate the database, the Go API, the OpenFGA model, the seed, and the React admin panel from this file alone.

The human-facing story and click script live in [README.md](README.md). Every step in that script has a tuple, a Check, and a route here. Keep the two files aligned.

If a behavior is not specified, pick the simplest behavior that preserves the invariants below. Do not add products, roles, identity providers, or authorization paths.

## Invariants

1. A mutating or reading API allows the action only after the OpenFGA check named in the decision catalog returns true. `ListObjects` is the check for collection reads.
2. Postgres `memberships.affiliation`, `memberships.role`, and `platform_admins` choose which tuple to write and what label to display. They do not allow or deny an action. Affiliation filtering is a display query after the membership Check.
3. The JWT `sub` claim is the user id. The active tenant is the `X-Tenant-ID` header. One user has many tenants, so the tenant is not a claim.
4. OpenFGA is allow-only. The model uses `or` and `and`. It does not use `but not`.
5. Disabling a product deletes one wildcard tuple. Content tuples stay. Enabling the product writes that wildcard again.
6. Removing a member deletes that member's direct role tuple and direct resource tuples. Group share tuples (`tenant:<id>#external_user`, `tenant:<id>#external_admin`) and `public`/`root` markers stay.
7. No HTTP route accepts a raw tuple write. Domain commands in the tuple catalog are the only writers.
8. Partial failures fail closed. Grants write Postgres then OpenFGA. Revocations delete in OpenFGA then Postgres.
9. Handlers, middleware, and React code do not branch on a role or affiliation string to permit an action. The UI reads permissions the API computed with Check (`GET /me/permissions` and the `permissions` object on content rows) and hides controls. The API still checks. The API rejects an internal role on an external member, and an external role on an internal member, before it writes a tuple.

## Goals

- Show ReBAC across platform, tenant, and resource layers.
- Show tenant-wide defaults by affiliation (internal members see everything; external members see public content), a private/public flag where any private ancestor wins, a direct share into private content, and an entitlement intersection.
- Show internal and external member lists, with roles that cannot cross affiliation.
- Give an admin panel that can perform the README script.

## Non-goals

Leave these out. Do not scaffold them "for later".

- SSO, OAuth, social login, refresh tokens, email verification, password reset
- Binary upload, S3, or local file storage. Documents and photos are metadata: name, mime type, byte size
- Cross-tenant shares
- Custom roles, conditions, and OpenFGA exclusion (`but not`)
- An API to grant or revoke `system_admin` after seed
- Deleting users or tenants
- An outbox, a message bus, or background tuple workers
- Pagination, search ranking, and multi-model modules
- Transfer of ownership through the grant API

## Stack

| Piece | Choice |
| --- | --- |
| Module | `github.com/vuhn/fga-demo/api` (its own `go.mod` in `api/`) |
| Go | 1.26 |
| HTTP | Gin |
| Postgres driver | `github.com/jackc/pgx/v5` |
| Migrations | goose v3, SQL files, embedded with `go:embed`, applied on `cmd/api` and `cmd/seed` startup |
| Authz server | `openfga/openfga:v1.22.0` |
| Authz SDK | `github.com/openfga/go-sdk@v0.8.3` |
| DSL transform | `github.com/openfga/language/pkg/go` `TransformDSLToProto` or the SDK-supported DSL equivalent. Do not hand-translate the model to JSON |
| Passwords | `golang.org/x/crypto/bcrypt`, cost `bcrypt.DefaultCost` |
| JWT | `github.com/golang-jwt/jwt/v5`, HMAC-SHA256 |
| Logging | `log/slog` |
| Config | `os.Getenv` in `api/internal/infrastructure/config`. No Viper |
| UI | Vite, React 19, TypeScript, `react-router-dom` 6, Mantine 9 (`@mantine/core`, `hooks`, `notifications`, `modals`) with `@tabler/icons-react`. One app stylesheet for small tweaks |
| App database | Postgres 16 database `fgademo` |
| OpenFGA database | Postgres 16 database `openfga` on the same server |

## Runtime

```text
browser :5173
    |  /api  proxied to :8088
    v
Gin API :8088
    |                 \
    |                  \
Postgres :5432         OpenFGA HTTP :8080
  db fgademo             gRPC :8081
  db openfga             playground :3000
```

`deploy/docker-compose.yml` runs Postgres, the OpenFGA migrate one-shot, and OpenFGA. The API and Vite run on the host so rebuilds stay fast.

Postgres healthcheck uses `pg_isready`. The migrate service `depends_on` Postgres with `condition: service_healthy` and runs:

```text
migrate
```

OpenFGA `depends_on` the migrate service completing successfully.

Environment for migrate and OpenFGA:

```text
OPENFGA_DATASTORE_ENGINE=postgres
OPENFGA_DATASTORE_URI=postgres://openfga:password@postgres:5432/openfga?sslmode=disable
OPENFGA_PLAYGROUND_ENABLED=true
```

Leave the ListObjects deadline at its default. OpenFGA v1.22 refuses to start when `OPENFGA_LIST_OBJECTS_DEADLINE` exceeds the request timeout (6s by default).

Do not set `OPENFGA_AUTHN_METHOD`. Local demo traffic is unauthenticated at the OpenFGA boundary. The application API is the authenticated edge.

Published ports: Postgres `5432`, OpenFGA HTTP `8080`, gRPC `8081`, playground `3000`.

`deploy/postgres/init.sql` runs on first Postgres start:

```sql
CREATE USER fgademo WITH PASSWORD 'password';
CREATE DATABASE fgademo OWNER fgademo;
CREATE USER openfga WITH PASSWORD 'password';
CREATE DATABASE openfga OWNER openfga;
```

The compose `POSTGRES_USER` is `postgres` and `POSTGRES_PASSWORD` is `password`. That superuser creates the two databases above.

## Configuration

`.env.example` contains:

```text
HTTP_ADDR=:8088
DATABASE_URL=postgres://fgademo:password@localhost:5432/fgademo?sslmode=disable
OPENFGA_API_URL=http://localhost:8080
OPENFGA_STORE_NAME=fga-demo
OPENFGA_MODEL_PATH=fga/model.fga
JWT_SECRET=dev-secret-change-me
JWT_TTL=12h
CORS_ORIGIN=http://localhost:5173
```

`cmd/api` and `cmd/seed` run from `api/` and load the first `.env` they find (`api/.env`, then the repository root `.env`), without overriding variables already set in the process. A missing `.env` falls back to the defaults above. `OPENFGA_MODEL_PATH` is relative to `api/`.

`.gitignore` includes `.env`, `web/node_modules`, and `web/dist`.

## Repository layout

All Go code lives in `api/`. A root `go.work` includes that module so the
language server resolves dependencies when the repository root is the
workspace. One content type is one file per layer; the listing shows
folders in full and the pattern for the others.

```text
go.work
api/go.mod
api/cmd/api/main.go                      # config, Postgres pool, OpenFGA store, serve
api/cmd/api/wire.go                      # dependency injection: gateways -> services -> handlers
api/cmd/seed/main.go
api/internal/domain/entity/              # entities and value types, stdlib only
    user.go tenant.go product.go membership.go grant.go authz.go errors.go validate.go
    folder.go document.go album.go photo.go
api/internal/domain/gateway/             # data-access interfaces
    authz_gateway.go                     # OpenFGA: Check, BatchCheck, ListObjectIDs, ReadTuples, Write
    user_gateway.go tenant_gateway.go product_gateway.go membership_gateway.go credential_gateway.go
    folder_gateway.go                    # FolderGateway, FolderGrantGateway, FolderRelationGateway
    document_gateway.go album_gateway.go photo_gateway.go
api/internal/usecase/                    # one interface per operation
    authorizer.go auth.go user.go tenant.go product.go member.go permission.go inspector.go
    folder.go                            # FolderLister, FolderGetter, FolderCreator, FolderRenamer,
                                         # FolderVisibilitySetter, FolderDeleter, FolderGrantLister,
                                         # FolderShareCandidateLister, FolderSharer, FolderShareRevoker
    document.go album.go photo.go
api/internal/usecase/helper/             # shared business-rule interfaces
    write_guard.go share_policy.go object_tenant.go
api/internal/service/                    # usecase implementations
    authorizer.go                        # usecase.Authorizer over gateway.AuthzGateway
    helper.go                            # WriteGuard (fail-closed ordering), SharePolicy, withPerms
    object_tenant_resolver.go
    folder_lister.go folder_getter.go folder_creator.go folder_renamer.go folder_visibility_setter.go
    folder_deleter.go folder_grant_lister.go folder_share_candidate_lister.go folder_sharer.go
    folder_share_revoker.go folder_helper.go
    document_*.go album_*.go photo_*.go
    authenticator.go token_verifier.go profile_getter.go permission_reader.go inspector.go access_reporter.go
    user_*.go tenant_*.go my_tenant_lister.go product_*.go member_*.go
api/internal/infrastructure/postgres/    # display-record gateways (pgx)
    db.go grant_rows.go user_repo.go tenant_repo.go product_repo.go membership_repo.go
    folder_repo.go folder_grant_repo.go document_repo.go document_grant_repo.go
    album_repo.go album_grant_repo.go photo_repo.go photo_grant_repo.go
api/internal/infrastructure/fga/         # OpenFGA gateways
    client.go bootstrap.go tuples.go
    membership_relations.go product_relations.go
    folder_relations.go document_relations.go album_relations.go photo_relations.go
    tuples_test.go model_integration_test.go role_integration_test.go
api/internal/infrastructure/auth/jwt.go
api/internal/infrastructure/auth/password.go
api/internal/infrastructure/idgen/uuid.go
api/internal/infrastructure/config/config.go
api/internal/handler/http/               # Gin
    router.go middleware.go views.go
    auth_handler.go permission_handler.go user_handler.go tenant_handler.go product_handler.go
    member_handler.go inspector_handler.go access_handler.go
    folder_handler.go document_handler.go album_handler.go photo_handler.go
api/internal/seed/world.go
api/migrations/00001_init.sql
api/migrations/00002_container_privacy.sql
api/migrations/embed.go
api/fga/model.fga
deploy/docker-compose.yml
deploy/postgres/init.sql
web/package.json
web/vite.config.ts
web/tsconfig.json
web/index.html
web/src/main.tsx
web/src/App.tsx
web/src/api.ts
web/src/auth.tsx
web/src/ui.tsx
web/public/favicon.svg
web/src/permissions.tsx
web/src/styles.css
web/src/pages/Login.tsx
web/src/pages/Home.tsx
web/src/pages/Users.tsx
web/src/pages/Tenants.tsx
web/src/pages/TenantDetail.tsx
web/src/pages/Documents.tsx
web/src/pages/Photos.tsx
web/src/pages/Inspector.tsx
web/src/components/TenantSwitcher.tsx
web/src/components/ShareDialog.tsx
web/src/components/ContentPage.tsx
Makefile
.env.example
.gitignore
```

## Clean architecture

Dependency direction (paths under `api/internal`):

```text
handler/http  ->  usecase (interfaces)  ->  domain/entity
service       ->  usecase, usecase/helper, domain/gateway, domain/entity
infrastructure -> domain/gateway, domain/entity
cmd/api/wire.go constructs infrastructure, services, and handlers and injects them
```

- `domain/entity` imports only the standard library. `domain/gateway` and `usecase` import only `domain`. None of them, nor `service`, imports Gin, pgx, or the OpenFGA SDK.
- Every dependency is injected through a constructor (`NewFolderCreator(folders, relations, authz, guard, ids)`); services hold only interfaces. `cmd/api/wire.go` is the only place that names concrete types. `cmd/seed` uses the Postgres repos and the pure FGA relation builders directly.
- A service reads as business logic: validate, Check through `usecase.Authorizer`, read and write through gateways, and order the two stores through `helper.WriteGuard`. Tuple formatting lives only in `infrastructure/fga`; services pass `entity.Tuple` values through untouched.
- **Each domain is separate in every layer.** Folder, document, album, and photo each have their own entity, gateway interfaces, Postgres repo (fixed table names), grant repo, FGA relation builder, usecase interfaces, services, handler, and routes, so any one can change without touching the others. Today they follow the same rules on purpose; the API contract for documents and photos is identical. Shared code is limited to cross-domain rules in `service/helper.go` (write ordering, share policy) and low-level SQL plumbing (`postgres/grant_rows.go`). `web/src/components/ContentPage.tsx` still renders both products with one component.
- `api/internal/seed/world.go` holds the fixed ids, rows, and tuples. `cmd/seed` writes them and the integration test asserts against them, so the two cannot disagree.
- HTTP handlers decode JSON, read the authenticated user and tenant header, call one usecase method, and write the status codes from the error catalog. Each handler registers its own routes; `router.go` only mounts the injected handlers.

## Identity and request context

Login accepts email and password, compares bcrypt, and returns a JWT.

JWT claims:

- `sub`: user UUID string
- `exp`: now plus `JWT_TTL`
- signing algorithm: HS256
- secret: `JWT_SECRET`

No tenant claim, no role claim.

Middleware, in order:

1. Recovery and request logging
2. CORS allowing `CORS_ORIGIN` with headers `Authorization` and `X-Tenant-ID`
3. Authentication on every route except `POST /api/v1/auth/login` and `GET /healthz`. Missing or invalid token returns 401.
4. Optional tenant parsing. Content routes require `X-Tenant-ID` to be a UUID and return 400 when it is missing or malformed. Middleware stores the header. It does not check membership.

The service receives `actorID` and, for content routes, `tenantID`.

## Postgres schema

Postgres is the display record and the membership label. Authorization reads do not use `memberships.role`.

Apply this as `api/migrations/00001_init.sql` with goose `-- +goose Up` and a matching Down that drops tables in reverse dependency order.

```sql
CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE platform_admins (
    user_id    uuid PRIMARY KEY REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenants (
    id         uuid PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    affiliation text NOT NULL CHECK (affiliation IN ('internal', 'external')),
    role        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id),
    CHECK (
        (affiliation = 'internal' AND role IN ('internal_admin', 'internal_user'))
        OR (affiliation = 'external' AND role IN ('external_admin', 'external_user'))
    )
);
CREATE INDEX memberships_tenant_affiliation_idx
    ON memberships (tenant_id, affiliation);

CREATE TABLE tenant_products (
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    product    text NOT NULL CHECK (product IN ('documents', 'photos')),
    enabled    boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, product)
);

CREATE TABLE folders (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    parent_id  uuid REFERENCES folders (id) ON DELETE RESTRICT,
    name       text NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX folders_tenant_id_idx ON folders (tenant_id);
CREATE INDEX folders_parent_id_idx ON folders (parent_id);

CREATE TABLE documents (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    folder_id  uuid NOT NULL REFERENCES folders (id) ON DELETE RESTRICT,
    name       text NOT NULL,
    mime_type  text NOT NULL,
    byte_size  bigint NOT NULL CHECK (byte_size >= 0),
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX documents_tenant_id_idx ON documents (tenant_id);
CREATE INDEX documents_folder_id_idx ON documents (folder_id);

CREATE TABLE albums (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    parent_id  uuid REFERENCES albums (id) ON DELETE RESTRICT,
    name       text NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX albums_tenant_id_idx ON albums (tenant_id);
CREATE INDEX albums_parent_id_idx ON albums (parent_id);

CREATE TABLE photos (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    album_id   uuid NOT NULL REFERENCES albums (id) ON DELETE RESTRICT,
    name       text NOT NULL,
    mime_type  text NOT NULL,
    byte_size  bigint NOT NULL CHECK (byte_size >= 0),
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX photos_tenant_id_idx ON photos (tenant_id);
CREATE INDEX photos_album_id_idx ON photos (album_id);

CREATE TABLE resource_grants (
    id           uuid PRIMARY KEY,
    tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    object_type  text NOT NULL CHECK (object_type IN ('folder', 'document', 'album', 'photo')),
    object_id    uuid NOT NULL,
    relation     text NOT NULL CHECK (relation IN ('owner', 'editor', 'viewer')),
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'internal_users', 'external_users', 'external_admins')),
    user_id      uuid REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (subject_type = 'user' AND user_id IS NOT NULL)
        OR (subject_type = 'internal_users' AND user_id IS NULL AND relation IN ('viewer', 'editor'))
        OR (subject_type = 'external_users' AND user_id IS NULL AND relation = 'viewer')
        OR (subject_type = 'external_admins' AND user_id IS NULL AND relation IN ('viewer', 'editor'))
    )
);
CREATE UNIQUE INDEX resource_grants_user_uniq
    ON resource_grants (object_type, object_id, user_id)
    WHERE subject_type = 'user';
CREATE UNIQUE INDEX resource_grants_userset_uniq
    ON resource_grants (object_type, object_id, subject_type)
    WHERE subject_type <> 'user';
CREATE INDEX resource_grants_tenant_user_idx
    ON resource_grants (tenant_id, user_id);
```

`api/migrations/00002_container_privacy.sql` then adds `is_private boolean NOT NULL DEFAULT true` to `folders` and `albums`, and narrows `resource_grants.subject_type` to `user`, `external_users`, and `external_admins` (the `internal_users` group share is gone: internal members already see all content).

`resource_grants` includes owners. A group share cannot use `user_id`. `external_users` maps to `tenant:<id>#external_user` and is viewer only, because an external user cannot upload. `external_admins` maps to `tenant:<id>#external_admin`. One group row per object and subject type; changing viewer to editor replaces that row.

`is_private` is the display flag. The `public` tuple is the authorization record (see the tuple catalog).

`created_by` is a display field. The owner tuple is the authorization record. They can diverge after a member is removed: the row keeps `created_by`, and the owner tuple is deleted.

There is no foreign key from `resource_grants.object_id` to a single table because the object type varies. The content repo writes the owner grant in the same transaction as the object and deletes grants when the object is deleted.

## Identifiers

Postgres primary keys and OpenFGA object ids are the same UUID strings, except the platform object id, which is the literal `main`.

OpenFGA object format is `<type>:<id>`. User format is `user:<uuid>`. The wildcard is `user:*`. Group-share usersets are `tenant:<uuid>#external_user` and `tenant:<uuid>#external_admin`. A container's `tenant`, `root`, and `public` relations point at the plain `tenant:<uuid>` object.

The fga infrastructure adds those prefixes. Services pass bare UUIDs, the platform id `main`, the relation name, and the object type.

Generate ids with UUIDv4, except the seed section, which uses fixed ids.

## OpenFGA model

Write this text to `api/fga/model.fga` exactly.

```fga
model
  schema 1.1

type user

type platform
  relations
    define system_admin: [user]
    define can_manage_platform: system_admin

type tenant
  relations
    define internal_admin: [user]
    define internal_user: [user] or internal_admin
    define external_admin: [user]
    define external_user: [user] or external_admin
    define documents_enabled: [user:*]
    define photos_enabled: [user:*]
    define can_manage_members: internal_admin
    define can_view_products: internal_admin
    define can_read_tenant: internal_user or external_user
    define can_contribute: internal_user or external_admin
    define can_create_folder: can_contribute and documents_enabled
    define can_create_album: can_contribute and photos_enabled
    define can_create_private_folder: internal_user and documents_enabled
    define can_create_private_album: internal_user and photos_enabled

type folder
  relations
    define tenant: [tenant]
    define parent: [folder]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible) and can_read_tenant from tenant and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and external_admin from tenant)) and can_contribute from tenant and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: can_manage and internal_user from tenant
    define can_create_document: can_edit

type document
  relations
    define tenant: [tenant]
    define folder: [folder]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from folder
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from folder
    define can_view: (internal_user from tenant or viewer or external_visible from folder) and can_read_tenant from tenant and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from folder and external_admin from tenant)) and can_contribute from tenant and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage

type album
  relations
    define tenant: [tenant]
    define parent: [album]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible) and can_read_tenant from tenant and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and external_admin from tenant)) and can_contribute from tenant and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: can_manage and internal_user from tenant
    define can_create_photo: can_edit

type photo
  relations
    define tenant: [tenant]
    define album: [album]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from album
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from album
    define can_view: (internal_user from tenant or viewer or external_visible from album) and can_read_tenant from tenant and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from album and external_admin from tenant)) and can_contribute from tenant and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
```

How to read it:

- **Roles.** `internal_admin` implies `internal_user`; `external_admin` implies `external_user`. An admin satisfies the lower relation without a second tuple.
- **Internal members see and edit everything.** Every content `can_view` and `can_edit` includes `internal_user from tenant`, so internal users and admins reach every folder, album, document, and photo, private or public, without any share.
- **External members see public content.** A container is externally visible (`external_visible`) when it is public **and** every ancestor is public: `external_user from public and (external_user from root or external_visible from parent)`. Top-level containers carry a `root` tuple that anchors the chain; children inherit through `parent`. One private ancestor hides the whole subtree, even subfolders flagged public. Documents and photos use their container's `external_visible`.
- **External admins contribute in public content.** `can_edit` includes `external_visible and external_admin from tenant`: they create subfolders, upload, and edit any item in public content. `can_manage` (delete, share) stays with the owner or the internal admin, so an external admin can delete only what they own.
- **External users are read-only.** `can_contribute` excludes them, so every `can_edit`, `can_manage`, and create relation is false for them even if a tuple names them as editor.
- **Shares reach into private content.** `viewer`/`editor` accept a specific `user` and the groups `tenant#external_user` (viewer) / `tenant#external_admin` (viewer or editor), and inherit down `parent` and to items. This is how an internal admin opens a private folder to an external person or group.
- **Creating containers.** `can_create_folder`/`can_create_album` is any contributor with the product on (internal members and external admins). `can_create_private_folder`/`can_create_private_album` is internal members only, so external admins can only create public containers. A subfolder also needs `can_edit` on its parent (checked in the service).
- **Changing visibility** (`can_change_visibility`) is `can_manage` intersected with `internal_user from tenant`: the internal admin, or an internal owner.
- **Owners** keep `can_view`/`can_manage` on what they own (owner implies editor implies viewer), even when the surrounding folder is private to them.
- **Membership is a prerequisite.** `can_view` intersects `can_read_tenant from tenant`, so a stale share on a removed member grants nothing.
- **Products.** `documents_enabled`/`photos_enabled` accept only `user:*`; when absent every content check and create relation is false. Only the platform toggles them (the service checks `can_manage_platform`); `can_view_products` lets the internal admin read their status. Member administration and product viewing are not intersected with a product.
- **No `but not`.** "Private wins" is expressed with intersection over the ancestor chain, so the model stays allow-only.
- A system admin is not an `internal_admin` of each tenant. Nothing pulls `platform#system_admin` into a tenant or resource relation.

## Model bootstrap

`api/internal/infrastructure/fga/bootstrap.go` has two entry points.

`Reset` (used by `cmd/seed`):

1. List stores. Delete every store named `OPENFGA_STORE_NAME` (`fga-demo`).
2. Create a fresh store with that name.
3. Read `OPENFGA_MODEL_PATH`, transform the DSL, and call WriteAuthorizationModel.
4. Print `store_id` and `model_id` to stdout.

`Open` (used by `cmd/api`):

1. List stores and find the one named `OPENFGA_STORE_NAME`. A missing store is a startup error that tells the operator to run `cmd/seed`.
2. Call ReadAuthorizationModels with page size 1. The first result is the newest model. Use its id.

Every Check, BatchCheck, ListObjects, Read, and Write sends that `authorization_model_id`. Check, BatchCheck, and ListObjects use consistency `HIGHER_CONSISTENCY`.

Resetting the store is what makes the seed repeatable: the demo always starts from the README's step 1. The store id changes after each seed run, so restart `cmd/api` after seeding and reselect the store in the Playground.

## Authz gateway

`entity.Tuple` and `entity.CheckRequest` live in `api/internal/domain/entity/authz.go`; the gateway is defined in `api/internal/domain/gateway/authz_gateway.go` and implemented by `infrastructure/fga.Client`. Services do not call it for decisions directly: `service.authorizer` wraps it as `usecase.Authorizer` (`Require` = 403, `RequireVisible` = 404, `PlatformOr`, `Permissions`), and `service.writeGuard` owns every `Write`.

```go
type Tuple struct {
    User     string // "user:<uuid>", "user:*", or "tenant:<uuid>#<relation>"
    Relation string
    Object   string // "<type>:<id>"
}

type CheckRequest struct {
    UserID     string
    Relation   string
    ObjectType string
    ObjectID   string
}

type AuthzGateway interface {
    Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error)
    BatchCheck(ctx context.Context, checks []CheckRequest) ([]bool, error)
    ListObjectIDs(ctx context.Context, userID, relation, objectType string) ([]string, error)
    ReadTuples(ctx context.Context, objectType, objectID string) ([]Tuple, error)
    Write(ctx context.Context, writes, deletes []Tuple) error
}
```

`ListObjectIDs` strips the type prefix and returns bare ids. An empty OpenFGA result is an empty slice, not an error.

`Write` sends writes and deletes in one OpenFGA Write API call so the tuple change is atomic. It sets `on_duplicate: ignore` for writes and `on_missing: ignore` for deletes, so writing a tuple that exists or deleting one that does not is success. That makes every tuple command idempotent and safe to retry. A command with more than 100 tuple keys returns an error instead of splitting, because splitting would break atomicity. The demo never reaches that size.

`BatchCheck` uses the server BatchCheck API, sends at most 50 checks per call, and returns results in request order.

## Tuple catalog

Pure relation builders in `api/internal/infrastructure/fga` build these slices, one per domain (`MembershipRelations`, `ProductRelations`, `FolderRelations`, `DocumentRelations`, `AlbumRelations`, `PhotoRelations`), reached through the `*RelationGateway` interfaces. Services persist Postgres, then hand the tuples to `helper.WriteGuard`. Unit tests cover the builders without a server.

Notation: `user:U`, `tenant:T`, `folder:F`, `document:D`, `album:A`, `photo:P`.

### Platform admin seed

Write `platform:main#system_admin@user:U`. The demo has no HTTP command for this. Only the seed writes it, and it also inserts `platform_admins`.

### Create user

No tuple. A user is an identity until some command relates them to an object.

### Create tenant

Insert `tenants`. Insert `tenant_products` rows for `documents` and `photos`, both `enabled = false`. No tuples. Products start disabled, which means the wildcard tuples are absent.

### Set product

Check is in the decision catalog. Then:

- Enabled documents: write `tenant:T#documents_enabled@user:*` and set `tenant_products.enabled = true`.
- Disabled documents: delete `tenant:T#documents_enabled@user:*` and set `enabled = false`.
- Photos uses relation `photos_enabled`.

Write the wildcard only. Do not write `documents_enabled` for a concrete user. Do not delete content tuples.

### Assign role

The body carries `affiliation` and `role` on create. A role change carries `role` only. Legal pairs:

| Affiliation | Roles |
| --- | --- |
| internal | `internal_admin`, `internal_user` |
| external | `external_admin`, `external_user` |

Any other pair is 400 `affiliation_role_mismatch`. On update, the new role must belong to the membership's existing affiliation. Affiliation is immutable. Changing it means delete the membership and create it again.

One OpenFGA Write. Write the new role tuple and delete the other three role tuples so a person has one direct role relation. The authz gateway ignores missing deletes, so the command does not need to know which tuple existed before:

| New role | Write | Delete |
| --- | --- | --- |
| `internal_admin` | `tenant:T#internal_admin@user:U` | `internal_user`, `external_admin`, `external_user` on `tenant:T` for `user:U` |
| `internal_user` | `tenant:T#internal_user@user:U` | `internal_admin`, `external_admin`, `external_user` |
| `external_admin` | `tenant:T#external_admin@user:U` | `external_user`, `internal_admin`, `internal_user` |
| `external_user` | `tenant:T#external_user@user:U` | `external_admin`, `internal_admin`, `internal_user` |

Do not also write the implied relation. `internal_admin` already satisfies `internal_user`, and `external_admin` already satisfies `external_user`.

Upsert `memberships.affiliation` and `memberships.role` on create. Update `role` only on change.

Reject the change with 409 when it would leave the tenant with zero rows where `role = internal_admin`. External admins do not count. That count is a stored invariant, not an authorization check.

### Remove member

Load the membership and the user's `resource_grants` rows for that tenant. One OpenFGA Write deletes the tuples below. Then one Postgres transaction deletes those grant rows and the `memberships` row. Revocations go to OpenFGA first, so a failure between the two steps leaves less access, not more. The tuples are:

- `tenant:T#internal_admin@user:U`, `tenant:T#internal_user@user:U`, `tenant:T#external_admin@user:U`, and `tenant:T#external_user@user:U` (the authz gateway ignores the three that do not exist)
- every direct grant: `folder|document|album|photo:<id>#<relation>@user:U`

Do not delete tuples whose user is `tenant:T#external_user` or `tenant:T#external_admin`, nor `root`/`public` markers.

The same last-internal-admin 409 applies.

### Create folder

Body carries `is_private` (required). Insert `folders` (with `is_private`) and an owner `resource_grants` row (`subject_type=user`, `relation=owner`). Tuples:

- `folder:F#tenant@tenant:T`
- `folder:F#owner@user:actor`
- `folder:F#parent@folder:parent` when `parent_id` is set, otherwise `folder:F#root@tenant:T`
- `folder:F#public@tenant:T` when `is_private` is false

Creating an album is the same with type `album`.

### Set visibility

`PUT …/visibility {is_private}`. Check `can_change_visibility`. Making a container public writes `folder:F#public@tenant:T` and sets `is_private = false` (a grant: Postgres first). Making it private deletes that tuple (a revoke: OpenFGA first). No other tuple changes; descendants follow automatically through the ancestor chain.

### Create document

Insert `documents` and an owner grant. Tuples:

- `document:D#tenant@tenant:T`
- `document:D#folder@folder:F`
- `document:D#owner@user:actor`

Creating a photo is the same with `photo:P#album@album:A`.

The creator becomes the owner. Do not also write `editor` or `viewer` for the creator. Those are implied.

### Share

Body `{relation, subject, user_id?}`. `relation` is `viewer` or `editor`. `subject` is `user`, `external_users`, or `external_admins`. `user_id` is required when `subject` is `user` and must be absent otherwise.

Specific user:

- Insert or replace the user grant. The unique index allows one relation per user per object. When the user already has the other relation, delete the old tuple and write the new one in the same Write, and update the row.
- User tuple: `<objectType>:<id>#viewer@user:U` or `#editor@user:U`.

Affiliation userset:

| Subject | Relation | Tuple user |
| --- | --- | --- |
| `external_users` | `viewer` only | `tenant:T#external_user` |
| `external_admins` | `viewer` or `editor` | `tenant:T#external_admin` |

When the subject already holds the other relation on that object, the same Write deletes the old tuple and writes the new one, and the row is updated.

Insert or replace the userset grant for that subject type. Reject `editor` with subject `external_users` with 400 `cannot_upload`.

Reject `owner` with 400. Reject a grant that would change or remove the owner row with 400.

Before a user grant, Check `can_read_tenant` for the target user on `T`. A false result is 400 `not_a_member`. An editor grant also Checks `can_contribute` for the target. A false result is 400 `cannot_upload`. An external user fails `can_contribute`, so they cannot be granted editor. Userset grants have no target user.

### Revoke grant

Body is the share shape. Delete the matching tuple, then the matching `resource_grants` row. Revoke of an owner row is 400. A missing grant is 404.

### Delete document or photo

Check `can_delete`. Read every tuple for that object, delete them in one Write, then delete the grants and the row in one Postgres transaction. If Postgres fails, write the tuples back.

### Delete folder or album

Return 409 when any child folder, document, child album, or photo still references it. Otherwise delete the object's tuples, then its grants and row, as for a document.

### Rename

Update `name` in Postgres. No tuple change. Folders and albums require `can_manage`. Documents and photos require `can_edit`.

## Write path and compensation

The order depends on whether the command adds or removes access. Either way, a crash between the two stores leaves less access than intended, never more.

```mermaid
sequenceDiagram
    participant API
    participant PG as Postgres
    participant FGA as OpenFGA
    Note over API,FGA: Grant (create, share, add member, enable product, role change)
    API->>PG: Begin, write rows, commit
    API->>FGA: Write tuples and deletes in one call
    alt tuple write fails
        API->>PG: Inverse the committed rows
        API-->>API: Return 500
    end
    Note over API,FGA: Revoke (remove member, revoke grant, delete object, disable product)
    API->>FGA: Delete tuples in one call
    API->>PG: Begin, delete rows, commit
    alt Postgres fails
        API->>FGA: Write the deleted tuples back
        API-->>API: Return 500
    end
```

There is no outbox. A crash between the two steps can leave a display row with no matching tuple. That is fail-closed. Say so in the 500 log. The operator reruns the action (tuple writes are idempotent) or reruns `cmd/seed`. Do not build a repair daemon.

A role change is treated as a grant. It writes the new role tuple and deletes the others in one Write, so there is no moment with zero or two roles.

Compensation:

- Insert: delete the rows this command inserted.
- Role change: restore the previous `memberships` row.
- Enable product: restore `enabled = false`.
- Revoke commands: write back the tuples that were deleted.

If compensation itself fails, log both errors and still return 500.

## Decision catalog

Object ids below are bare. The fga infrastructure formats them.

| Action | Check user | Relation | Object |
| --- | --- | --- | --- |
| List all tenants, create tenant, create user, patch user | actor | `can_manage_platform` | `platform:main` |
| Set product | actor | `can_manage_platform` | `platform:main` (system admin only; tenant admins cannot toggle) |
| List my tenants | actor | ListObjects `can_read_tenant` | type `tenant` |
| Get one tenant | actor | `can_manage_platform` on `platform:main` OR `can_read_tenant` on that tenant | either check true |
| List products | actor | `can_manage_platform` on `platform:main` OR `can_view_products` on that tenant | either check true; 403 otherwise |
| List members | actor | `can_manage_platform` on `platform:main` OR `can_manage_members` on that tenant | either check true; 403 otherwise |
| List share candidates for an object | actor | `can_share` | that object |
| Assign, change, or remove a member | actor | `can_manage_platform` on `platform:main` OR `can_manage_members` on that tenant | either check true |
| Create folder | actor | `can_create_folder`; plus `can_create_private_folder` when `is_private`; plus `can_edit` on the parent when `parent_id` is set | that tenant / parent folder |
| Create album | actor | `can_create_album`; plus `can_create_private_album` when `is_private`; plus `can_edit` on the parent | that tenant / parent album |
| Make a folder or album public or private | actor | `can_change_visibility` | that folder or album |
| List folders | actor | ListObjects `can_view` | type `folder` |
| List documents | actor | ListObjects `can_view` | type `document` |
| List albums | actor | ListObjects `can_view` | type `album` |
| List photos | actor | ListObjects `can_view` | type `photo` |
| Get or rename folder, create document in it | actor | `can_view` for get, `can_manage` for rename, `can_create_document` for create | that folder |
| Get document | actor | `can_view` | that document |
| Rename document | actor | `can_edit` | that document |
| Delete document | actor | `can_delete` | that document |
| Share or revoke on a folder or document | actor | `can_share` | that object |
| Album and photo verbs | actor | the album or photo relation with the same name as the folder or document verb | that album or photo |
| Delete folder or album | actor | `can_delete` | that folder or album |

OR in the table means the service calls Check twice (`Authorizer.PlatformOr`) and allows the action when either call is true. That composition stays in the service layer. It is not a role test, and it is not a new OpenFGA relation that would make a system admin a tenant member.

List queries:

```sql
SELECT ... FROM documents
WHERE tenant_id = $tenant AND id = ANY($ids)
```

`$ids` is the ListObjects result. When `$ids` is empty, return an empty JSON array and do not query the table. After the query, drop nothing further except ids ListObjects returned that are not in this tenant. Never `SELECT` every row in the tenant and filter in Go.

The same shape applies to folders, albums, and photos.

Parent folder for `POST /documents`: the folder row's `tenant_id` must equal `X-Tenant-ID`. Otherwise 404.

Depth: reject folder and album creation at 409 when the parent chain is longer than 10. Walk `parent_id` in Postgres and reject a cycle with 409.

## HTTP API

Base path `/api/v1`. JSON uses snake_case. Timestamps are RFC3339. Authenticated routes require `Authorization: Bearer <jwt>`. Creates return 201 with the created object. Deletes and revokes return 204 with no body.

### Error catalog

| Status | Code | When |
| --- | --- | --- |
| 400 | `validation` | Malformed JSON, bad UUID, empty name, bad slug, bad role, bad product, owner grant, unknown affiliation |
| 400 | `affiliation_role_mismatch` | Internal role on an external member, external role on an internal member, or a role change that would switch affiliation |
| 400 | `not_a_member` | Share target fails `can_read_tenant` |
| 400 | `cannot_upload` | Editor grant targets an external user, or subject `external_users` is combined with relation `editor` |
| 401 | `unauthenticated` | Missing or invalid JWT |
| 403 | `forbidden` | A required Check returned false on a write |
| 404 | `not_found` | Unknown id, or a GET-by-id whose Check returned false, or `X-Tenant-ID` does not own the object |
| 409 | `conflict` | Duplicate email or slug, duplicate membership, last internal admin, cycle, depth, folder or album still has children |
| 500 | `tuple_write_failed` | OpenFGA Write failed and compensation ran |
| 500 | `internal` | Any other unexpected error |

Error body:

```json
{"error": "forbidden", "check": {"relation": "can_view", "object": "document:<uuid>"}}
```

Omit `check` when the failure is not a Check. GET-by-id uses 404 for a failed Check so existence in another tenant stays hidden. Collection routes return an empty list, not 403, when ListObjects is empty. Writes use 403 when Check fails.

### Auth and profile

`POST /api/v1/auth/login`

```json
{"email": "ada@example.com", "password": "password"}
```

Response 200:

```json
{
  "access_token": "<jwt>",
  "expires_at": "2026-10-18T12:00:00Z",
  "user": {"id": "<uuid>", "email": "ada@example.com", "name": "Ada", "is_system_admin": true}
}
```

`is_system_admin` on login and on `GET /api/v1/me` is the Check `can_manage_platform` on `platform:main`.

`GET /api/v1/me` returns the same user object plus:

```json
{
  "user": {},
  "tenants": [
    {"id": "<uuid>", "slug": "acme", "name": "Acme", "affiliation": "internal", "role": "internal_admin"}
  ]
}
```

`tenants` comes from ListObjects `can_read_tenant`, then a Postgres load of those ids. `affiliation` and `role` are the display labels from `memberships`.

`GET /api/v1/me/permissions?tenant_id=<optional uuid>` returns the actor's own permissions, evaluated in one BatchCheck:

```json
{
  "platform": {"can_manage_platform": false},
  "tenant_id": "<uuid>",
  "tenant": {
    "can_read_tenant": true, "can_contribute": true, "can_manage_members": true, "can_view_products": true,
    "can_create_folder": true, "can_create_album": true, "documents_enabled": true, "photos_enabled": true
  }
}
```

`tenant_id` and `tenant` are null when the query parameter is omitted. A malformed `tenant_id` is 400. The endpoint only evaluates the actor, so it needs no Check of its own and reveals nothing beyond the actor's own access. The relation lists are the inspector whitelist for `platform` and `tenant`.

`GET /healthz` returns 200 `{"status":"ok"}` with no auth. It does not touch OpenFGA.

### Users

System admin (`can_manage_platform`).

- `GET /api/v1/users` returns every user. `is_system_admin` is true when a `platform_admins` row exists.
- `POST /api/v1/users` body `{email, name, password}` creates an identity. 201 and the user object. No tuple.
- `PATCH /api/v1/users/:id` body `{name}` updates the display name.

Validation: email contains `@`, name length 1 to 200 after trim, password length 8 to 72.

### Tenants

- `GET /api/v1/tenants` lists every tenant. Check `can_manage_platform`. Include each tenant's products from `tenant_products`.
- `POST /api/v1/tenants` body `{name, slug}`. Slug matches `^[a-z0-9-]{2,40}$`. Check `can_manage_platform`. Insert products disabled. 201.
- `GET /api/v1/tenants/mine` is ListObjects `can_read_tenant` for the actor, plus Postgres rows and the actor's `affiliation` and `role` labels. Any authenticated user.
- `GET /api/v1/tenants/:id` is allowed by the "get one tenant" OR check.
- `PATCH /api/v1/tenants/:id` body `{name}`. Check `can_manage_platform`.

There is no delete tenant route.

### Products

- `GET /api/v1/tenants/:id/products` returns `{product, enabled, updated_at}` for both products. OR check of `can_manage_platform` or `can_view_products` (internal admin); 403 otherwise. Tenant admins can see product state but not change it. `enabled` is the Postgres display flag. (Members learn whether a product is on from `GET /me/permissions`.)
- `PUT /api/v1/tenants/:id/products/:product` where `:product` is `documents` or `photos`. Body `{"enabled": true}`. Allow only when `can_manage_platform` is true: products are a platform entitlement. Then the set-product tuple command.

An internal tenant admin and a system admin can both call this PUT. An internal user, an external admin, and an external user receive 403. Disabling the product still deletes only the wildcard tuple.

### Members

Routes do not require `X-Tenant-ID`. The tenant is the path id.

- `GET /api/v1/tenants/:id/members?affiliation=internal|external` returns `{user_id, email, name, affiliation, role}`. The query parameter is optional. `internal` and `external` add `AND affiliation = $aff` after the authz check. Any other value is 400. OR check of `can_manage_platform` or `can_manage_members`; 403 otherwise. The roster is tenant administration: internal admins and the platform only. The share dialog does not use it; it uses the object's share candidates. The filter is display. It is not an authorization decision.
- `POST /api/v1/tenants/:id/members` body `{user_id, affiliation, role}`. `affiliation` is `internal` or `external`. `role` must be one of the two roles for that affiliation. OR check of `can_manage_platform` or `can_manage_members`. 409 when the membership exists. 404 when the user id does not exist. 400 `affiliation_role_mismatch` for a crossed pair.
- `PATCH /api/v1/tenants/:id/members/:userId` body `{role}`. Same OR check. The role must belong to the current affiliation. Role-change tuples. There is no field for affiliation.
- `DELETE /api/v1/tenants/:id/members/:userId`. Same OR check. Remove-member tuples.

### Documents and folders

Require `X-Tenant-ID`.

- `GET /api/v1/folders`
- `POST /api/v1/folders` body `{name, parent_id?, is_private}`. `is_private` is required (400 when missing). `parent_id` null creates a top-level folder. Checks per the decision catalog. Parent must be in the header tenant.
- `PUT /api/v1/folders/:id/visibility` body `{is_private}`. Check `can_change_visibility`.
- `GET /api/v1/folders/:id`
- `PATCH /api/v1/folders/:id` body `{name}`. Check `can_manage` on the folder.
- `DELETE /api/v1/folders/:id`. Check `can_delete`.
- `GET /api/v1/documents?folder_id=<optional>`
- `POST /api/v1/documents` body `{folder_id, name, mime_type, byte_size}`. Check `can_create_document` on the folder. This is the upload command. It stores metadata and does not accept a multipart body.
- `GET /api/v1/documents/:id`
- `PATCH /api/v1/documents/:id` body `{name}`
- `DELETE /api/v1/documents/:id`
- `POST /api/v1/folders/:id/grants` and `POST /api/v1/documents/:id/grants` body `{relation, subject, user_id?}`
  - Specific person: `{relation: "viewer"|"editor", subject: "user", user_id}`. Editor requires the target's `can_contribute`.
  - All external users: `{relation: "viewer", subject: "external_users"}`
  - All external admins: `{relation: "viewer"|"editor", subject: "external_admins"}`
  - Shares exist for external members; internal members already see everything.
- `DELETE /api/v1/folders/:id/grants` and `DELETE /api/v1/documents/:id/grants` with the same body shape, identifying the grant to remove.
- `GET /api/v1/folders/:id/share-candidates` and `GET /api/v1/documents/:id/share-candidates` return the external members of the object's tenant as `{user_id, email, name, affiliation, role}`. Check `can_share` on the object. This lets an owner share without reading the full roster.
- `GET /api/v1/folders/:id/grants` and `GET /api/v1/documents/:id/grants` list the `resource_grants` rows as `{relation, subject, user_id, name}`. Check `can_share`. The share dialog uses this to show and revoke existing grants.

`GET /documents` returns every visible document in the tenant when `folder_id` is omitted, including documents whose parent folder the caller cannot view. The UI uses that response for a "Shared with me" group.

Name length is 1 to 200. `mime_type` length is 1 to 200. `byte_size` is an integer greater than or equal to 0.

Folder and document JSON includes `id`, `tenant_id`, `name`, `created_by`, `created_at`, plus `parent_id` and `is_private` for folders or `folder_id` for documents, and for documents `mime_type` and `byte_size`.

List and get-by-id responses also include `permissions`: the actor's result for every content relation in the inspector whitelist for that type (for a folder: `can_view`, `can_edit`, `can_manage`, `can_delete`, `can_share`, `can_change_visibility`, `can_create_document`). The service computes them with one BatchCheck per page (`Authorizer.Permissions`). Create and rename responses omit it.

### Photos and albums

Same routes and checks, with these replacements:

| Documents | Photos |
| --- | --- |
| `/folders` | `/albums` |
| `/documents` | `/photos` |
| `parent_id` on folder | `parent_id` on album |
| `folder_id` | `album_id` |
| `can_create_folder` | `can_create_album` |
| `can_create_document` | `can_create_photo` |
| `documents_enabled` | `photos_enabled` |

Grant routes: `POST` and `DELETE /api/v1/albums/:id/grants` and `/api/v1/photos/:id/grants`.

### Access inspector

`POST /api/v1/authz/check`

```json
{
  "user_id": "<uuid>",
  "relation": "can_view",
  "object_type": "folder",
  "object_id": "<uuid>"
}
```

Response 200: `{"allowed": false}`.

The actor may check their own `user_id` always. The actor may check another user when `can_manage_platform` is true, or when the actor has `can_manage_members` on the object's tenant. The API derives that tenant from the object, never from the request: a `tenant` object is its own tenant, a content object's tenant is loaded from Postgres, and `platform:main` has no tenant. A missing object is 404. Otherwise 403. This keeps an admin of one tenant from probing access to another tenant's objects.

Whitelisted relations: `can_view`, `can_edit`, `can_delete`, `can_share`, `can_create_document`, `can_create_photo`, `can_create_folder`, `can_create_album`, `can_manage`, `can_manage_members`, `can_view_products`, `can_contribute`, `can_read_tenant`, `can_manage_platform`, `documents_enabled`, `photos_enabled`.

`can_manage_platform` requires `object_type=platform` and `object_id=main`. Tenant relations require `object_type=tenant`. Content relations require the matching object type. A mismatch is 400.

`POST /api/v1/authz/batch-check` body `{"checks":[...]}` with the same item shape, maximum 50. Same per-item authorization rule. Response `{"results":[{"allowed":true}]}` in request order. The UI uses this to hide buttons.

`GET /api/v1/access?object_type=document&object_id=<uuid>`

Caller must pass at least one of: `can_manage_platform`, `can_share` on the object, or `can_manage_members` on the object's tenant. Load the object's tenant from Postgres. A missing object is 404.

Response:

```json
{
  "object": "document:<uuid>",
  "direct_tuples": [
    {"user": "user:<uuid>", "relation": "owner", "object": "document:<uuid>"}
  ],
  "members": [
    {"user_id": "<uuid>", "name": "Bob", "can_view": true, "can_edit": true, "can_delete": true}
  ]
}
```

`direct_tuples` is OpenFGA Read for that object. `members` is the Postgres member list of the object's tenant, plus every platform admin, each evaluated with BatchCheck for `can_view`, `can_edit`, and `can_delete`. This stays accurate when the product wildcard removes access and the owner tuple remains.

Do not add a ListUsers requirement. Read plus BatchCheck is the inspector.

## Seed

`cmd/seed` resets the demo. It runs migrations, truncates every app table, recreates the OpenFGA store (see Model bootstrap), inserts the rows in this section, and writes the tuples in this section. Running it again always returns the world to this state and discards anything created in the UI. That is the intended behavior for a click-through demo.

Password for every user is `password`, hashed at seed time.

### People

| Name | Email | Id | Platform |
| --- | --- | --- | --- |
| Ada | ada@example.com | `11111111-1111-1111-1111-111111111111` | system admin |
| Bob | bob@example.com | `22222222-2222-2222-2222-222222222222` | |
| Cara | cara@example.com | `33333333-3333-3333-3333-333333333333` | |
| Dan | dan@example.com | `44444444-4444-4444-4444-444444444444` | |
| Eve | eve@example.com | `55555555-5555-5555-5555-555555555555` | |
| Fay | fay@example.com | `66666666-6666-6666-6666-666666666666` | |

### Tenants and memberships

| Tenant | Slug | Id | Members |
| --- | --- | --- | --- |
| Acme | acme | `aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | Bob internal_admin, Cara internal_user, Dan external_user, Fay external_admin |
| Globex | globex | `bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | Eve internal_admin, Bob external_user |

Role tuples, and no implied duplicate:

- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#internal_admin@user:22222222-2222-2222-2222-222222222222`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#internal_user@user:33333333-3333-3333-3333-333333333333`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#external_user@user:44444444-4444-4444-4444-444444444444`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#external_admin@user:66666666-6666-6666-6666-666666666666`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#internal_admin@user:55555555-5555-5555-5555-555555555555`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#external_user@user:22222222-2222-2222-2222-222222222222`

Both products enabled on both tenants, which means all four wildcard tuples exist.

### Acme content

| Kind | Name | Id | Parent or container | Visibility | Owner | Shares |
| --- | --- | --- | --- | --- | --- | --- |
| folder | Company | `f1111111-1111-1111-1111-111111111111` | top level | private | Bob | |
| folder | Board | `f2222222-2222-2222-2222-222222222222` | top level | private | Bob | |
| folder | Legal | `f3333333-3333-3333-3333-333333333333` | Board | public flag, hidden by Board | Bob | |
| folder | Client | `f7777777-7777-7777-7777-777777777777` | top level | public | Bob | |
| folder | Specs | `f9999999-9999-9999-9999-999999999999` | Client | private | Bob | |
| folder | Partners | `faaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | top level | private | Bob | viewer Dan |
| document | Handbook | `d1111111-1111-1111-1111-111111111111` | Company | | Bob | |
| document | Secret | `d2222222-2222-2222-2222-222222222222` | Board | | Bob | |
| document | Q3 Plan | `d3333333-3333-3333-3333-333333333333` | Board | | Bob | |
| document | Legal Hold | `d4444444-4444-4444-4444-444444444444` | Legal | | Bob | |
| document | Client Brief | `d7777777-7777-7777-7777-777777777777` | Client | | Bob | |
| document | Visit Agenda | `d9999999-9999-9999-9999-999999999999` | Client | | Fay | |
| document | Pricing | `daaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | Specs | | Bob | |
| document | Partner Deck | `dbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | Partners | | Bob | |
| album | All-hands | `c1111111-1111-1111-1111-111111111111` | top level | private | Bob | |
| album | Offsite | `c2222222-2222-2222-2222-222222222222` | top level | private | Bob | |
| album | Client visit | `c7777777-7777-7777-7777-777777777777` | top level | public | Bob | |
| photo | Kickoff | `e1111111-1111-1111-1111-111111111111` | All-hands | | Bob | |
| photo | Offsite Shot | `e2222222-2222-2222-2222-222222222222` | Offsite | | Bob | |
| photo | Lobby | `e7777777-7777-7777-7777-777777777777` | Client visit | | Bob | |

Mime type `text/plain` and byte size `128` on every seeded document. Mime type `image/jpeg` and byte size `2048` on every seeded photo. `created_by` is the owner.

### Globex content

| Kind | Name | Id | Parent or container | Visibility | Owner |
| --- | --- | --- | --- | --- | --- |
| folder | Company | `f4444444-4444-4444-4444-444444444444` | top level | private | Eve |
| folder | Board | `f5555555-5555-5555-5555-555555555555` | top level | private | Eve |
| folder | Client | `f8888888-8888-8888-8888-888888888888` | top level | public | Eve |
| document | Handbook | `d5555555-5555-5555-5555-555555555555` | Company | | Eve |
| document | Secret | `d6666666-6666-6666-6666-666666666666` | Board | | Eve |
| document | Client Brief | `d8888888-8888-8888-8888-888888888888` | Client | | Eve |
| album | All-hands | `c3333333-3333-3333-3333-333333333333` | top level | private | Eve |
| album | Offsite | `c4444444-4444-4444-4444-444444444444` | top level | private | Eve |
| album | Client visit | `c8888888-8888-8888-8888-888888888888` | top level | public | Eve |
| photo | Kickoff | `e3333333-3333-3333-3333-333333333333` | All-hands | | Eve |
| photo | Offsite Shot | `e4444444-4444-4444-4444-444444444444` | Offsite | | Eve |
| photo | Lobby | `e8888888-8888-8888-8888-888888888888` | Client visit | | Eve |

Every container gets the tuples from "Create folder": `tenant`, `owner`, `parent` or `root`, and `public` when public. Every item gets `tenant`, its container, and `owner`. The one share is `folder:faaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#viewer@user:44444444-4444-4444-4444-444444444444` (Partners to Dan). All four product wildcards exist. The ids, rows, and tuples live in `api/internal/seed/world.go` (one slice per domain: `Folders`, `Albums`, `Documents`, `Photos`); this table describes them.

## React admin panel

Vite dev server proxies `/api` to `http://localhost:8088`. The client sends the JWT from `sessionStorage` and the selected tenant id from `sessionStorage` key `tenant_id` as `X-Tenant-ID`.

Routes:

| Path | Page | Behavior |
| --- | --- | --- |
| `/login` | Login | `POST /auth/login`. Store the token. |
| `/` | Home | `GET /me`. Show the person, whether they are a system admin, and each membership's affiliation and role. Explain internal versus external in a short static paragraph. |
| `/users` | Users | Feature `nav.users` (`can_manage_platform`) for the menu entry and the route guard. List and create users. |
| `/tenants` | Tenants | Feature `nav.tenants`, same rule. List tenants, create a tenant, link to detail. |
| `/tenants/:id` | Tenant detail | Tenant administration: products and members. Guarded by feature `tenant.admin` (`can_manage_members` or `can_manage_platform`) for the tenant in the URL; other members get "Not available". The sidebar entry "Members" uses `nav.members` (`tenant:can_manage_members` on the active tenant), so only internal admins see it; a system admin opens the page from the Tenants directory. Product switches render only for `tenant.products.manage` (`can_manage_platform`); internal admins see status badges and the note "Products are enabled or disabled by the platform admin." Member controls render for `tenant.members.manage` (`can_manage_platform` or `can_manage_members`). A filter control calls `GET /members?affiliation=internal` or `external`. The invite form picks affiliation first and then offers only `internal_admin` and `internal_user`, or only `external_admin` and `external_user`. The role editor for an existing member offers only the two roles of that member's affiliation. |
| `/documents` | Documents | Require a selected tenant. Tree of folders the user can view, each with a Private, Public, or "Public · hidden by parent" badge (computed from loaded ancestors). The row menu offers Make public/private when `can_change_visibility` is true; making something public asks for confirmation. The New folder dialog shows a Private switch (default on) only with feature `folder.createPrivate`; otherwise it states the folder will be public. The parent list contains only folders where the actor has `can_edit`. Documents whose folder is hidden and that ListObjects returned go under "Shared with me". Create folder, add document metadata, rename, delete, and share call the matching routes. Hide each button unless the row's `permissions` allow the action (see `ACTIONS`). Route guard `nav.documents`; when the actor is a member but the product is off, the guard explains that the product is turned off. |
| `/photos` | Photos | Same interaction as documents, for albums and photos. |
| `/inspector` | Inspector | Form for `POST /authz/check`. Object lookup via `GET /access`, showing direct tuples and the member Check grid. |

`TenantSwitcher` calls `GET /tenants/mine` (ListObjects `can_read_tenant`) and shows each tenant with the actor's affiliation and role labels. Changing tenant writes `tenant_id`, which reloads page data and the active-tenant permissions. On `/tenants/:id` it also navigates to the new tenant's page. A stored tenant the user can no longer read is replaced after `/me` refreshes. Ada's switcher is empty. She still opens `/tenants` from the nav.

`ShareDialog` loads the object's `GET …/share-candidates` and `GET …/grants`, lists existing grants with a revoke button for each non-owner grant, and offers viewer or editor for an external person, viewer for all external users, and viewer or editor for all external admins. Only external members are listed: internal members already see everything. Editor is disabled in the dialog when the selected person is an external user. The API still returns 400 `cannot_upload` if that grant is posted. It posts the grant route.

Permissions in the UI live in `web/src/permissions.tsx`, the only file that maps permissions to UI:

- `FEATURES` maps a feature name (`nav.users`, `tenant.members.manage`, `folder.create`, …) to a rule over permissions written `platform:<relation>` or `tenant:<relation>`. A rule has `all` (every permission must be true) and/or `any` (one is enough, mirroring the OR rows of the decision catalog); when both are present both must pass, and an empty rule is denied.
- Product menus and pages need membership **and** the product: `nav.documents` is `all: [tenant:can_read_tenant, tenant:documents_enabled]`, `nav.photos` is `all: [tenant:can_read_tenant, tenant:photos_enabled]`. Disabling a product removes its menu entry for every member, and a typed URL shows "<Product> is turned off for this tenant".
- `ACTIONS` maps an object action (`rename`, `delete`, `share`, `addItem`) and object type to the relation in that row's `permissions` object. Rename is `can_manage` on folders and albums and `can_edit` on documents and photos.
- `PermissionsProvider` loads `GET /me/permissions` for the active tenant and reloads when the tenant changes or `/me` is refreshed after a mutation. `useCan(feature)`, `useAllows()`, and `<Can feature>` read it. Everything is denied while loading.
- `RequireFeature` guards routes, so a typed URL shows "Not available" instead of a failed request. `/tenants/:id` is guarded against the tenant in the URL through `usePermissionSet(id)`, because a system admin opens tenants that are not active.
- The top bar menu is a config array of `{label, to, feature}` filtered with `useAllows()`. Home and Inspector have no feature: everyone may inspect their own access. The Members entry uses `tenant:can_manage_members` only: internal admins of the active tenant. The Inspector loads the roster for its user picker only with `tenant.members.manage`; everyone else checks themselves.
- Permissions are revalidated in the background when the tab regains focus, so a product another admin switched off disappears without a reload. A revalidation keeps the current permissions on screen; only a tenant switch denies everything until the new answer arrives.
- Content pages render buttons from each row's `permissions` through `canAct`. They do not call batch Check. `POST /authz/batch-check` stays available as an API; the inspector page uses `POST /authz/check`.
- Components never name an OpenFGA relation and never read `role` or `affiliation` to show a control. The one display-only exception is the share dialog greying out editor for a person labelled `external_user`, which the API enforces with `cannot_upload` anyway.

Empty lists stay empty, with one line of copy: "Nothing here is visible with the current relationships." Do not fall back to a role name to show rows the API omitted.

Visual design uses Mantine and looks like a real admin app:

- `AppShell` with a header (logo, tenant switcher `Select`, user menu with a dark-mode toggle and sign out) and a sidebar split into Workspace and Platform sections. The sidebar collapses behind a burger below the `sm` breakpoint.
- Pages start with `PageHeader` (title, description, primary action). Lists are `Table`s inside `Card`s; folders and albums are a collapsible tree with a `Menu` of row actions built from `ACTIONS`.
- Forms open in `Modal`s. Destructive actions confirm through `@mantine/modals`. Results and API errors appear as `@mantine/notifications` toasts via `run`/`notifyError` in `web/src/ui.tsx`, which also maps error codes to readable titles.
- Role and affiliation labels show as badges for display only.
- Theme: indigo primary, `defaultColorScheme="auto"`. `web/src/styles.css` holds only app tweaks that use Mantine CSS variables.
- Pages are lazy-loaded with `React.lazy`. `vite.config.ts` splits `react`, `mantine`, and `icons` into long-lived vendor chunks.

## Makefile

```makefile
compose-up:
	docker compose -f deploy/docker-compose.yml up -d

seed:
	cd api && go run ./cmd/seed

api:
	cd api && go run ./cmd/api

web:
	cd web && npm install && npm run dev

test:
	cd api && go test ./...

test-integration:
	cd api && OPENFGA_API_URL=http://localhost:8080 go test -tags=integration ./internal/infrastructure/fga/
```

## Tests

`tuples_test.go` asserts the pure relation builders: role assignment writes one role and deletes the other three; a crossed affiliation/role pair emits nothing; a product toggle is only the wildcard; a created document or photo emits `tenant`, its folder or album, and `owner`; a created folder or album emits `root` at top level or `parent` below, plus `public` only when public; `Public` is the single visibility marker; grants map `external_users`/`external_admins` to their usersets, reject `internal_users` and editor for `external_users`; remove member deletes the four role relations and the member's direct grants and never a group or marker tuple.

`model_integration_test.go` (build tag `integration`, skips without `OPENFGA_API_URL`) creates a throwaway store, writes `api/fga/model.fga` and the seed tuples from `api/internal/seed`, and asserts with Check and ListObjects:

- Internal members (Bob, Cara) list every Acme folder, including private Company, Board, Specs, and Partners. Cara can edit Secret but not delete it, and cannot change visibility; Bob can.
- External users (Dan) list exactly Client and the shared Partners, and their documents. Legal (public flag under private Board) and Specs/Pricing (private under public Client) stay hidden. Dan cannot edit or create.
- External admins (Fay) create folders but not private ones, upload and edit in Client, edit Client Brief but not delete or share it, delete their own Visit Agenda, and cannot see Specs, Partners, or Company.
- Bob as a Globex external user sees only public Globex content and cannot create.
- Sharing private Board with all external admins lets Fay view Board, Q3 Plan, and Legal but not edit; Dan still cannot.
- Making Client private hides it and its content from Dan and Fay (Fay still sees her own file); making Board and Specs public reveals Board, Legal, Legal Hold, and Pricing.
- Disabling photos blocks every photo check while the owner tuple stays; re-enabling restores access.
- Removing Dan's role tuple removes all access, including the stale Partners share.

`role_integration_test.go` asserts that a bare internal admin is allowed every permission an internal user is, on every Acme object.

## Implementation order

1. Compose file, Postgres init, `.env.example`, Makefile, module skeleton.
2. Goose migration, domain entities, and gateway interfaces.
3. `api/fga/model.fga`, bootstrap, authz client, relation builders, unit tests.
4. Postgres repos, one per domain.
5. Usecase interfaces, then services for auth, platform, membership, folders, documents, albums, photos, and access, including compensation.
6. Gin handlers (one per domain), router, and `cmd/api/wire.go`.
7. `cmd/seed` with the fixed ids.
8. Integration test against OpenFGA.
9. React pages in the route table, wired only to the HTTP API above.

Stop when `make test` passes, `make test-integration` passes with compose up, and the README script's Checks match the table in this document.

## Hard rules for the implementing agent

- Copy the model DSL and the seed ids verbatim.
- Call OpenFGA on the decision-catalog relation before allowing an action.
- Return an empty collection when ListObjects is empty.
- Keep product disable to a single wildcard delete.
- Keep delete inheritance off. `can_delete` stays `can_manage`: owner or internal admin, intersected with `can_contribute` and the product wildcard.
- Keep `can_read_tenant from tenant` inside every content `can_view`.
- Keep "private wins": external visibility requires the container and every ancestor to be public. Never let a share or flag on a child bypass a private ancestor except through an explicit `viewer`/`editor` share.
- Only internal members may create private containers or change visibility.
- Keep external users out of `can_contribute`. Reject editor grants to them.
- Keep role and affiliation pairs on the legal matrix. Do not store a second role tuple for an implied relation.
- Keep system admin out of tenant content unless a later tuple assigns them a tenant or resource relation.
- Do not introduce `but not`, conditions, custom roles, raw tuple routes, or file bytes.
