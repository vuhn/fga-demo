# Progress

Single place for "where are we". Read at session start, update at session end.

## Current verified state

- Static tier: `make check` passes (vet, unit tests, architecture rules, docs/model sync, web build). Verified 2026-11-12 on `main` with uncommitted companies/projects work.
- Live tier: `make verify-live` passes against live compose (TestModelAgainstSeed, TestProjectRoleUnion, TestProjectMembershipFailsClosed, TestInternalAdminCoversInternalUser, TestCustomRoleGrant, TestSeedCustomRolesHaveNoAssignees). Verified 2026-11-12.
- README script: asserted by the integration tests above; steps 1 and the UI walk are not automated (gap).
- Active task: companies/projects/project roles implemented (migration 00005); docs aligned; authz-reviewer ran once (Revise: live tier, now done; re-review pending).

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
