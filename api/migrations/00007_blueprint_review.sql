-- +goose Up
-- "Fix issues with AI" stores its result as a version of kind 'review'.
-- See docs/database-schema.md and prds/plans/text2bim-adoption.md.
ALTER TABLE blueprint_versions DROP CONSTRAINT blueprint_versions_kind_check;
ALTER TABLE blueprint_versions ADD CONSTRAINT blueprint_versions_kind_check
    CHECK (kind IN ('create', 'revise', 'review', 'edit', 'restore'));

-- +goose Down
-- Fails while review versions exist, so no version is silently dropped.
ALTER TABLE blueprint_versions DROP CONSTRAINT blueprint_versions_kind_check;
ALTER TABLE blueprint_versions ADD CONSTRAINT blueprint_versions_kind_check
    CHECK (kind IN ('create', 'revise', 'edit', 'restore'));
