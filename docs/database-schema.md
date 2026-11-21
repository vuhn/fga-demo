# Database schema

Postgres is the display record. Authorization never reads `tenant_members.role` or any company data. Overview: [api/ARCHITECTURE.md](../api/ARCHITECTURE.md). Migrations live in `api/migrations/`.

## Entity relationship diagram

Current schema after migration 00011. Columns are abbreviated to keys, foreign keys, and the fields that matter for authorization display. Composite foreign keys `(project_id, tenant_id)` and `(tenant_id, user_id)` are drawn as one relationship.

```mermaid
erDiagram
    users {
        uuid id PK
        text email UK "unique on lower(email)"
        text name
        uuid company_id FK
    }
    platform_admins {
        uuid user_id PK, FK
    }
    companies {
        uuid id PK
        text name
        text name_key "unique per created_by_tenant_id"
        text tax_id
        uuid created_by_tenant_id FK "null for own company"
        uuid tenant_id FK, UK "set = tenant's own or claimed"
        timestamptz claimed_at
    }
    tenants {
        uuid id PK
        text slug UK
        text name
    }
    tenant_products {
        uuid tenant_id PK, FK
        text product PK "documents | photos | blueprints | tasks"
        boolean enabled
    }
    tenant_members {
        uuid tenant_id PK, FK
        uuid user_id PK, FK
        text role "built-in role or tenant_role id"
        uuid tenant_role_id FK
    }
    tenant_roles {
        uuid id PK
        uuid tenant_id FK
        text name "unique per tenant"
        text affiliation "internal | external"
    }
    tenant_role_permissions {
        uuid role_id PK, FK
        text permission PK
    }
    role_assignees {
        uuid role_id PK, FK
        uuid user_id PK, FK
    }
    projects {
        uuid id PK
        uuid tenant_id FK "unique with name and with id"
        text name
        uuid created_by FK
    }
    project_roles {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        text name "unique per project"
        text affiliation "internal | external"
    }
    project_role_permissions {
        uuid role_id PK, FK
        text permission PK
    }
    project_members {
        uuid project_id PK, FK
        uuid user_id PK, FK
        uuid tenant_id FK
        text role "built-in role or project_role id"
        uuid project_role_id FK
    }
    folders {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        uuid parent_id FK
        text name
        boolean is_private
        uuid created_by FK
    }
    documents {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        uuid folder_id FK
        text name
        uuid created_by FK
    }
    albums {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        uuid parent_id FK
        text name
        boolean is_private
        uuid created_by FK
    }
    photos {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        uuid album_id FK
        text name
        uuid created_by FK
    }
    blueprints {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        text name
        int current_version
        text source_kind "prompt | sketch | form | import"
        uuid created_by FK
    }
    tasks {
        uuid id PK
        uuid tenant_id FK
        uuid project_id FK
        text title
        text description
        uuid assignee_id FK "null"
        date planned_start "null"
        date planned_end "null"
        date actual_start "null"
        date actual_end "null"
        text status "not_started | in_progress | completed | blocked"
        numeric weight
        uuid created_by FK
    }
    task_comments {
        uuid id PK
        uuid task_id FK
        text body
        uuid author_id FK
        timestamptz edited_at "null"
    }
    blueprint_versions {
        uuid blueprint_id PK
        int version PK
        text kind "create | revise | review | edit | restore | import"
        jsonb brief
        jsonb model
        uuid created_by FK
    }
    estimate_sessions {
        uuid blueprint_id PK "one session per blueprint"
        int blueprint_version FK
        jsonb state "agent state, status inside"
        int seq "finished turns"
        int epoch "restarts"
        uuid created_by FK
    }
    estimate_messages {
        uuid blueprint_id PK
        int seq PK
        text role "user | assistant"
        text content
    }
    blueprint_estimates {
        uuid id PK
        uuid blueprint_id FK
        int version FK
        bigint total_base
        jsonb result
    }
    resource_grants {
        uuid id PK
        uuid tenant_id FK
        text object_type "folder | document | album | photo | blueprint"
        uuid object_id "no FK, type varies"
        text relation "owner | editor | viewer"
        text subject_type "user | external_users | external_admins"
        uuid user_id FK
    }

    companies ||--o{ users : "employs (company_id)"
    tenants |o--o| companies : "own company (tenant_id)"
    tenants |o--o{ companies : "created subcontractors"
    users ||--o| platform_admins : is
    tenants ||--o{ tenant_products : enables
    tenants ||--o{ tenant_members : has
    users ||--o{ tenant_members : "member of"
    tenants ||--o{ tenant_roles : defines
    tenant_roles ||--o{ tenant_role_permissions : grants
    tenant_roles ||--o{ role_assignees : assigned
    users ||--o{ role_assignees : holds
    tenant_roles |o--o{ tenant_members : "custom role"
    tenants ||--o{ projects : owns
    projects ||--o{ project_roles : defines
    project_roles ||--o{ project_role_permissions : grants
    projects ||--o{ project_members : has
    tenant_members ||--o{ project_members : "only tenant members"
    project_roles |o--o{ project_members : "custom role"
    projects ||--o{ folders : contains
    projects ||--o{ albums : contains
    projects ||--o{ documents : contains
    projects ||--o{ photos : contains
    projects ||--o{ blueprints : contains
    blueprints ||--o{ blueprint_versions : versions
    projects ||--o{ tasks : contains
    users |o--o{ tasks : "assignee_id"
    tasks ||--o{ task_comments : has
    blueprint_versions ||--o{ estimate_sessions : pinned
    estimate_sessions ||--o{ estimate_messages : chat
    blueprint_versions ||--o{ blueprint_estimates : priced
    folders ||--o{ folders : parent
    folders ||--o{ documents : holds
    albums ||--o{ albums : parent
    albums ||--o{ photos : holds
    tenants ||--o{ resource_grants : scopes
    users |o--o{ resource_grants : "user grants"
```

Affiliation is not stored on `tenant_members`: a member is internal iff `users.company_id` equals the tenant's own company (`companies.tenant_id`). Authorization still comes from OpenFGA, not from these rows.

## Postgres schema

Postgres is the display record and the membership label. Authorization reads do not use `tenant_members.role`.

This is the 00001 baseline; later migrations (below) rename and extend it. Apply it as `api/migrations/00001_init.sql` with goose `-- +goose Up` and a matching Down that drops tables in reverse dependency order.

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
    product    text NOT NULL CHECK (product IN ('documents', 'photos', 'blueprints', 'tasks')),
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

`api/migrations/00003_custom_roles.sql` adds tenant-scoped custom roles. `roles.name` is unique per tenant. `role_permissions.permission` is one of `view`, `edit`, `manage`, `change_visibility`, `contribute`, `create_private`, `manage_members`, `view_products`. `api/migrations/00004_role_affiliation.sql` adds `roles.affiliation` (`internal` or `external`) and `memberships.custom_role_id`. A member holds one role: a built-in role with `custom_role_id` null, or a custom role whose id is stored in both `role` and `custom_role_id`. These rows display the tuples; OpenFGA still decides.

## Companies, projects, and tenant/project roles (migration 00005)

Spec for `prds/update-tenant-structure.md`. `api/migrations/00005_companies_projects.sql` applies it to existing data (see Data migration) and the seed reseeds from scratch.

**Companies.** `companies(id, name, name_key, tax_id null, created_by_tenant_id null, tenant_id null unique, claimed_at null, created_at)`, unique on `(created_by_tenant_id, name_key)`.

- Every tenant owns exactly one company (`tenant_id` set), inserted by `TenantRepo.Create`. Its people are the internal members.
- A subcontractor a tenant creates on its behalf is an unclaimed company: `tenant_id` null, `created_by_tenant_id` = the creator. It is a display record with no tuples, not a tenant: it has no contract, admins, products, or roles.
- The same real company created by two tenants is two rows. A tenant cannot verify another tenant's data, and one shared editable row would leak or alter data across tenants. Duplicates are flagged (same name) for the platform admin and merged at claim.
- `users.company_id uuid NOT NULL`: one person, one account, one employer. Email stays unique, case-insensitively (`users_email_lower_uniq`). Add-member looks the email up first and reuses the account.

**Claim.** The platform admin has verified ownership out of band (SSO is a non-goal; for example a verified email at the company or a matching `tax_id`). `POST /tenants/:id/company-claims` merges the named unclaimed companies (the one other tenants created and confirmed duplicates) into the claiming tenant's own company in one transaction: users move to it, the unclaimed rows are deleted. Memberships in other tenants are untouched, so those users stay external there, nothing in OpenFGA changes, and the claiming tenant takes over the company and its people. The claim is a 409 when any of those users is already a member of the claiming tenant (their affiliation there would flip); remove them first.

**Affiliation is derived, not stored.** A member is internal iff `users.company_id` points at the tenant's own company; otherwise external. `memberships.affiliation` is dropped. Repos compute it (`affiliationExpr` in `membership_repo.go`) and the API still returns `affiliation` as a display label. OpenFGA still gets the `internal_*` or `external_*` relation chosen at add time. Bob is internal in Acme (his company) and external in Globex, one account.

**Renames.** `memberships` -> `tenant_members` (`custom_role_id` -> `tenant_role_id`, no `affiliation`), `roles` -> `tenant_roles`, `role_permissions` -> `tenant_role_permissions`. FGA type `role` -> `tenant_role`. Go entity and usecase names for tenant roles were kept (`Role`, `RoleLister`); project roles are `ProjectRole*`.

**Projects.** `projects(id, tenant_id, name, created_by, created_at)`, unique `(tenant_id, name)` and `(id, tenant_id)`. `folders`, `albums`, `documents`, `photos` get `project_id uuid NOT NULL` with composite FK `(project_id, tenant_id) -> projects (id, tenant_id)`, so content cannot sit in another tenant's project. `tenant_id` stays on content for the `tenant_id = $1 AND id = ANY($2)` rule.

**Project roles and members.**

- `project_roles(id, tenant_id, project_id, name, affiliation)`, unique `(project_id, name)`: same shape as tenant roles but scoped to one project. `project_role_permissions(role_id, permission)` uses the same catalog.
- `project_members(project_id, tenant_id, user_id, role, project_role_id null, created_at)`, PK `(project_id, user_id)`. FK `(tenant_id, user_id) -> tenant_members` (only tenant members, and removing the tenant membership cascades), FK `(project_id, tenant_id) -> projects`. `role` is a built-in role name, or the project role id when `project_role_id` is set (CHECK).
- Role-type rule, for tenant and project assignment alike: an internal role goes only to an internal member, an external role only to an external member. The service checks the role against the member's derived affiliation (`CheckAffiliationRole`, and the custom role's `affiliation`), answering 400 `affiliation_role_mismatch`.

**Effective permissions** in a project are the union of the tenant role's and the project role's grants, evaluated by OpenFGA (see the authorization model). A tenant member who is not a project member: internal members keep tenant-wide access; external members get nothing in that project (open requirement, decided as this default).

**Data migration.** One company per tenant; `users.company_id` = the company of the tenant where the user is internal; external members with no company join a placeholder "Unassigned external (<tenant>)" company per tenant that the tenant admin can rename; everyone else (platform admins) joins a "Platform" company. Each tenant gets a default project "General" that receives all existing containers and content. Role tables are renamed in place. No project members are created, so existing external members must be added to a project; re-run `make seed` for the demo.

`resource_grants` includes owners. A group share cannot use `user_id`. `external_users` maps to `tenant:<id>#external_user` and is viewer only, because an external user cannot upload. `external_admins` maps to `tenant:<id>#external_admin`. One group row per object and subject type; changing viewer to editor replaces that row.

`is_private` is the display flag. The `public` tuple is the authorization record (see the tuple catalog).

`created_by` is a display field. The owner tuple is the authorization record. They can diverge after a member is removed: the row keeps `created_by`, and the owner tuple is deleted.

There is no foreign key from `resource_grants.object_id` to a single table because the object type varies. The content repo writes the owner grant in the same transaction as the object and deletes grants when the object is deleted.

## Blueprints (migration 00006)

`api/migrations/00006_blueprints.sql`. Spec: `prds/plans/text2blueprint-poc.md`.

- `blueprints(id, tenant_id, project_id, name, current_version, source_kind, source_name null, source_mime null, source_size null, created_by, created_at, updated_at)`. FK `(project_id, tenant_id) -> projects (id, tenant_id)` like other content. `source_*` describe an uploaded file (DXF, PDF or picture) for display; only the DXF read from it is stored, in `blueprint_originals`.
- `blueprint_versions(blueprint_id, version, kind, message, brief jsonb, model jsonb, checks jsonb, notes jsonb, ok, created_by, created_at)`, PK `(blueprint_id, version)`, cascade on blueprint delete. Append-only. The brief is the only editable source; `model` is what the engine derived from it and is stored so a drawing re-renders identically and the next revision keeps rooms in place.
- Appending a version is one transaction: `UPDATE blueprints SET current_version = base + 1 WHERE id = $1 AND current_version = base`, then insert `version = base + 1`. Zero rows updated is 409 `conflict` and nothing is inserted.
- `tenant_products.product` adds `blueprints` (one `false` row per existing tenant); `resource_grants.object_type` adds `blueprint`.

## Blueprint reviews (migration 00007)

`api/migrations/00007_blueprint_review.sql` widens `blueprint_versions.kind` to include `review` (a version written by "Fix issues with AI"). Spec: `prds/plans/text2bim-adoption.md`. Down restores the old check; it fails while any `review` row exists, so no version is silently dropped.


## Blueprint imports (migration 00008)

`api/migrations/00008_blueprint_import.sql` adds `import` to `blueprint_versions.kind` and `blueprints.source_kind`, and the table `blueprint_originals(blueprint_id, version, name, mime_type, byte_size > 0, sha256, data bytea, created_at)`, PK `(blueprint_id, version)`, FK to `blueprint_versions` with cascade (deleting the blueprint deletes its files). A blueprint made from a brief before DXF plans gets its original later: converting it appends a version (`kind = edit`, `brief = null`) and stores the drawn DXF as `blueprint_originals` of that version, so a blueprint's file is its newest `blueprint_originals` row, not always version 1. A created plan is stored the same way (version 1 `kind = create`, `brief = null`, the DXF the engine drew as its original; its `source_kind` stays `prompt` or `form`, with no `source_*`). An imported version stores `brief = null` (JSON null; the column stays `NOT NULL`) and the plan as `model`. The original is written in the same transaction as the blueprint and version 1; the API caps it at 20 MB. Spec: `prds/plans/text2blueprint-production.md`. Down drops the table and restores the checks; it fails while import rows exist.

## Cost estimation (migrations 00009 to 00011)

`api/migrations/00009_estimate_sessions.sql`, `00010_one_estimate_session.sql`, `00011_session_per_blueprint.sql` (the shape below). Spec: `prds/plans/cost-estimation.md`. There are no price tables: unit rates are researched by the engine per estimate (LLM web search) and stored with their sources inside the estimate.

- `estimate_sessions(blueprint_id PK, blueprint_version, state jsonb, seq, epoch, created_by, created_at, updated_at)`. **One row per blueprint**: the blueprint id is the key, so there is no surrogate id, no `tenant_id` (the blueprint has it; every service scopes by the blueprint) and no `status` column (the agent's status lives in `state`, which Go reads for the API view). FK `(blueprint_id, blueprint_version) -> blueprint_versions` with cascade, so deleting a blueprint deletes its session. `state` is the agent's own JSON (facts with their source, the condition of each room, researched rates, the latest estimate, `status`); Go treats it as opaque. Starting a new estimate is `INSERT ... ON CONFLICT (blueprint_id) DO UPDATE` (state cleared, `seq = 0`, `epoch + 1`) plus deleting the messages, in one transaction. `seq` counts finished turns and `epoch` the restarts; together they guard concurrent writes: `UPDATE ... WHERE blueprint_id = $1 AND epoch = $2 AND seq = $3`, and zero rows updated is 409 with nothing written, so a turn that was running when the person started over can never land in the new session.
- `estimate_messages(blueprint_id, seq, role user|assistant, content, created_at)`, PK `(blueprint_id, seq)`, FK `blueprint_id -> estimate_sessions` with cascade. A turn writes two rows (`seq*2+1` and `seq*2+2`) in the same transaction as the state update.
- `blueprint_estimates(id, blueprint_id, version, total_low, total_base, total_high, result jsonb, created_at)`. Append-only history of the blueprint's priced estimates; it does not point at a session, so restarting the chat never touches it. FK `(blueprint_id, version) -> blueprint_versions`. `result` holds the priced lines, the markup cascade and every rate with its sources. The three totals are copied out of `result` on purpose: lists and the stale marker read them without loading the large jsonb, and viewers (`can_view`) read this table while the session (`can_edit`) stays private.

## Tasks (migration 00012)

`api/migrations/00012_tasks.sql`. Spec: `specs/task-management/db-design.md`. Postgres is the display record; authorization is OpenFGA (`tasks_enabled` per tenant, a `task_comment` object per comment).

- `tasks(id, tenant_id, project_id, title 1..200, description <= 10000 default '', assignee_id null, planned_start/end null, actual_start/end null, status not_started|in_progress|completed|blocked, weight numeric(8,2) > 0 default 1, created_by, created_at, updated_at)`. FK `(project_id, tenant_id) -> projects (id, tenant_id)` with cascade; `assignee_id -> users` SET NULL (the task survives). `planned_end >= planned_start` and `actual_end >= actual_start` (checked only when both dates are set). Index `tasks_tenant_project_idx (tenant_id, project_id, created_at, id)` serves the project list and progress queries.
- `task_comments(id, task_id, body 1..4000, author_id, created_at, edited_at null)`. FK `task_id -> tasks` with cascade; no `tenant_id` (every path loads the task under `tenant_id` first, as `estimate_messages`). The id equals the OpenFGA `task_comment` object id. Index `task_comments_task_created_idx (task_id, created_at, id)`.
- Progress, comment counts and assignee names are derived by query, never stored.
- `tenant_products.product` adds `tasks` (one `false` row per existing tenant). A tenant created by a build older than the task routes has no `tasks` row; a later task treats a missing row as disabled.
- Down drops both tables and the `tasks` product rows (all tasks and comments are lost) and restores the three-value check.
