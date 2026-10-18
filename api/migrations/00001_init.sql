-- +goose Up
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

-- +goose Down
DROP TABLE IF EXISTS resource_grants;
DROP TABLE IF EXISTS photos;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS tenant_products;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS platform_admins;
DROP TABLE IF EXISTS users;
