# Plan: task-management

Input: approved [requirements.md](requirements.md), [system-design.md](system-design.md), [db-design.md](db-design.md).

Base branch: `main`. Every task is additive and merges into `main`; no integration branch is needed (no breaking change: only new tables, new routes, a superset model and one widened CHECK). Every task keeps `make check` green and the system running (see `.claude/skills/plan-tasks`).

## Strategy
- **Delivery pattern:** additive + dark launch + expand-only data. Everything new is unreachable until a tenant's `tasks` product is switched on: `tasks_enabled` is a `user:*` tuple that exists for no tenant, so every new Check is false (writes 403, reads 404 or `[]`) and the web tab is hidden. Routes may be registered from the first endpoint task; they change nothing for existing flows.
- **Order inside the API:** migration (T1) and model (T2) first, then the product toggle (T3), then task endpoints, then comment endpoints, then the cross-cutting seed (T15a) and authorization matrix (T15b). The web follows the API endpoint it calls.
- **Cutover (F):** none as code. "Live" = a platform admin switches `tasks` on for a tenant (`PUT /tenants/:id/products/tasks`), per tenant, reversible by switching it off (the tuple is deleted; data stays). Seeded demo: Acme on, Globex off.
- **Rollback:** revert the PR (all tasks are additive). Data: the migration's Down drops both tables and the product rows (demo data, stated in db-design). A tenant can be switched off without a revert.
- **Soak gate / contract:** none, nothing is dropped or renamed (no C task).
- **Baselines, recorded in T0 before T1** and stored in `docs/harness/progress.md`: `make check` and `make verify-live` green on `main`; `go test ./... -count=1` test count; `scripts/smoke-replay.sh` and `scripts/smoke-replay-domains.sh` outputs (existing flows must stay identical after every API task).
- **Environments:** only local exists (compose: Postgres, OpenFGA; the API applies migrations on start). There is no CI and no dev/staging/prod in this repo (`docs/operations.md`), so each infra row is "local".
- **Authorization review:** `authz-reviewer` runs in T2, T3, T4, T10, T14 and T15b (every task touching `service/`, `fga/`, `model.fga` or `permissions.tsx`; T5-T9 and T11-T13 reuse the same Check helpers and are covered by T15b).

## Endpoint inventory
| Deployable | Endpoint / page | New or changed | Slice task |
| --- | --- | --- | --- |
| api | migration `00012_tasks.sql` | new | T1 |
| api | OpenFGA model (`tasks_enabled`, `can_*_tasks`, `task_comment`) | changed (superset) | T2 |
| api | `PUT /tenants/:id/products/tasks`, tenant product list | changed | T3 |
| api | `POST /projects/:id/tasks` | new | T4 |
| api | `GET /projects/:id/tasks` | new | T5 |
| api | `GET /tasks/:id` | new | T6 |
| api | `PATCH /tasks/:id` | new | T7 |
| api | `GET /projects/:id/task-progress` | new | T8 |
| api | `GET /projects/:id/task-assignees` | new | T9 |
| api | `POST /tasks/:id/comments` | new | T10 |
| api | `GET /tasks/:id/comments` | new | T11 |
| api | `PATCH /task-comments/:id` | new | T12 |
| api | `DELETE /task-comments/:id` | new | T13 |
| api | `DELETE /tasks/:id` | new | T14 |
| api | seed data | changed | T15a |
| api | authorization matrix test | new | T15b |
| web | product toggle row, permissions entries, API client | changed | T16 |
| web | Tasks tab: table, progress bar, filters, read-only drawer with comments | new | T17 |
| web | create task with assignee | new | T18a |
| web | edit, status change, delete task | new | T18b |
| web | comment add / edit / delete in the drawer | new | T19 |
| web | board view | new | T20 |
| infra | baselines | docs | T0 |
| infra | README click-script step, smoke script | docs | T21 |

## Infrastructure / 3rd-party changes
| Item | Environment | create / update / delete | Notes (safe default so early merge is harmless) | Task |
| --- | --- | --- | --- | --- |
| Postgres migration 00012 | local | create (applied by the API on start) | tables unused until T4; `pg_dump -Fc` first outside the demo, `make seed` rebuilds the demo | T1 |
| OpenFGA model version | local | update | superset; the old API ignores new types. `make seed` writes it; the API opens the newest model, so restart it after reseeding | T2 |
| `tasks_enabled` tuple per tenant | local | create (by toggle or seed) | absent = feature off | T3, T15a |
| Compose, env vars, Engine, Agent, 3rd parties | none | none | no config added | - |

## Tasks
Types: M migration, I infra, E endpoint slice, D docs. One PR per task, one deployable per PR. Estimates include tests and the doc edits named in the task (behavior docs ship with the behavior).

| ID | Type | Deployable | Endpoint / slice | Scope (layers / files) | Est (h) | Depends on | Merge safety (state of `main` after merge) | E2E test | Rollback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| T0 (done) | D | infra | baselines on untouched `main` | run `make check`, `make verify-live`, `go test ./... -count=1` (record the test count), `scripts/smoke-replay.sh` and `scripts/smoke-replay-domains.sh`; store results in `docs/harness/progress.md` | 2 | - | Docs only, no runtime effect | the recorded outputs exist and are green | revert PR |
| T1 (done) | M | api | migration `00012_tasks.sql` | Up/Down per db-design (`SET LOCAL lock_timeout`, tables, indexes, widened CHECK flagged, `tenant_products` backfill); `postgres/db.go` `Truncate`; `docs/database-schema.md` (ER block, both stale CHECK lists, tables); `scripts/check-migrations.sh`; the one-row `tenant_products` backfill stays in this migration (a bulk INSERT of one row per tenant, precedent 00006, not a separate task) | 3 | T0 | Additive; nothing reads the tables; old code passes the widened CHECK | dry run Up and Down in a rolled-back transaction on the live schema and a seeded copy; the previous build (`main` before T1) started on the migrated DB: `make verify-live`, smoke-replay, and its `GET /tenants/:id/products` and web toggle row still work with the extra `tasks` row; EXPLAIN output for Q1-Q3 pasted into the T5/T8/T10 PR descriptions | run the Down migration first (loses demo tasks), then revert the PR |
| T2 (done) | I | api | OpenFGA model | `api/fga/model.fga` + DSL block in `docs/authorization-model.md` (equal), `docs/tuple-catalog.md`, `docs/decision-catalog.md` rows, `fga/model_dsl_test.go`; integration test of the relation truth table (internal admin/user, external admin/user, member or not, custom role, product off) on a scratch store | 4 | - | Superset model; no code uses it; `make seed` still works | `make verify-live` after reseed + new model test; `authz-reviewer`; `authz-reviewer` (model) | revert PR, then reseed (`make seed`) so the store has the previous model |
| T3 (done) | E | api | `PUT /tenants/:id/products/tasks`, product list | `model.ProductTasks`, `product_relations.go` wildcard, `ProductRepo.Set` upsert (`INSERT ... ON CONFLICT`), toggling a missing row creates it (a missing row stays absent from the list until then; T1 backfilled every existing tenant), tenant repo inserts four rows, `docs/http-api.md` (`:product` list, whitelisted relations), inspector whitelist, `model/authz.go` relation names (`can_view_tasks`, `can_edit_tasks`, `can_manage_tasks`, `tasks_enabled`), `/me/permissions` whitelist in `permission_handler` so rows and the web get them | 5 | T1, T2 | Additive; existing three products unchanged; toggling creates the feature switch only | HTTP: platform admin enables/disables `tasks` for a tenant, tenant admin 403, a tenant lacking the row (made by old build) toggles; real-Postgres test for the upsert; `/me/permissions` returns the new relations (test); `authz-reviewer` | revert PR |
| T4 | E | api | `POST /projects/:id/tasks` | `model/task.go`, `TaskGateway`, `TaskCreator` usecase + input/output, service (project load under tenant, `can_edit_tasks` Check, assignee `can_access` Check, validation), `task_repo.go` insert, handler + `mapper/task_mapper.go`, route, wiring, `docs/http-api.md`, `docs/decision-catalog.md`, `api/ARCHITECTURE.md`, `ARCHITECTURE.md` and `CLAUDE.md` domain lists (task) | 6 | T1, T2, T3 | New route; product off for everyone, so it 403s; no existing path touched | HTTP with product on: title-only 201, all fields, 400s (empty title, weight 0, end before start, unknown status), `not_a_member`, contributor vs external user 403, other tenant 404; `authz-reviewer` (core Checks) | revert PR |
| T5 | E | api | `GET /projects/:id/tasks` | `TaskLister` (Q1, Q2), repo list + comment count query, one BatchCheck for row `permissions`, `[]` when denied, mapper, route, docs | 4 | T4 | New route, additive | HTTP: member lists, external member lists, non-member `[]`, other tenant `[]`; EXPLAIN Q1/Q2 on seeded data | revert PR |
| T6 | E | api | `GET /tasks/:id` | `TaskGetter`, repo Q4, 404 on denial or other tenant, route, docs | 2 | T4 | New route | HTTP: 200, denied 404, other tenant 404 | revert PR |
| T7 | E | api | `PATCH /tasks/:id` | `TaskUpdater`: partial body (absent vs `null`), re-Check assignee only when changed, validation, `updated_at`, repo update, mapper, route, docs | 4 | T4 | New route | HTTP: single-field edit, clear a field with `null`, change assignee (400 for a non-member), old assignee who lost access does not block an unrelated edit, external user 403 | revert PR |
| T8 | E | api | `GET /projects/:id/task-progress` | `TaskProgressGetter` (Q3 aggregate), zero progress when denied or no tasks, mapper, route, docs | 2 | T4 | New route | HTTP: weights 2+1+1 with one completed = 25 %, counts per status, empty project 0 | revert PR |
| T9 | E | api | `GET /projects/:id/task-assignees` | `TaskAssigneeLister`: `can_edit_tasks` Check, tenant member rows (Q12), `can_access` BatchChecks in batches, name only, `[]` when denied, route, docs | 4 | T4 | New route | HTTP: contributor sees members, external user `[]`, someone without project access is absent | revert PR |
| T10 | E | api | `POST /tasks/:id/comments` | model `task_comment.go`, `TaskCommentGateway`, `task_comment_relations.go` (project + owner tuples), `TaskCommentCreator` (task row `FOR UPDATE`, `can_view_tasks` Check, cap 50, PG insert then FGA write, compensate), repo, handler + mapper, route, wiring, docs, `ARCHITECTURE.md` and `CLAUDE.md` domain lists (task_comment) | 6 | T4 | New route; creates tuples only for new comments | HTTP: member (incl. external) comments 201, 51st 409 `comment_limit`, concurrent adds never exceed 50 (real Postgres test), FGA failure rolls the row back, non-member 403/404; `authz-reviewer` | revert PR |
| T11 | E | api | `GET /tasks/:id/comments` | `TaskCommentLister` (Q7), BatchCheck row `permissions`, `[]` when denied, mapper, route, docs | 3 | T10 | New route | HTTP: oldest first, author name, `can_edit`/`can_delete` flags per viewer | revert PR |
| T12 | E | api | `PATCH /task-comments/:id` | `TaskCommentUpdater`: comment loaded under tenant (Q9), `can_edit` Check on `task_comment`, `edited_at`, route, docs | 3 | T10 | New route | HTTP: author 200, other member 403, manager 403 (edit is author only), other tenant 404 | revert PR |
| T13 | E | api | `DELETE /task-comments/:id` | `TaskCommentDeleter`: `can_delete` Check, `WriteGuard.Revoke` (FGA delete, then PG, restore on failure), route, docs | 3 | T10 | New route | HTTP: author 204, internal admin 204, other member 403, tuples gone (Read), PG failure restores tuples | revert PR |
| T14 | E | api | `DELETE /tasks/:id` | `TaskDeleter`: task row `FOR UPDATE`, `can_edit_tasks` Check, comment ids (Q10), FGA delete of their tuples, PG delete (cascade), restore on failure, route, docs | 4 | T10 | New route | HTTP: deletes task and comments and tuples, 403 for external user, add-comment racing a delete ends 404, restore path test; `authz-reviewer` | revert PR |
| T15a | I | api | seed data | `internal/seed/world.go` (tasks, comments, `tasks_enabled` for Acme; Globex off; ids only here), `cmd/seed` tuples, `docs/seed-data.md` | 2 | T3, T10 | Seed is run only by `make seed` | `make seed`, API restarted, list/get through HTTP show the demo rows; smoke-replay unchanged | revert PR, reseed |
| T15b | E | api | authorization matrix test | `TestTaskAccess` integration matrix (AC9-11: internal admin/user, external admin/user, project member or not, custom role with and without contribute, product off, platform admin, other tenant, over every task and comment endpoint), `docs/testing.md` | 4 | T3-T14, T15a | A test only | `make verify-live`; `authz-reviewer` over the whole diff vs `main` | revert PR |
| T16 | E | web | foundation | `TenantDetail.tsx` product row, `permissions.tsx` (`nav.tasks`/project tab rule needing membership and `tasks_enabled`, `ACTIONS` for create/edit/delete/comment), `api.ts` types and calls, `Inspector.tsx` relations | 3 | T3 | Tab not yet rendered; only a toggle row | `npm run build`; click: toggle `tasks` for Acme shows in the products list | revert PR |
| T17 | E | web | Tasks tab (read) | `components/ProjectTasks.tsx` table, progress bar + counts, filters (status, assignee, mine), overdue date style; `components/TaskDrawer.tsx` read-only with comment list; `ProjectDetail.tsx` tab `?tab=tasks`; `docs/web-ui.md`, `web/ARCHITECTURE.md` | 6 | T5, T6, T8, T11, T15a, T16 | Tab hidden until the product is on | seeded Acme as ada, bob, dan (external): see tasks and comments; Globex has no tab; `npm run build` | revert PR |
| T18a | E | web | create task and assign | New task form, assignee picker (T9), `run()` toasts, create button hidden by `ACTIONS` | 4 | T4, T9, T17 | Extends the same tab | contributor creates a task with only a title and with an assignee; external user sees no button and the API still denies | revert PR |
| T18b | E | web | edit, status, delete | inline edit in the drawer, status change, delete with `confirmDanger`, buttons hidden by `ACTIONS` | 4 | T7, T14, T18a | Extends the same tab | edit fields, clear a date, change status, delete; progress bar updates | revert PR |
| T19 | E | web | comments | add, edit own, delete (own or manager) in the drawer; 409 `comment_limit` message | 4 | T10, T11, T12, T13, T17 | Extends the drawer | two users discuss a task; non-author has no edit button | revert PR |
| T20 | E | web | board view | Table/Board switch, four status columns, change status from the card menu | 4 | T7, T17 | Extends the tab | move a card between columns, progress bar updates | revert PR |
| T21 | D | infra | docs and smoke | README click-script step, `scripts/smoke-tasks.sh` (HTTP walk of AC1-8), `progress.md` | 2 | T15b, T20 | No runtime effect | run `scripts/smoke-tasks.sh` on a fresh seed | revert PR |

Total estimate: 88 h (rows sum to 88). Critical path: T0 -> T1 -> T4 -> T10 -> T15a -> T17 -> T18a -> T18b -> T20 -> T21. Parallel: T1 with T2; after T4 the API tasks T5-T9 are independent of each other and of T10-T13; web T16 can start after T3; T15b runs in parallel with the web tasks.

## DB design coverage
| db-design item | Task | Proof | Rollout (local) |
| --- | --- | --- | --- |
| `tasks` table (columns, CHECKs, composite FK, `assignee_id` SET NULL) | T1 | dry run; T4 integration (constraint errors map to 400); `check-migrations.sh` | `pg_dump -Fc` (demo: `make seed` rebuilds), apply on API start, verify `\d tasks`, rollback = Down |
| `tasks_tenant_project_idx` | T1 | EXPLAIN Q1/Q3 in T5/T8 show the index scan | same |
| `task_comments` table, `task_comments_task_created_idx` | T1 | dry run; EXPLAIN Q2/Q7/Q8/Q10 in T10-T11 | same |
| `tenant_products` CHECK widened + `tasks` backfill + `SET LOCAL lock_timeout` | T1 | dry run; old build runs on the migrated DB | same |
| `force-allow-breaking` exception (DROP CONSTRAINT) | T1 | `check-migrations.sh` accepts with reason | same |
| Down migration | T1 | Down in a rolled-back transaction, tables gone and CHECK restored | rollback = Down |
| `ProductRepo.Set` upsert, missing row shown as disabled, tenant repo four rows | T3 | real-Postgres test with a tenant that has no `tasks` row | old build coexists: tenants it creates lack the row |
| `Truncate` lists the new tables | T1 | `make seed` succeeds | local reseed |
| `docs/database-schema.md` (ER, both CHECK lists, tables) | T1 | doc review (spec first) | - |
| `docs/seed-data.md`, `world.go` rows and comment tuples | T15a | `make seed` + `make verify-live` | local reseed |
| Access paths Q1, Q2, Q3 | T5, T8 | EXPLAIN on seeded data | - |
| Q4, Q5, Q6 | T4, T6, T7, T10, T14 | integration tests; Q5 lock test in T10 and T14 | - |
| Q7, Q8, Q9, Q10, Q11 | T10-T14 | integration tests, EXPLAIN | - |
| Q12 | T9 | HTTP test | - |
| Checklist: old-build smoke (incl. old product list with the extra row), restore point, dry run | T1 | recorded in the PR | per above |
| Checklist: concurrency (cap and delete serialize on the task lock) | T10, T14 | real-Postgres concurrent test | - |

How old and new builds coexist: the migration is expand-only; the old build ignores the tables and writes only the three old product values; the new build treats a missing `tasks` row as disabled and upserts it on toggle (T3).

## Merge sequence walk
After T1: new empty tables and a widened CHECK; `make check` green; old behavior unchanged. After T2: model superset; reseed works; nothing uses it. After T3: toggle for `tasks` works; no feature behind it. After T4: tasks can be created for a tenant that has the product on (none by default). After T5-T9 (any order): reads and progress; each needs only T4. After T10-T13: comments; T11-T13 need T10 only. After T14: task delete removes comments and tuples (it needs only T10). After T15a: demo data. After T15b: the matrix test; `make verify-live` green. After T16-T20 (T16, T17, T18a, T18b, T19, T20): web grows behind the hidden tab; each depends only on merged API tasks. After T21: docs and smoke. Any dependency-respecting order keeps `make check` green and, with the product off, every pre-existing behavior identical (smoke-replay diff is part of each API task's proof).

## Release order
1. Merge T1; deploy; migration applies on API start (take a restore point outside the demo).
2. Merge T2; write the new model (`make seed` for the demo; any non-demo store needs a new model version written before the new API starts, since the API opens the newest).
3. Merge T3-T15b, T16-T21; restart the API after the reseed.
4. Go live per tenant: platform admin switches `tasks` on; verify with `scripts/smoke-tasks.sh`. Roll back per tenant by switching it off.

## Test plan
| Step | Acceptance criteria | Gates which task |
| --- | --- | --- |
| Create with title only, all fields, validation 400s | AC1, AC2 | T4, T7 |
| Assignee must be a member; clear it | AC3 | T4, T7, T9 |
| Members see tasks and comments; non-members `[]`/404 | AC4 | T5, T6, T11, T15b |
| External user cannot write tasks but can comment | AC5 | T4, T10, T15b |
| Comment edit by author only, delete by author or manager | AC6 | T12, T13 |
| Deleting a task deletes its comments (rows and tuples) | AC7 | T14 |
| Weighted progress and counts | AC8 | T8 |
| Product off denies everything; toggle platform only | AC9 | T3, T15b, T16 |
| Tenant isolation | AC10 | T15b |
| Authorization matrix incl. custom role and platform admin | AC11 | T15b |
| UI per proposal; README step; `make check`, `make verify-live` | AC12 | T17-T21 |

## Risks
- **Model rollout on a live store:** the API opens the newest model; a store without the new version makes every new Check fail (403). Demo reseeds; document the write-new-model step (T2).
- **Row lock held across an OpenFGA call** (comment create, task delete): latency under contention; acceptable at demo scale, bounded by the 50-comment cap.
- **T17 (6 h)** is near the review limit; split table vs drawer if the diff grows. T18 is already split.
- **Old build creating tenants** during the rollout leaves no `tasks` row until T3 handles it.
- Estimates are not measured against history; treat 88 h as order of magnitude.
