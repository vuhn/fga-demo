# Database schema

Postgres is the display record. Authorization never reads `memberships.role`. Overview: [api/ARCHITECTURE.md](../api/ARCHITECTURE.md). Migrations live in `api/migrations/`.

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

