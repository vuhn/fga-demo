-- +goose Up
-- One estimate session per blueprint. Starting a new estimate replaces the session (its chat and answers),
-- but finished estimates are kept: they outlive the session, so viewers still see them and the stale marker
-- still works. session_id becomes optional and is cleared when its session is deleted.
ALTER TABLE blueprint_estimates ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE blueprint_estimates DROP CONSTRAINT blueprint_estimates_session_id_fkey;
ALTER TABLE blueprint_estimates ADD CONSTRAINT blueprint_estimates_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES estimate_sessions (id) ON DELETE SET NULL;

-- Existing blueprints may have several sessions: keep the newest of each.
DELETE FROM estimate_sessions s
 USING estimate_sessions newer
 WHERE newer.blueprint_id = s.blueprint_id
   AND (newer.created_at, newer.id) > (s.created_at, s.id);

DROP INDEX IF EXISTS estimate_sessions_blueprint_idx;
CREATE UNIQUE INDEX estimate_sessions_blueprint_key ON estimate_sessions (blueprint_id);

-- +goose Down
DROP INDEX IF EXISTS estimate_sessions_blueprint_key;
CREATE INDEX estimate_sessions_blueprint_idx ON estimate_sessions (blueprint_id, created_at DESC);
ALTER TABLE blueprint_estimates DROP CONSTRAINT blueprint_estimates_session_id_fkey;
DELETE FROM blueprint_estimates WHERE session_id IS NULL;
ALTER TABLE blueprint_estimates ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE blueprint_estimates ADD CONSTRAINT blueprint_estimates_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES estimate_sessions (id) ON DELETE CASCADE;
