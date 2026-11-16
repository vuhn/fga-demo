-- +goose Up
-- The estimate session is a 1:1 child of its blueprint, so it is keyed by blueprint_id and carries nothing the
-- blueprint already says:
--   - no surrogate id (the blueprint id is the key; an `epoch` counts restarts so a turn that was running when
--     the person started over can never be saved into the new session),
--   - no tenant_id (the blueprint has it, and the services scope by the blueprint),
--   - no status column (the engine's state already holds it; Go reads it from there),
--   - blueprint_estimates no longer points at a session (an estimate belongs to the blueprint version it prices).
ALTER TABLE blueprint_estimates DROP COLUMN session_id;

ALTER TABLE estimate_messages ADD COLUMN blueprint_id uuid;
UPDATE estimate_messages m SET blueprint_id = s.blueprint_id FROM estimate_sessions s WHERE s.id = m.session_id;
ALTER TABLE estimate_messages ALTER COLUMN blueprint_id SET NOT NULL;
ALTER TABLE estimate_messages DROP CONSTRAINT estimate_messages_pkey;
ALTER TABLE estimate_messages DROP COLUMN session_id;

DROP INDEX estimate_sessions_blueprint_key;
ALTER TABLE estimate_sessions DROP CONSTRAINT estimate_sessions_pkey;
ALTER TABLE estimate_sessions DROP COLUMN id, DROP COLUMN tenant_id, DROP COLUMN status;
ALTER TABLE estimate_sessions ADD COLUMN epoch integer NOT NULL DEFAULT 1 CHECK (epoch >= 1);
ALTER TABLE estimate_sessions ADD PRIMARY KEY (blueprint_id);

ALTER TABLE estimate_messages ADD PRIMARY KEY (blueprint_id, seq);
ALTER TABLE estimate_messages ADD FOREIGN KEY (blueprint_id) REFERENCES estimate_sessions (blueprint_id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE estimate_messages DROP CONSTRAINT estimate_messages_blueprint_id_fkey;
ALTER TABLE estimate_messages DROP CONSTRAINT estimate_messages_pkey;
ALTER TABLE estimate_sessions DROP CONSTRAINT estimate_sessions_pkey;
ALTER TABLE estimate_sessions DROP COLUMN epoch;
ALTER TABLE estimate_sessions ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE estimate_sessions ADD COLUMN tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE;
UPDATE estimate_sessions s SET tenant_id = b.tenant_id FROM blueprints b WHERE b.id = s.blueprint_id;
ALTER TABLE estimate_sessions ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE estimate_sessions ADD COLUMN status text NOT NULL DEFAULT 'interviewing'
    CHECK (status IN ('interviewing', 'researching', 'ready', 'failed'));
UPDATE estimate_sessions SET status = state->>'status' WHERE state->>'status' IN ('interviewing', 'researching', 'ready', 'failed');
ALTER TABLE estimate_sessions ADD PRIMARY KEY (id);
CREATE UNIQUE INDEX estimate_sessions_blueprint_key ON estimate_sessions (blueprint_id);

ALTER TABLE estimate_messages ADD COLUMN session_id uuid;
UPDATE estimate_messages m SET session_id = s.id FROM estimate_sessions s WHERE s.blueprint_id = m.blueprint_id;
ALTER TABLE estimate_messages ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE estimate_messages DROP COLUMN blueprint_id;
ALTER TABLE estimate_messages ADD PRIMARY KEY (session_id, seq);
ALTER TABLE estimate_messages ADD FOREIGN KEY (session_id) REFERENCES estimate_sessions (id) ON DELETE CASCADE;

ALTER TABLE blueprint_estimates ADD COLUMN session_id uuid REFERENCES estimate_sessions (id) ON DELETE SET NULL;
-- Estimates cannot be matched to a session after the fact (they never recorded one in this shape), so a roll-back
-- leaves session_id NULL. Rolling further back through 00010 Down deletes those estimates: demo data only.

