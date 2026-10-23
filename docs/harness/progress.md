# Progress

Single place for "where are we". Read at session start, update at session end.

## Current verified state

- Static tier: `make check` passes (vet, unit tests, architecture rules, docs/model sync, web build). Verified 2026-10-23 on `main` (ae789fc + uncommitted harness changes).
- Live tier: `make test-integration` passes against live OpenFGA (TestModelAgainstSeed, TestInternalAdminCoversInternalUser, TestCustomRoleGrant, TestSeedCustomRolesHaveNoAssignees). Verified 2026-10-23.
- README script coverage: steps 2-11 and the custom-role behavior are asserted by `TestModelAgainstSeed`, `TestInternalAdminCoversInternalUser`, `TestCustomRoleGrant`. Step 1 (platform admin lists tenants, creates users) has no automated test (gap). No README step has been walked in the UI yet.
- Active task: none.

## Last session

- Added the harness: `make check`, `make verify-live`, `make init`, `scripts/check-architecture.sh`, `docs/harness/*`, `.claude/agents/authz-reviewer.md`, `AGENTS.md`.
- Ran the live tier; dropped `feature-list.json` (tests are the source of truth).
- No product, model, or API behavior changed.

## Blockers

None.

## Next best action

1. Walk README steps 1-11 in the UI (after `make seed` and an API restart) and note any divergence here.
2. Close the S01 gap: an HTTP-level test for platform-admin tenant list and user creation.
3. Commit the harness (nothing from this work is committed yet).
