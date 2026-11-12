-- +goose Up
-- Companies, derived affiliation, projects, and tenant/project role split.
-- See docs/database-schema.md.

-- ---- Companies ----
CREATE TABLE companies (
    id                   uuid PRIMARY KEY,
    name                 text NOT NULL,
    name_key             text NOT NULL,
    tax_id               text,
    created_by_tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    tenant_id            uuid UNIQUE REFERENCES tenants (id) ON DELETE CASCADE,
    claimed_at           timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (created_by_tenant_id, name_key)
);
CREATE INDEX companies_name_key_idx ON companies (name_key);

-- Every tenant owns one company; its members are the internal members.
INSERT INTO companies (id, name, name_key, created_by_tenant_id, tenant_id, claimed_at)
SELECT gen_random_uuid(), name, lower(name), id, id, now() FROM tenants;

ALTER TABLE users ADD COLUMN company_id uuid REFERENCES companies (id);

-- Internal members take their tenant's company.
UPDATE users u SET company_id = c.id
FROM (
    SELECT DISTINCT ON (m.user_id) m.user_id, c.id
    FROM memberships m JOIN companies c ON c.tenant_id = m.tenant_id
    WHERE m.affiliation = 'internal'
    ORDER BY m.user_id, m.created_at
) c
WHERE c.user_id = u.id;

-- External members without a company get one placeholder company per tenant.
INSERT INTO companies (id, name, name_key, created_by_tenant_id)
SELECT gen_random_uuid(), 'Unassigned external (' || t.name || ')', lower('Unassigned external (' || t.name || ')'), t.id
FROM tenants t
WHERE EXISTS (
    SELECT 1 FROM memberships m JOIN users u ON u.id = m.user_id
    WHERE m.tenant_id = t.id AND m.affiliation = 'external' AND u.company_id IS NULL
);
UPDATE users u SET company_id = c.id
FROM (
    SELECT DISTINCT ON (m.user_id) m.user_id, c.id
    FROM memberships m JOIN companies c ON c.created_by_tenant_id = m.tenant_id AND c.tenant_id IS NULL
    WHERE m.affiliation = 'external'
    ORDER BY m.user_id, m.created_at
) c
WHERE c.user_id = u.id AND u.company_id IS NULL;

-- Everyone else (platform admins, members of no tenant) belongs to the platform company.
INSERT INTO companies (id, name, name_key)
SELECT gen_random_uuid(), 'Platform', 'platform'
WHERE EXISTS (SELECT 1 FROM users WHERE company_id IS NULL);
UPDATE users SET company_id = (SELECT id FROM companies WHERE name_key = 'platform' AND created_by_tenant_id IS NULL)
WHERE company_id IS NULL;
ALTER TABLE users ALTER COLUMN company_id SET NOT NULL;
CREATE INDEX users_company_id_idx ON users (company_id);
CREATE UNIQUE INDEX users_email_lower_uniq ON users (lower(email));

-- ---- Tenant members: affiliation is derived from the company ----
ALTER TABLE memberships RENAME TO tenant_members;
ALTER TABLE roles RENAME TO tenant_roles;
ALTER TABLE role_permissions RENAME TO tenant_role_permissions;
ALTER TABLE tenant_members RENAME COLUMN custom_role_id TO tenant_role_id;
ALTER TABLE tenant_members DROP COLUMN affiliation;
ALTER TABLE tenant_members DROP CONSTRAINT IF EXISTS memberships_role_check;
ALTER TABLE tenant_members ADD CONSTRAINT tenant_members_role_check CHECK (
    (tenant_role_id IS NULL AND role IN ('internal_admin', 'internal_user', 'external_admin', 'external_user'))
    OR (tenant_role_id IS NOT NULL AND role = tenant_role_id::text)
);

-- ---- Projects ----
CREATE TABLE projects (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    UNIQUE (id, tenant_id)
);

-- Every existing tenant gets a default project that receives its content.
INSERT INTO projects (id, tenant_id, name) SELECT gen_random_uuid(), id, 'General' FROM tenants;

ALTER TABLE folders   ADD COLUMN project_id uuid;
ALTER TABLE documents ADD COLUMN project_id uuid;
ALTER TABLE albums    ADD COLUMN project_id uuid;
ALTER TABLE photos    ADD COLUMN project_id uuid;
UPDATE folders   x SET project_id = p.id FROM projects p WHERE p.tenant_id = x.tenant_id;
UPDATE documents x SET project_id = p.id FROM projects p WHERE p.tenant_id = x.tenant_id;
UPDATE albums    x SET project_id = p.id FROM projects p WHERE p.tenant_id = x.tenant_id;
UPDATE photos    x SET project_id = p.id FROM projects p WHERE p.tenant_id = x.tenant_id;
ALTER TABLE folders   ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE documents ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE albums    ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE photos    ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE folders   ADD CONSTRAINT folders_project_fk   FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE;
ALTER TABLE documents ADD CONSTRAINT documents_project_fk FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE;
ALTER TABLE albums    ADD CONSTRAINT albums_project_fk    FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE;
ALTER TABLE photos    ADD CONSTRAINT photos_project_fk    FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE;
CREATE INDEX folders_project_id_idx   ON folders (project_id);
CREATE INDEX documents_project_id_idx ON documents (project_id);
CREATE INDEX albums_project_id_idx    ON albums (project_id);
CREATE INDEX photos_project_id_idx    ON photos (project_id);

-- ---- Project roles and members ----
CREATE TABLE project_roles (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    project_id  uuid NOT NULL,
    name        text NOT NULL,
    affiliation text NOT NULL CHECK (affiliation IN ('internal', 'external')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name),
    FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE
);

CREATE TABLE project_role_permissions (
    role_id    uuid NOT NULL REFERENCES project_roles (id) ON DELETE CASCADE,
    permission text NOT NULL CHECK (permission IN (
        'view', 'edit', 'manage', 'change_visibility',
        'contribute', 'create_private', 'manage_members', 'view_products'
    )),
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE project_members (
    project_id      uuid NOT NULL,
    tenant_id       uuid NOT NULL,
    user_id         uuid NOT NULL,
    role            text NOT NULL,
    project_role_id uuid REFERENCES project_roles (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id),
    FOREIGN KEY (project_id, tenant_id) REFERENCES projects (id, tenant_id) ON DELETE CASCADE,
    -- Only members of the tenant can be project members.
    FOREIGN KEY (tenant_id, user_id) REFERENCES tenant_members (tenant_id, user_id) ON DELETE CASCADE,
    CHECK (
        (project_role_id IS NULL AND role IN ('internal_admin', 'internal_user', 'external_admin', 'external_user'))
        OR (project_role_id IS NOT NULL AND role = project_role_id::text)
    )
);
CREATE INDEX project_members_user_idx ON project_members (tenant_id, user_id);

-- +goose Down
DROP TABLE project_members;
DROP TABLE project_role_permissions;
DROP TABLE project_roles;
ALTER TABLE photos    DROP COLUMN project_id;
ALTER TABLE albums    DROP COLUMN project_id;
ALTER TABLE documents DROP COLUMN project_id;
ALTER TABLE folders   DROP COLUMN project_id;
DROP TABLE projects;

ALTER TABLE tenant_members ADD COLUMN affiliation text;
UPDATE tenant_members m SET affiliation = CASE
    WHEN EXISTS (SELECT 1 FROM users u JOIN companies c ON c.id = u.company_id
                 WHERE u.id = m.user_id AND c.tenant_id = m.tenant_id) THEN 'internal' ELSE 'external' END;
ALTER TABLE tenant_members ALTER COLUMN affiliation SET NOT NULL;
ALTER TABLE tenant_members ADD CONSTRAINT tenant_members_aff_check CHECK (affiliation IN ('internal', 'external'));
ALTER TABLE tenant_members DROP CONSTRAINT tenant_members_role_check;
ALTER TABLE tenant_members RENAME COLUMN tenant_role_id TO custom_role_id;
ALTER TABLE tenant_role_permissions RENAME TO role_permissions;
ALTER TABLE tenant_roles RENAME TO roles;
ALTER TABLE tenant_members RENAME TO memberships;
ALTER TABLE memberships ADD CONSTRAINT memberships_role_check CHECK (
    (custom_role_id IS NULL AND (
        (affiliation = 'internal' AND role IN ('internal_admin', 'internal_user'))
        OR (affiliation = 'external' AND role IN ('external_admin', 'external_user'))))
    OR (custom_role_id IS NOT NULL AND role = custom_role_id::text)
);

DROP INDEX users_email_lower_uniq;
ALTER TABLE users DROP COLUMN company_id;
DROP TABLE companies;
