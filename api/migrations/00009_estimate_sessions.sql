-- +goose Up
-- Cost estimation: a chat session per blueprint version (the engine's agent state is opaque JSON), its
-- messages, and the finished estimates. No price tables: unit rates are researched per estimate and
-- stored with their sources inside the estimate. See prds/plans/cost-estimation.md.
CREATE TABLE estimate_sessions (
    id                uuid PRIMARY KEY,
    tenant_id         uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    blueprint_id      uuid NOT NULL,
    blueprint_version integer NOT NULL,
    status            text NOT NULL CHECK (status IN ('interviewing', 'researching', 'ready', 'failed')),
    state             jsonb NOT NULL DEFAULT '{}',
    seq               integer NOT NULL DEFAULT 0 CHECK (seq >= 0),
    created_by        uuid NOT NULL REFERENCES users (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (blueprint_id, blueprint_version) REFERENCES blueprint_versions (blueprint_id, version) ON DELETE CASCADE
);
CREATE INDEX estimate_sessions_blueprint_idx ON estimate_sessions (blueprint_id, created_at DESC);

CREATE TABLE estimate_messages (
    session_id uuid NOT NULL REFERENCES estimate_sessions (id) ON DELETE CASCADE,
    seq        integer NOT NULL CHECK (seq >= 1),
    role       text NOT NULL CHECK (role IN ('user', 'assistant')),
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, seq)
);

-- Append-only: one row per priced estimate, with every rate and its sources inside result.
CREATE TABLE blueprint_estimates (
    id           uuid PRIMARY KEY,
    session_id   uuid NOT NULL REFERENCES estimate_sessions (id) ON DELETE CASCADE,
    blueprint_id uuid NOT NULL,
    version      integer NOT NULL,
    total_low    bigint NOT NULL,
    total_base   bigint NOT NULL,
    total_high   bigint NOT NULL,
    result       jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (blueprint_id, version) REFERENCES blueprint_versions (blueprint_id, version) ON DELETE CASCADE
);
CREATE INDEX blueprint_estimates_blueprint_idx ON blueprint_estimates (blueprint_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS blueprint_estimates;
DROP TABLE IF EXISTS estimate_messages;
DROP TABLE IF EXISTS estimate_sessions;
