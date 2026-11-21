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
Status: `todo`, `in progress`, `built` (code finished and reviewed, not merged or not fully verified), `done` (merged and verified). Types: M migration, I infra, E endpoint slice, D docs. One PR per task, one deployable per PR. Estimates include tests and the doc edits named in the task (behavior docs ship with the behavior).

| ID | Status | Type | Deployable | Endpoint / slice | Description | Est (h) | Depends on | Merge safety (state of `main` after merge) | E2E test | Rollback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| T0 | done | D | infra | baselines on untouched `main` | Record baselines on untouched `main` so that "existing behavior unchanged" can be proved later. | 2 | - | Docs only, no runtime effect | the recorded outputs exist and are green | revert PR |
| T1 | done | M | api | migration `00012_tasks.sql` | Add the storage for tasks and comments as one expand-only migration. | 3 | T0 | Additive; nothing reads the tables; old code passes the widened CHECK | dry run Up and Down in a rolled-back transaction on the live schema and a seeded copy; the previous build (`main` before T1) started on the migrated DB: `make verify-live`, smoke-replay, and its `GET /tenants/:id/products` and web toggle row still work with the extra `tasks` row; EXPLAIN output for Q1-Q3 pasted into the T5/T8/T10 PR descriptions | run the Down migration first (loses demo tasks), then revert the PR |
| T2 | done | I | api | OpenFGA model | Add the OpenFGA model that decides who can see, edit and manage tasks and comments. | 4 | - | Superset model; no code uses it; `make seed` still works | `make verify-live` after reseed + new model test; `authz-reviewer`; `authz-reviewer` (model) | revert PR, then reseed (`make seed`) so the store has the previous model |
| T3 | done | E | api | `PUT /tenants/:id/products/tasks`, product list | Let a platform admin switch the `tasks` product on or off for a tenant. | 5 | T1, T2 | Additive; existing three products unchanged; toggling creates the feature switch only | HTTP: platform admin enables/disables `tasks` for a tenant, tenant admin 403, a tenant lacking the row (made by old build) toggles; real-Postgres test for the upsert; `/me/permissions` returns the new relations (test); `authz-reviewer` | revert PR |
| T4 | done | E | api | `POST /projects/:id/tasks` | Let a project contributor create a task. | 6 | T1, T2, T3 | New route; product off for everyone, so it 403s; no existing path touched | HTTP with product on: title-only 201, all fields, 400s (empty title, weight 0, end before start, unknown status), `not_a_member`, contributor vs external user 403, other tenant 404; `authz-reviewer` (core Checks) | revert PR |
| T5 | todo | E | api | `GET /projects/:id/tasks` | Let a project member list the tasks of a project. | 4 | T4 | New route, additive | HTTP: member lists, external member lists, non-member `[]`, other tenant `[]`; EXPLAIN Q1/Q2 on seeded data | revert PR |
| T6 | todo | E | api | `GET /tasks/:id` | Let a project member read one task. | 2 | T4 | New route | HTTP: 200, denied 404, other tenant 404 | revert PR |
| T7 | todo | E | api | `PATCH /tasks/:id` | Let a contributor edit a task with a partial body. | 4 | T4 | New route | HTTP: single-field edit, clear a field with `null`, change assignee (400 for a non-member), old assignee who lost access does not block an unrelated edit, external user 403 | revert PR |
| T8 | todo | E | api | `GET /projects/:id/task-progress` | Return the weighted progress of a project and the number of tasks per status. | 2 | T4 | New route | HTTP: weights 2+1+1 with one completed = 25 %, counts per status, empty project 0 | revert PR |
| T9 | todo | E | api | `GET /projects/:id/task-assignees` | List the people a task can be assigned to. | 4 | T4 | New route | HTTP: contributor sees members, external user `[]`, someone without project access is absent | revert PR |
| T10 | todo | E | api | `POST /tasks/:id/comments` | Let a project member comment on a task. | 6 | T4 | New route; creates tuples only for new comments | HTTP: member (incl. external) comments 201, 51st 409 `comment_limit`, concurrent adds never exceed 50 (real Postgres test), FGA failure rolls the row back, non-member 403/404; `authz-reviewer` | revert PR |
| T11 | todo | E | api | `GET /tasks/:id/comments` | Let a project member read the comments of a task. | 3 | T10 | New route | HTTP: oldest first, author name, `can_edit`/`can_delete` flags per viewer | revert PR |
| T12 | todo | E | api | `PATCH /task-comments/:id` | Let the author edit their own comment. | 3 | T10 | New route | HTTP: author 200, other member 403, manager 403 (edit is author only), other tenant 404 | revert PR |
| T13 | todo | E | api | `DELETE /task-comments/:id` | Let the author or a task manager delete a comment. | 3 | T10 | New route | HTTP: author 204, internal admin 204, other member 403, tuples gone (Read), PG failure restores tuples | revert PR |
| T14 | todo | E | api | `DELETE /tasks/:id` | Let a contributor delete a task together with its comments. | 4 | T10 | New route | HTTP: deletes task and comments and tuples, 403 for external user, add-comment racing a delete ends 404, restore path test; `authz-reviewer` | revert PR |
| T15a | todo | I | api | seed data | Add demo tasks and comments to the seed. | 2 | T3, T10 | Seed is run only by `make seed` | `make seed`, API restarted, list/get through HTTP show the demo rows; smoke-replay unchanged | revert PR, reseed |
| T15b | todo | E | api | authorization matrix test | Prove with one integration test who may do what with tasks and comments. | 4 | T3-T14, T15a | A test only | `make verify-live`; `authz-reviewer` over the whole diff vs `main` | revert PR |
| T16 | built | E | web | foundation | Make the web aware of the `tasks` product and its permissions. | 3 | T3 | Tab not yet rendered; only a toggle row | `npm run build`; click: toggle `tasks` for Acme shows in the products list | revert PR |
| T17 | todo | E | web | Tasks tab (read) | Show a read-only Tasks tab on the project page. | 6 | T5, T6, T8, T11, T15a, T16 | Tab hidden until the product is on | seeded Acme as ada, bob, dan (external): see tasks and comments; Globex has no tab; `npm run build` | revert PR |
| T18a | todo | E | web | create task and assign | Let a contributor create a task from the Tasks tab. | 4 | T4, T9, T17 | Extends the same tab | contributor creates a task with only a title and with an assignee; external user sees no button and the API still denies | revert PR |
| T18b | todo | E | web | edit, status, delete | Let a contributor edit a task, change its status and delete it from the drawer. | 4 | T7, T14, T18a | Extends the same tab | edit fields, clear a date, change status, delete; progress bar updates | revert PR |
| T19 | todo | E | web | comments | Let members discuss a task in the drawer. | 4 | T10, T11, T12, T13, T17 | Extends the drawer | two users discuss a task; non-author has no edit button | revert PR |
| T20 | todo | E | web | board view | Add a Board view of the tasks next to the table. | 4 | T7, T17 | Extends the tab | move a card between columns, progress bar updates | revert PR |
| T21 | todo | D | infra | docs and smoke | Add the demo step to the README and a smoke script for tasks. | 2 | T15b, T20 | No runtime effect | run `scripts/smoke-tasks.sh` on a fresh seed | revert PR |

T16 is `built`: merged, but the browser walk in its contract is still pending.

Total estimate: 88 h (rows sum to 88). Critical path: T0 -> T1 -> T4 -> T10 -> T15a -> T17 -> T18a -> T18b -> T20 -> T21. Parallel: T1 with T2; after T4 the API tasks T5-T9 are independent of each other and of T10-T13; web T16 can start after T3; T15b runs in parallel with the web tasks.

## Task descriptions
One ticket per task. Every criterion comes from `requirements.md` (`AC<n>`) or the design documents; the table's Description is the Summary.

### T0: Baselines
**Summary:** Record baselines on untouched `main` so that "existing behavior unchanged" can be proved later.

**Context:** Every later task must leave `main` green and existing flows unchanged. That is only measurable with numbers taken before the first change.

**Requirements & Acceptance Criteria:**
- `make check` and `make verify-live` are green on `main` and the outputs are recorded.
- The Go test count (`go test ./... -count=1`) is recorded.
- The outputs of `scripts/smoke-replay.sh` and `scripts/smoke-replay-domains.sh` are recorded.
- Everything is written to `docs/harness/progress.md`.

**Out of scope:** Any code change.

### T1: Migration for tasks and comments
**Summary:** Add the storage for tasks and comments as one expand-only migration.

**Context:** T4 and later tasks need the tables. The migration ships first and nothing reads it, so the old build keeps working against the new schema.

**Requirements & Acceptance Criteria:**
- `tasks` and `task_comments` exist with the columns, constraints and two indexes defined in `db-design.md`.
- `tenant_products` accepts the value `tasks`, and every existing tenant gets a `tasks` product row.
- The Down migration restores the previous schema; Up and Down pass a dry run in a rolled-back transaction.
- `Truncate` and `docs/database-schema.md` include the new tables; `scripts/check-migrations.sh` passes.
- The previous build starts and passes the smoke checks on the migrated database.

**Out of scope:** Any code that reads or writes the new tables.

### T2: OpenFGA model for tasks
**Summary:** Add the OpenFGA model that decides who can see, edit and manage tasks and comments.

**Context:** Every task action is decided by OpenFGA, so the model has to exist before any code Checks it. Nothing uses it yet.

**Requirements & Acceptance Criteria:**
- The model has the `tasks_enabled` product on the tenant, `can_view_tasks`, `can_edit_tasks` and `can_manage_tasks` on the project, and a `task_comment` type with `project`, `owner`, `can_edit` and `can_delete` (`system-design.md` "Authorization").
- `api/fga/model.fga` equals the DSL block in `docs/authorization-model.md`; `tuple-catalog.md` and `decision-catalog.md` are updated.
- A truth-table integration test on a scratch store covers internal admin and user, external admin and user, project member or not, a custom role and product off (AC11).
- The model stays allow-only.

**Out of scope:** Code that writes or checks the new relations (T3 onward).

### T3: Toggle the tasks product
**Summary:** Let a platform admin switch the `tasks` product on or off for a tenant.

**Context:** Tasks are a platform-toggled product like documents, photos and blueprints. T1 backfilled existing tenants, but a tenant created by the old build has no `tasks` row.

**Requirements & Acceptance Criteria:**
- `PUT /tenants/:id/products/tasks` works like the other products: platform admin only, tenant admins see the status (AC9).
- Toggling a tenant that has no `tasks` row creates it.
- The product list includes `tasks`; a missing row reads as disabled; a new tenant gets four product rows.
- `/me/permissions` and the inspector include `can_view_tasks`, `can_edit_tasks`, `can_manage_tasks` and `tasks_enabled`.
- `http-api` and `decision-catalog` are updated.

**Out of scope:** Any task or comment endpoint.

### T4: Create a task
**Summary:** Let a project contributor create a task.

**Context:** First task endpoint; the rest build on it. The product is off for every tenant, so it is unreachable until a platform admin enables it.

**Requirements & Acceptance Criteria:**
- `POST /projects/:id/tasks` with only a title returns 201 with status `not_started` and weight 1 (AC1).
- All task fields can be set; invalid input is 400 `validation`: empty or over-long title, over-long description, unknown status, weight not above 0 or with more than 2 decimals, an end date before its start (AC2).
- An assignee must be a project member, otherwise 400 `not_a_member` (AC3).
- External users and other non-contributors get 403 (AC5); a project of another tenant gives 404 (AC10).
- Standard authorization conventions apply. `http-api`, `decision-catalog` and the domain lists in the architecture files are updated.

**Out of scope:** Listing, reading, editing and deleting tasks (T5-T7, T14).

### T5: List the tasks of a project
**Summary:** Let a project member list the tasks of a project.

**Context:** T4 creates tasks. The Tasks tab (T17) needs the list, each task's comment count and what the viewer may do with each row.

**Requirements & Acceptance Criteria:**
- `GET /projects/:id/tasks` returns every task of the project, oldest first, with title, status, assignee name, planned dates, weight, comment count and its permissions.
- Anyone who can view the project's tasks sees them, external project members included (AC4); a task of another tenant never appears (AC10). Standard authorization conventions apply.
- Comment counts come from one query, not one per task; the access path is checked with EXPLAIN on seeded data.
- No pagination (a stated non-goal).

**Out of scope:** Progress (T8), reading one task (T6), comments (T11).

### T6: Read one task
**Summary:** Let a project member read one task.

**Context:** The drawer in the Tasks tab opens one task by id.

**Requirements & Acceptance Criteria:**
- `GET /tasks/:id` returns the task with assignee name, comment count and permissions.
- A task the caller cannot view, or one in another tenant, gives 404 (AC4, AC10). Standard authorization conventions apply.

**Out of scope:** Comments of the task (T11).

### T7: Edit a task
**Summary:** Let a contributor edit a task with a partial body.

**Context:** Edits and status changes in the UI (T18b, T20) send only the fields that changed.

**Requirements & Acceptance Criteria:**
- `PATCH /tasks/:id`: a missing field is unchanged and `null` clears an optional field (description, assignee, the four dates); title, status and weight cannot be `null` (AC2).
- The create validation applies; an end date is compared with the start the task will have after the edit (AC2).
- Changing the assignee requires a project member (400 `not_a_member`); an unrelated edit does not fail because the current assignee lost access (AC3).
- External users and other non-contributors get 403 (AC5); `updated_at` changes.

**Out of scope:** Creating (T4) and deleting (T14) a task.

### T8: Project progress
**Summary:** Return the weighted progress of a project and the number of tasks per status.

**Context:** The Tasks tab shows a progress bar with counts (T17). The figure is computed on the server so every client agrees.

**Requirements & Acceptance Criteria:**
- `GET /projects/:id/task-progress` returns `total`, `weight_total`, `weight_done`, `percent` and `counts` for all four statuses (AC8).
- `percent` is the weight of `completed` tasks over the weight of all tasks, and 0 when there are no tasks; weights 2, 1 and 1 with the weight-1 task completed give 25 (AC8).
- It reflects every change immediately and is computed in one grouped query.
- Standard authorization conventions apply.

**Out of scope:** The progress bar (T17).

### T9: Assignee candidates
**Summary:** List the people a task can be assigned to.

**Context:** The create and edit forms (T18a) need a picker, and an assignee has to be a project member (AC3).

**Requirements & Acceptance Criteria:**
- `GET /projects/:id/task-assignees` returns `[{user_id, name}]` for tenant members who can access the project; no email, no role.
- Only people who can edit tasks get the list; it is a convenience, every write still re-checks.
- Someone without access to the project is absent; people of other tenants never appear (AC3, AC10).

**Out of scope:** The picker in the UI (T18a).

### T10: Add a comment
**Summary:** Let a project member comment on a task.

**Context:** Comments are discussed next to the work. Each comment has its own OpenFGA object so that "edit your own" is an authorization decision, not a branch in the service.

**Requirements & Acceptance Criteria:**
- `POST /tasks/:id/comments` with `{body}` of 1-4000 characters returns 201 (AC6).
- Anyone who can view the task may comment, external users included (AC5, AC6).
- The 51st comment on a task is 409 `comment_limit`, and concurrent adds never exceed 50 (a real-Postgres test).
- The comment is stored with its OpenFGA tuples; if the tuple write fails, the comment is not stored.
- A task in another tenant gives 404 (AC10). Standard authorization conventions apply.

**Out of scope:** Listing, editing and deleting comments (T11-T13).

### T11: List comments
**Summary:** Let a project member read the comments of a task.

**Context:** The drawer shows the thread under the task (T17).

**Requirements & Acceptance Criteria:**
- `GET /tasks/:id/comments` returns the comments oldest first with id, author id and name, body, `created_at`, `edited_at` and the viewer's edit and delete permissions (AC6).
- Anyone who can view the task sees them (AC4); another tenant never does (AC10). Standard authorization conventions apply.

**Out of scope:** Writing comments (T10, T12, T13).

### T12: Edit a comment
**Summary:** Let the author edit their own comment.

**Context:** Editing is limited to the author; managers can only delete (AC6).

**Requirements & Acceptance Criteria:**
- `PATCH /task-comments/:id` with `{body}` (1-4000 characters) updates the text and sets `edited_at` (AC6).
- Only the author may edit; anyone else, a manager included, gets 403 with the `check` body (AC6).
- A comment in another tenant, or a missing one, gives 404 (AC10).

**Out of scope:** Deleting a comment (T13).

### T13: Delete a comment
**Summary:** Let the author or a task manager delete a comment.

**Context:** A comment has OpenFGA tuples, so a delete has to remove them and the row together.

**Requirements & Acceptance Criteria:**
- `DELETE /task-comments/:id` returns 204 for the author or a task manager; any other member gets 403 (AC6).
- The comment's tuples are removed with the row; if the database delete fails, the tuples that existed are restored.
- A comment in another tenant, or a missing one, gives 404 (AC10).

**Out of scope:** Editing a comment (T12).

### T14: Delete a task
**Summary:** Let a contributor delete a task together with its comments.

**Context:** A task's comments have tuples in OpenFGA, so deleting a task must clean those too (AC7).

**Requirements & Acceptance Criteria:**
- `DELETE /tasks/:id` returns 204 and removes the task, its comments and their tuples (AC7).
- External users and other non-contributors get 403 (AC5); another tenant gives 404 (AC10).
- A comment added while the delete runs never survives without its task: the add ends with 404.
- If the database delete fails, the tuples that existed are restored.

**Out of scope:** Deleting a single comment (T13).

### T15a: Demo data
**Summary:** Add demo tasks and comments to the seed.

**Context:** The UI tasks (T17-T20), the authorization test (T15b) and the demo script (T21) need data to work with.

**Requirements & Acceptance Criteria:**
- The seeded project of Acme has tasks in all four statuses, some assigned, with different weights, and comments from different people.
- `tasks_enabled` is on for Acme and off for Globex.
- `make seed` succeeds, ids live only in `internal/seed/world.go`, and `docs/seed-data.md` is updated.

**Out of scope:** Any endpoint change.

### T15b: Authorization matrix test
**Summary:** Prove with one integration test who may do what with tasks and comments.

**Context:** Authorization is the main risk of this feature; the matrix covers every actor over every endpoint.

**Requirements & Acceptance Criteria:**
- `TestTaskAccess` covers internal admin, internal user, external admin and external user (each as project member and not), a custom role with and without contribute, platform admin, product off, and a user of another tenant (AC11).
- It runs every task and comment endpoint and asserts AC4-AC6, AC9 and AC10.
- A failing case is fixed in the owning task's code in this PR, with a note; `make verify-live` and `authz-reviewer` pass.
- `docs/testing.md` is updated.

**Out of scope:** New product behavior.

### T16: Web foundation
**Summary:** Make the web aware of the `tasks` product and its permissions.

**Context:** The later web tasks only add screens; the permission mapping and API client are done once here.

**Requirements & Acceptance Criteria:**
- The tenant page shows a `tasks` product row with the same platform-admin toggle as the other products (AC9).
- `permissions.tsx` declares the Tasks tab (project membership and `tasks_enabled`) and the actions for create, edit and delete task and for comments; components never name relations.
- `api.ts` has the types and a call per endpoint T4-T14; the inspector lists the new project relations.
- `npm run build` passes.

**Out of scope:** The Tasks tab itself (T17).

### T17: Tasks tab, read only
**Summary:** Show a read-only Tasks tab on the project page.

**Context:** The first screen of the feature; writing comes in T18a-T19 and the board in T20.

**Requirements & Acceptance Criteria:**
- A `Tasks` tab (`?tab=tasks`) appears for project members only when the product is on (AC9); the Globex demo has no tab.
- A progress bar shows the percent and the count per status.
- A table shows title, status badge, assignee, planned dates, weight and comment count; a planned end in the past on a task that is not `completed` is a red date; filters for status, assignee and "assigned to me" work.
- Clicking a row opens a side drawer with every field and the comments, read only.
- Acme as ada, bob and dan (external) sees tasks and comments (AC4, AC12); `docs/web-ui.md` and `web/ARCHITECTURE.md` are updated.

**Out of scope:** Creating, editing and commenting (T18a-T19), the board (T20).

### T18a: Create a task in the UI
**Summary:** Let a contributor create a task from the Tasks tab.

**Context:** Builds on the read-only tab (T17) and the assignee list (T9).

**Requirements & Acceptance Criteria:**
- A "New task" form takes title (required), description, assignee, planned and actual dates, status and weight (AC1, AC2).
- The assignee picker lists only people who can be assigned and can be cleared (AC3).
- Validation and `not_a_member` errors are shown to the user; on success the list and progress reload.
- External users do not see the button, and the API still denies (AC5).

**Out of scope:** Editing and deleting (T18b).

### T18b: Edit and delete a task in the UI
**Summary:** Let a contributor edit a task, change its status and delete it from the drawer.

**Context:** Completes task writing in the UI after T18a.

**Requirements & Acceptance Criteria:**
- Fields are edited inline and only the changed fields are sent; clearing a date or the assignee works (AC2).
- The status can move to any other status (requirements: no workflow rules).
- Delete asks for confirmation and closes the drawer; the list and progress bar update after each change (AC7, AC8).
- Users who cannot edit do not see the buttons (AC5).

**Out of scope:** Comments (T19), the board (T20).

### T19: Comments in the UI
**Summary:** Let members discuss a task in the drawer.

**Context:** Comments live next to the work (requirements Outcome); this adds them to the drawer from T17.

**Requirements & Acceptance Criteria:**
- A member can add a comment of 1-4000 characters; external users too (AC5, AC6).
- The author sees Edit and the author or a manager sees Delete; others see neither (AC6).
- Reaching the 50-comment cap shows a clear message instead of a generic error.
- The thread reloads after each change.

**Out of scope:** Editing task fields (T18b).

### T20: Board view
**Summary:** Add a Board view of the tasks next to the table.

**Context:** The requirements propose a table and a board; the board is a second view of the same data.

**Requirements & Acceptance Criteria:**
- A Table/Board switch on the tab.
- The board has four columns, one per status, with cards showing title, assignee, weight and planned end.
- A card menu changes the status (AC2); the card moves and the progress bar updates (AC8).
- No drag and drop (out of scope in the requirements).

**Out of scope:** New fields or filters.

### T21: Demo script and smoke script
**Summary:** Add the demo step to the README and a smoke script for tasks.

**Context:** Closes the feature: a human click path and an automated HTTP walk of the main criteria.

**Requirements & Acceptance Criteria:**
- The README click script has a step: open the Acme project, use the Tasks tab, add a task and a comment, see progress change (AC12).
- `scripts/smoke-tasks.sh` walks AC1-AC8 over HTTP on a fresh seed and exits non-zero on any mismatch.
- `docs/harness/progress.md` is updated.

**Out of scope:** Runtime code.

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
