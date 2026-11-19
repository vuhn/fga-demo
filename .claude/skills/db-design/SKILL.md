---
name: db-design
description: Stage 3 of the feature SDLC. Design production-safe Postgres schema changes (goose migrations) that never break the running system; writes specs/<feature>/db-design.md. Use whenever a feature adds or changes tables, columns, indexes, constraints, or data.
---

# DB design

Input: approved `system-design.md`. Output: `specs/<feature>/db-design.md` (template), then migration SQL during build.

## Read first
`docs/database-schema.md`, the newest files in `api/migrations/` (goose, embedded, applied on API start; next number follows the highest), `api/internal/infrastructure/postgres/` repos, `docs/seed-data.md`. Postgres ids equal OpenFGA object ids (real UUIDs).

## Never break the running system
The old code and the new code both run against the schema during a rollout. So:
- Forbidden in one step: drop/rename a column or table in use, change enum/CHECK value sets, NOT NULL -> NULL, new NOT NULL column without DEFAULT, narrowing a type, adding a UNIQUE/FK that existing rows violate.
- Use expand -> backfill -> contract: add nullable or defaulted columns/tables first; backfill separately (batched for big tables); remove the old shape only in a later release after no code reads it. Mark any unavoidable exception with `-- breaking-ok: <reason>` and justify it in the doc (the linter `scripts/check-migrations.sh` rejects the rest).
- Never edit a migration that is committed; add a new one (a hook blocks it).

## Production-level checklist
- Tenant-owned table: `tenant_id uuid NOT NULL`, queries are `WHERE tenant_id = $1 AND id = ANY($2)`, index leads with `tenant_id`.
- Keys and constraints chosen on purpose: PK, FK with ON DELETE, UNIQUE, CHECK; timestamps `timestamptz`; JSON only for opaque engine state.
- Index every new query path; `CREATE INDEX` on a large table should be `CONCURRENTLY` (goose `-- +goose NO TRANSACTION`).
- Postgres is the display record; authorization lives in OpenFGA, so do not add role/affiliation columns that code branches on.
- Each domain owns its tables (literal table names in its repo); no generic shared tables.
- Down migration works or is explicitly irreversible.
- Seed: add new tables to `Truncate` (FK order; `make seed` fails otherwise) and to `internal/seed/world.go` if demo data is needed.
- Update `docs/database-schema.md` and `docs/seed-data.md` in the same change.
- Dry-run the migration on the live schema inside a rolled-back transaction before claiming it works.

## Output
Fill the template: current schema touched, change table with "backward compatible because", the expand/backfill/contract steps and which release each lands in, the checklist, exceptions. Run `design-reviewer`, then gate: user approves.
