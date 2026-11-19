---
name: build-task
description: Stage 5 of the feature SDLC. Implement exactly one task from an approved specs/<feature>/plan.md with its tests and verification. Use when asked to build or implement a planned task.
---

# Build one task

1. Read `plan.md` and pick one task whose dependencies are done. One task = one service = one PR. Do not pull in other tasks.
2. Clock-in: `docs/harness/progress.md`, `make check` baseline.
3. Short implementation plan first (files, order, tests, risks); if it differs from `plan.md`, update `plan.md` and tell the user.
4. Implement with the matching skill: `go-api` for `api/`, `db-design` for migrations, and CLAUDE.md for web (`permissions.tsx`, Mantine, `ui.tsx`) and the engine. Write unit tests, and integration tests where the plan says so. Bug fixes: failing test first, then the fix without editing that test.
5. Verify with evidence: `make check`; `make verify-live` for authorization changes (it RESETS data; restart the API after reseeding).
6. Run `authz-reviewer` when `service/`, `fga/`, `model.fga`, or `permissions.tsx` changed; fix findings below 2.
7. Update docs/architecture files touched, mark the task done in `plan.md`, update `docs/harness/progress.md`. Do not commit unless asked; keep the diff reviewable in 15-30 minutes.
