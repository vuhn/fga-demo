-- +goose Up
-- Folders and albums are private by default: only internal members see them
-- until they are made public or shared.
ALTER TABLE folders ADD COLUMN is_private boolean NOT NULL DEFAULT true;
ALTER TABLE albums ADD COLUMN is_private boolean NOT NULL DEFAULT true;

-- Internal members see all content, so the internal_users group share is gone.
DELETE FROM resource_grants WHERE subject_type = 'internal_users';
ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_check;
ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_subject_type_check;
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_subject_type_check
    CHECK (subject_type IN ('user', 'external_users', 'external_admins'));
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_check CHECK (
    (subject_type = 'user' AND user_id IS NOT NULL)
    OR (subject_type = 'external_users' AND user_id IS NULL AND relation = 'viewer')
    OR (subject_type = 'external_admins' AND user_id IS NULL AND relation IN ('viewer', 'editor'))
);

-- +goose Down
ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_check;
ALTER TABLE resource_grants DROP CONSTRAINT resource_grants_subject_type_check;
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_subject_type_check
    CHECK (subject_type IN ('user', 'internal_users', 'external_users', 'external_admins'));
ALTER TABLE resource_grants ADD CONSTRAINT resource_grants_check CHECK (
    (subject_type = 'user' AND user_id IS NOT NULL)
    OR (subject_type = 'internal_users' AND user_id IS NULL AND relation IN ('viewer', 'editor'))
    OR (subject_type = 'external_users' AND user_id IS NULL AND relation = 'viewer')
    OR (subject_type = 'external_admins' AND user_id IS NULL AND relation IN ('viewer', 'editor'))
);
ALTER TABLE albums DROP COLUMN is_private;
ALTER TABLE folders DROP COLUMN is_private;
