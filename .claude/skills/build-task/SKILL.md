---
name: build-task
description: Stage 5 of the feature SDLC. Implement exactly one task from an approved specs/<feature>/plan.md with its tests and verification. Use when asked to build or implement a planned task.
---

# Build one task

1. Read `plan.md` and pick one task whose dependencies are done. One task = one PR = one deployable (`api/`, `web/`, `agent/`, `blueprint-engine/`, or infra) x one endpoint; if the work needs code in a second deployable, stop and ask to split it into another planned task (see "Definitions" in the `plan-tasks` skill). Do not pull in other tasks.
2. Clock-in: `scripts/sync-base.sh <integration-branch>` first (pull the latest base before anything else), then `docs/harness/progress.md`, `make check` baseline.
3. Short implementation plan first (files, order, tests, risks); if it differs from `plan.md`, update `plan.md` and tell the user.
4. Implement with the matching skill, loaded before the first edit (see "Skill per service" in `docs/harness/sdlc.md`): `go-api` for `api/`, `python-agent` for `agent/`, `python-engine` for `blueprint-engine/`, `db-design` for migrations, and CLAUDE.md for web (`permissions.tsx`, Mantine, `ui.tsx`). A task that touches several services loads each service's skill (one task is still one service). Write unit tests, and integration tests where the plan says so. Bug fixes: failing test first, then the fix without editing that test.
5. Verify with evidence: the task's **e2e test** from `plan.md` through the real entry point (not only unit tests), and that its **merge safety** claim holds (new code unreachable or additive, existing behavior unchanged on the base branch; check the rollback works); then `make check`; `make verify-live` for authorization changes (it RESETS data; restart the API after reseeding).
6. Run `authz-reviewer` when `service/`, `fga/`, `model.fga`, or `permissions.tsx` changed; fix findings below 2.
7. Update docs/architecture files touched, mark the task done in `plan.md`, update `docs/harness/progress.md`. Do not commit unless asked (user is the only author, never on or to the protected branch, PR body per `.github/pull_request_template.md`, temp branches stay local and `scripts/clean-temp-branches.sh` runs after the PR is created); keep the diff reviewable in 15-30 minutes.
