-- +goose Up
-- A custom role is internal or external, and a member holds one role: a
-- built-in role, or one custom role of the same affiliation.
ALTER TABLE roles ADD COLUMN affiliation text NOT NULL DEFAULT 'internal'
    CHECK (affiliation IN ('internal', 'external'));
ALTER TABLE roles ALTER COLUMN affiliation DROP DEFAULT;

ALTER TABLE memberships ADD COLUMN custom_role_id uuid REFERENCES roles (id);

ALTER TABLE memberships DROP CONSTRAINT memberships_check;
ALTER TABLE memberships ADD CONSTRAINT memberships_role_check CHECK (
    (
        custom_role_id IS NULL
        AND (
            (affiliation = 'internal' AND role IN ('internal_admin', 'internal_user'))
            OR (affiliation = 'external' AND role IN ('external_admin', 'external_user'))
        )
    )
    OR (custom_role_id IS NOT NULL AND role = custom_role_id::text)
);

-- Who holds a custom role is the membership row, same as a built-in role.
DROP TABLE role_assignees;

-- +goose Down
CREATE TABLE role_assignees (
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, user_id)
);
CREATE INDEX role_assignees_user_id_idx ON role_assignees (user_id);

ALTER TABLE memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE memberships ADD CONSTRAINT memberships_check CHECK (
    (affiliation = 'internal' AND role IN ('internal_admin', 'internal_user'))
    OR (affiliation = 'external' AND role IN ('external_admin', 'external_user'))
);
ALTER TABLE memberships DROP COLUMN custom_role_id;
ALTER TABLE roles DROP COLUMN affiliation;
