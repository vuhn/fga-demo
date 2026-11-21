# Requirements: task-management

## Problem
Project teams (internal staff and the external companies working with them) have no place in the app to say what work a project consists of, who does it, when, and how far along it is. They also cannot discuss a piece of work next to the work itself.

## Outcome
Each project has a simple progress-management area: a list of tasks (work items) with owner, dates, status and weight, a weighted progress figure for the whole project, and a comment thread on every task. Who sees and changes what follows the existing project/tenant authorization; OpenFGA decides every allow/deny.

## Scope
- In:
  - **Task** (belongs to one project): `title` (required, the only mandatory field), `description`, `assignee` (one project member), `planned_start`, `planned_end`, `actual_start`, `actual_end` (dates), `status` (`not_started` default, `in_progress`, `completed`, `blocked`), `weight` (positive number, default 1).
  - **Task comment**: free text on a task, author and time shown; the author can edit or delete their own comment; a manager can delete any.
  - Project progress: weighted % complete (sum of weights of `completed` tasks / sum of all weights) and a count per status, computed server-side.
  - `tasks_enabled` product on the tenant, toggled by platform admins like the other products.
  - UI (proposal, to approve): a **Tasks** tab on the project page. Table view (title, status badge, assignee, planned dates, weight, comment count) with filters (status, assignee, "assigned to me") and a Board view (four status columns, change status from the card menu). A progress bar with status counts at the top. Clicking a task opens a side drawer with all fields (edit inline when allowed) and the comment thread below. Overdue (planned end passed, not completed) is shown as a red date. Mantine only, buttons hidden through `permissions.tsx` rules.
  - Seed data for the demo (a few tasks and comments in the seeded project of a tenant with the product on) and docs/README click-script step.
- Out (CLAUDE.md non-goals checked, none violated):
  - Subtasks, dependencies, Gantt, drag-and-drop ordering, multiple assignees, labels, attachments, @mentions, notifications, activity/audit log, comment threading/reactions, recurring tasks, time tracking, pagination, per-task sharing or private tasks, cross-project tasks, import/export.
  - Status workflow rules: any status can move to any other; dates are not forced to match status.

## Acceptance criteria
1. A project contributor can create a task with only a title: it is `not_started`, weight 1, other fields empty (201).
2. All fields in the Task list above can be set and changed by a contributor; invalid input is 400: empty or over-long title, unknown status, weight <= 0, a planned/actual end before its start.
3. An assignee must be a member of that project (can access it); otherwise 400 `not_a_member`. Clearing the assignee is allowed.
4. Any member who can view the project sees its tasks and comments, including external users who are project members. A person who cannot access the project gets 404 on get and an empty list.
5. An external user or other non-contributor cannot create, edit or delete a task: 403 with the `check` body. They can still comment.
6. Anyone who can view may add a comment; only its author can edit it; the author or a task manager can delete it (403 otherwise).
7. Deleting a task deletes its comments.
8. The project's progress equals sum(weight of completed) / sum(weight) over its tasks (0 when there are no tasks), with counts per status, and updates after each change.
9. With `tasks_enabled` off for a tenant, every task and comment call is denied, the Tasks tab is hidden, and the product toggle in the tenant admin works like the other three (platform only; tenant admins see the status).
10. Tenant isolation: tasks of tenant A never appear to members of tenant B (list, get, comment, assignee candidates).
11. Authorization integration tests cover internal admin, internal user, external admin, external user (project member and not), custom role with/without contribute, product off, and platform admin.
12. The UI covers the proposal above; README click script gains a step; `make check` and `make verify-live` pass.

## Impact on the current system
- Authorization: a new action family (list/get tasks and comments, create/edit/delete task, comment/edit/delete comment, manage product) needs new Checks in `docs/decision-catalog.md` and model additions (new product `tasks_enabled`; project relations or a `task` type, decided in system design). Allow-only, no `but not`; "platform OR tenant relation" stays two Checks. `model.fga` and the DSL in `docs/authorization-model.md` must stay equal.
- Data: new tables (tasks, task comments) by a new additive goose migration; no change to existing rows. Seeds (`internal/seed/world.go`) gain tasks and the product flag for the demo tenants.
- Domains: `task` and `task_comment` are separate domains in every backend layer (no generic `Kind` abstraction).
- Clients: web gets a Tasks tab, product toggle row, permissions entries; no change to the Agent or Engine; Inspector and `tuple-catalog.md` follow whatever tuples are added.
- Infra / third party: none.

## Decisions taken with PO / biz
- Access: every project member who can view reads tasks and comments and may comment; contributors create/edit/delete tasks (internal members, external admins in the project, custom roles with contribute/edit/manage) -> user, 2026-11-21.
- Tasks are a platform-toggled product `tasks_enabled`, like documents/photos/blueprints -> user, 2026-11-21.
- One optional assignee, any project member (internal or external) -> user, 2026-11-21.
- Weight is a positive number (default 1) and the project shows weighted progress -> user, 2026-11-21.
- UI: user left it to us ("think a good UI by yourself"); proposal above, approved with this document.

## Open questions
None. Defaults chosen for later design to confirm: dates are calendar dates (no time of day, no time zone); weight allows decimals up to 2 places; comment length limit 4000 chars, description 10000.
