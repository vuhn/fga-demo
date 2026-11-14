-- +goose Up
-- A blueprint can be imported from a DXF: its version has kind 'import' and no brief (JSON null),
-- and the original file is kept next to the version. See prds/plans/text2blueprint-production.md.
ALTER TABLE blueprint_versions DROP CONSTRAINT blueprint_versions_kind_check;
ALTER TABLE blueprint_versions ADD CONSTRAINT blueprint_versions_kind_check
    CHECK (kind IN ('create', 'revise', 'review', 'edit', 'restore', 'import'));
ALTER TABLE blueprints DROP CONSTRAINT blueprints_source_kind_check;
ALTER TABLE blueprints ADD CONSTRAINT blueprints_source_kind_check
    CHECK (source_kind IN ('prompt', 'sketch', 'form', 'import'));

CREATE TABLE blueprint_originals (
    blueprint_id uuid NOT NULL,
    version      integer NOT NULL,
    name         text NOT NULL,
    mime_type    text NOT NULL,
    byte_size    bigint NOT NULL CHECK (byte_size > 0),
    sha256       text NOT NULL,
    data         bytea NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blueprint_id, version),
    FOREIGN KEY (blueprint_id, version) REFERENCES blueprint_versions (blueprint_id, version) ON DELETE CASCADE
);

-- +goose Down
-- Fails while imported blueprints exist, so nothing is silently dropped.
DROP TABLE IF EXISTS blueprint_originals;
ALTER TABLE blueprints DROP CONSTRAINT blueprints_source_kind_check;
ALTER TABLE blueprints ADD CONSTRAINT blueprints_source_kind_check
    CHECK (source_kind IN ('prompt', 'sketch', 'form'));
ALTER TABLE blueprint_versions DROP CONSTRAINT blueprint_versions_kind_check;
ALTER TABLE blueprint_versions ADD CONSTRAINT blueprint_versions_kind_check
    CHECK (kind IN ('create', 'revise', 'review', 'edit', 'restore'));
