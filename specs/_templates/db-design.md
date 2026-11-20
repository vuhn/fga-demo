# DB design: <feature>

Must not break the running system: old code keeps working against the new schema (expand), new code against the old data (backfill), and the migration can be rolled forward without downtime.

## Current schema touched
<tables/columns, from `docs/database-schema.md` and the latest migration>

## Changes
| Table | Change | Backward compatible because |
| --- | --- | --- |
| | | |

## Migration steps (expand -> backfill -> contract)
1. Migration `000NN_<name>.sql` (Up/Down): additive only.
2. Backfill: <separate step, batched if large>.
3. Contract (later release, only after no code reads the old shape): <drop/rename>.

## Checks
- [ ] No drop/rename of used columns or tables; no enum value changes; no NOT NULL -> NULL; no new NOT NULL without DEFAULT
- [ ] Every tenant-owned table has `tenant_id` and an index that leads with it
- [ ] FKs, uniqueness, CHECK constraints, ON DELETE behavior chosen on purpose
- [ ] Indexes for every new query path (list by tenant + `id = ANY($2)`)
- [ ] Down migration works, or is explicitly irreversible and why
- [ ] `Truncate` in the seed lists the new tables (FK order)
- [ ] `docs/database-schema.md`, `docs/seed-data.md` updated
- [ ] Dry-run in a rolled-back transaction on the live schema

## `force-allow-breaking` exceptions
<each `-- force-allow-breaking: reason` used, and the release order that makes it safe>
