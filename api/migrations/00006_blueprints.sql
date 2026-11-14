-- +goose Up
-- Blueprints: a third product. Versions are append-only display records; the
-- OpenFGA tuples are only tenant, project, owner, and shares.
-- See docs/database-schema.md and prds/plans/text2blueprint-poc.md.

ALTER TABLE tenant_products DROP CONSTRAINT tenant_products_product_check;
ALTER TABLE tenant_products ADD CONSTRAINT tenant_products_product_check
    CHECK (product IN ('documents', 'photos', 'blueprints'));
INSERT INTO tenant_products (tenant_id, product, enabled)
SELECT id, 'blueprints', false FROM tenants
ON CONFLICT DO NOTHING;

ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_object_type_check;
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_object_type_check
    CHECK (object_type IN ('folder', 'document', 'album', 'photo', 'blueprint'));

CREATE TABLE blueprints (
    id              uuid PRIMARY KEY,
    tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    project_id      uuid NOT NULL,
    name            text NOT NULL,
    current_version integer NOT NULL DEFAULT 1 CHECK (current_version >= 1),
    source_kind     text NOT NULL CHECK (source_kind IN ('prompt', 'sketch', 'form')),
    source_name     text,
    source_mime     text,
    source_size     bigint CHECK (source_size >= 0),
    created_by      uuid NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT blueprints_project_fk FOREIGN KEY (project_id, tenant_id)
        REFERENCES projects (id, tenant_id) ON DELETE CASCADE
);
CREATE INDEX blueprints_tenant_project_idx ON blueprints (tenant_id, project_id);

CREATE TABLE blueprint_versions (
    blueprint_id uuid NOT NULL REFERENCES blueprints (id) ON DELETE CASCADE,
    version      integer NOT NULL CHECK (version >= 1),
    kind         text NOT NULL CHECK (kind IN ('create', 'revise', 'edit', 'restore')),
    message      text NOT NULL DEFAULT '',
    brief        jsonb NOT NULL,
    model        jsonb NOT NULL,
    checks       jsonb NOT NULL,
    notes        jsonb NOT NULL DEFAULT '[]',
    ok           boolean NOT NULL,
    created_by   uuid NOT NULL REFERENCES users (id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blueprint_id, version)
);

-- +goose Down
DROP TABLE IF EXISTS blueprint_versions;
DROP TABLE IF EXISTS blueprints;
DELETE FROM resource_grants WHERE object_type = 'blueprint';
ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_object_type_check;
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_object_type_check
    CHECK (object_type IN ('folder', 'document', 'album', 'photo'));
DELETE FROM tenant_products WHERE product = 'blueprints';
ALTER TABLE tenant_products DROP CONSTRAINT tenant_products_product_check;
ALTER TABLE tenant_products ADD CONSTRAINT tenant_products_product_check
    CHECK (product IN ('documents', 'photos'));
