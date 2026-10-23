# Progress

Single place for "where are we". Read at session start, update at session end. Details of each feature are in [feature-list.json](feature-list.json).

## Current verified state

- Static tier: `make check` passes (vet, unit tests, architecture rules, docs/model sync, web build). Verified 2026-10-23 on `main` (ae789fc + uncommitted harness changes).
- Live tier: `make test-integration` passes against live OpenFGA (TestModelAgainstSeed, TestInternalAdminCoversInternalUser, TestCustomRoleGrant, TestSeedCustomRolesHaveNoAssignees). Verified 2026-10-23.
- Features: S02-S11 and R01 are `passing` on automated evidence only. S01 has no automated test (gap). No README step has been walked in the UI yet.
- Active feature: none.

## Last session

- Added the harness: `make check`, `make verify-live`, `make init`, `scripts/check-architecture.sh`, `docs/harness/*`, `.claude/agents/authz-reviewer.md`, `AGENTS.md`.
- Ran the live tier and recorded evidence in `feature-list.json`.
- No product, model, or API behavior changed.

## Blockers

None.

## Next best action

1. Walk README steps 1-11 in the UI (after `make seed` and an API restart) and note any divergence here.
2. Close the S01 gap: an HTTP-level test for platform-admin tenant list and user creation.
3. Optional: `scripts/record-evidence.sh`, called by `verify-live`, to write statuses and dates automatically instead of by hand.
4. Commit the harness (nothing from this work is committed yet).
