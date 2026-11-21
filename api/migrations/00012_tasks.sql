-- +goose Up
-- Tasks: per-project work items and their comments, plus the `tasks` product.
-- Display records only; authorization stays in OpenFGA (tuples: tasks_enabled per
-- tenant, owner/project/tenant per comment). Nothing reads these tables until the
-- task routes ship. See specs/task-management/db-design.md.

CREATE TABLE tasks (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    project_id    uuid NOT NULL,
    title         text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description   text NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    assignee_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    planned_start date,
    planned_end   date,
    actual_start  date,
    actual_end    date,
    status        text NOT NULL DEFAULT 'not_started'
        CHECK (status IN ('not_started', 'in_progress', 'completed', 'blocked')),
    weight        numeric(8,2) NOT NULL DEFAULT 1 CHECK (weight > 0),
    created_by    uuid NOT NULL REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tasks_project_fk FOREIGN KEY (project_id, tenant_id)
        REFERENCES projects (id, tenant_id) ON DELETE CASCADE,
    CONSTRAINT tasks_planned_range_check CHECK (planned_end >= planned_start),
    CONSTRAINT tasks_actual_range_check CHECK (actual_end >= actual_start)
);
CREATE INDEX tasks_tenant_project_idx ON tasks (tenant_id, project_id, created_at, id);

CREATE TABLE task_comments (
    id         uuid PRIMARY KEY,
    task_id    uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    body       text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
    author_id  uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at  timestamptz
);
CREATE INDEX task_comments_task_created_idx ON task_comments (task_id, created_at, id);

SET LOCAL lock_timeout = '5s';

-- force-allow-breaking: widening a closed set only; old code never writes 'tasks', and reads of the extra row are harmless
ALTER TABLE tenant_products DROP CONSTRAINT tenant_products_product_check;
ALTER TABLE tenant_products ADD CONSTRAINT tenant_products_product_check
    CHECK (product IN ('documents', 'photos', 'blueprints', 'tasks')) NOT VALID;
ALTER TABLE tenant_products VALIDATE CONSTRAINT tenant_products_product_check;
INSERT INTO tenant_products (tenant_id, product, enabled)
SELECT id, 'tasks', false FROM tenants
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS tasks;
DELETE FROM tenant_products WHERE product = 'tasks';
ALTER TABLE tenant_products DROP CONSTRAINT tenant_products_product_check;
ALTER TABLE tenant_products ADD CONSTRAINT tenant_products_product_check
    CHECK (product IN ('documents', 'photos', 'blueprints'));
