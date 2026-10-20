-- +goose Up
-- Custom roles are tenant-scoped. The four built-in roles stay on memberships.
-- permissions and assignees are the display record of the grant_* and assignee tuples.
CREATE TABLE roles (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE role_permissions (
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission text NOT NULL CHECK (permission IN (
        'view', 'edit', 'manage', 'change_visibility',
        'contribute', 'create_private', 'manage_members', 'view_products'
    )),
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE role_assignees (
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, user_id)
);
CREATE INDEX role_assignees_user_id_idx ON role_assignees (user_id);

-- +goose Down
DROP TABLE role_assignees;
DROP TABLE role_permissions;
DROP TABLE roles;
