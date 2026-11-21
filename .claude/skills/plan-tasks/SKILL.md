---
name: plan-tasks
description: Stage 4 of the feature SDLC. Break an approved design into small, independently mergeable PR tasks sliced by delivery endpoint (not by code layer), each safe to merge into the base branch, with e2e tests, estimates, dependencies, per-environment infra tasks, rollout and rollback; writes specs/<feature>/plan.md. Use after system and DB design are approved.
---

# Plan

Input: approved `requirements.md`, `system-design.md`, `db-design.md`. Output: `specs/<feature>/plan.md` from `specs/_templates/plan.md`. Read the code that will change so tasks name real files, routes and RPCs. The reference example of a good plan is `specs/re-architecture-system/plan.md` (strategy section, per-task merge safety, soak gate).

## Definitions (read first, they are used exactly this way everywhere)

- **Deployable** (also written "service" in older text): a unit that is built, released and rolled back on its own. In this repo: `api/` (Go), `web/` (React), `agent/` (Python gRPC), `blueprint-engine/` (Python gRPC), and `infra` (compose, proto contracts, scripts, tooling, env/config). It does NOT mean a Go `service/` layer file, a usecase, or a class.
- **Layer**: a folder inside one deployable (domain, usecase, service, repo, handler, mapper). Layers are never a unit of planning.
- **Endpoint**: what a caller invokes: an HTTP route, gRPC RPC, stream, queue consumer, job, CLI command, or a web page/flow.
- **Slice**: one endpoint implemented through all its layers inside one deployable.

**One task = one PR = exactly one deployable x one endpoint (or one skeleton / migration / infra item / switch).**
- Allowed in one PR: every layer of that endpoint inside that one deployable, its tests, its docs, and a small additive contract change it needs.
- Not allowed in one PR: code in two deployables (e.g. `agent/` and `api/`); several unrelated endpoints; one layer across many endpoints.
- An endpoint that spans deployables becomes one task per deployable, the provider first and the consumer after it, each merge-safe alone. Example: "chat revise" = T-a `agent/` adds the RPC (dark, additive), T-b `api/` calls it behind a switch (default off), T-c flips the switch.
- Shared contracts (proto, OpenAPI) go in their own `infra` task before both sides unless the change is tiny and additive.
- `web/` tasks follow the same rule: one page/flow per PR, after the API endpoint it calls is merged.

Wrong: "T3 api: all usecases for estimates", "T4 agent+api: revise flow", "T5 api: add routes and services and repos for 6 endpoints". Right: "T3 api: GET /estimates/:id (handler, service, repo, e2e test)", "T4 agent: Revise RPC", "T5 api: POST /blueprints/:id/revisions via Agent client, flag off".

## Two rules that override everything else

1. **Slice by delivery endpoint, never by code layer.** A task delivers one *endpoint* end to end in one deployable: one HTTP route, gRPC RPC, stream, queue consumer, CLI command, scheduled job, or one web page/flow. All layers it needs (model, gateway, repo, service, handler, wiring, docs) ship in that one PR, with an e2e test through the real entry point. "Add all the usecases", "add all the repos", "add the handlers" is the wrong cut: half-built layers cannot be e2e tested and tempt breaking changes.
2. **Every PR must leave the base branch working.** The base branch is whatever the PRs merge into (`main`, or an integration branch such as `feat/re-architecture`; record it in the plan, and if it is an integration branch, add a final "merge to main" gate with the same rules). After any single task merges, in any allowed order: `make check` is green, the system starts and every pre-existing behavior is unchanged, and the new code is either complete and tested or unreachable. No task may rely on a later task to repair it.

## Step by step

0. **State the strategy** (top of plan.md): base branch; the delivery pattern chosen (see "Making a merge safe"); how new code stays dark until complete; the switch/cutover step; rollback per phase; the gate before irreversible steps (contract/delete). Mark each task's type (below).
1. **Inventory the endpoints and deployables.** List every endpoint/RPC/page/job the feature adds or changes, per deployable, plus every infra resource. This list is what you slice; derive it from `system-design.md` (sequence diagram arrows between services = endpoints), and trace each through the existing code.
2. **Skeleton first, only if the structure does not exist.** If the service, package, module, layer folders, proto/contract, test harness, CI/make target, or health endpoint the feature needs is missing, the first tasks are skeleton tasks (type S): empty-but-running structure, wiring, a health/"hello" path, tooling, architecture-rule entries, with **no behavior and no effect on existing flows**. Keep the skeleton minimal (just what the first endpoint slice needs); later slices grow it. A skeleton is mergeable on its own because nothing calls it yet. If the structure already exists, skip this.
3. **One task per endpoint slice (type E).** Include handler/transport, mapping, service, gateway/repo, tests and the doc edits for that endpoint. Order slices so the simplest, lowest-risk one goes first and proves the skeleton (a thin vertical slice), the riskiest last. Where one endpoint is too big for a 15-30 min review, split by *behavior* inside the endpoint (happy path, validation/authz denials, streaming/cancel, edge cases), each part still tested, the endpoint staying unregistered or flagged off until the last part lands. Never split by layer unless the layer-only PR is provably dark and behavior-free (like a skeleton).
4. **Data changes (type M)** come from `db-design.md` and must be covered completely (see "DB design coverage" below). The expand migration is its own task, before any code that reads or writes it; backfill is its own task; contract (drop/rename) is its own later task after the soak. Never bundle a migration with code that depends on it unless the migration is purely additive and the code tolerates both states. The migration task owns everything that makes the schema change safe, not just the SQL file: the migration (Up and Down), seed `Truncate` and demo rows, `docs/database-schema.md` and `docs/seed-data.md`, `scripts/check-migrations.sh`, a dry run in a rolled-back transaction, a check that the previous build still runs against the new schema (run the existing smoke/integration suite on the migrated database), and a restore point (`pg_dump`) before applying. Repository code (queries, inserts, constraints behavior) belongs to the endpoint slice that uses it, with a real-Postgres test for anything concurrency- or constraint-dependent. Applying the migration in each environment is part of the rollout (release order), with backup, verification query and rollback.
5. **Infrastructure per environment (type I).** For each infra/3rd-party resource (compose service, env var/secret, queue, bucket, DNS, OpenFGA model, provider key), list the environments that exist (local, dev, staging, prod, per region/tenant if relevant) and make **one task per resource per environment**, or smaller (per resource part: create, configure, expose, switch). Order: create (unused) -> verify -> wire -> switch -> remove old. Prod is its own task after staging has soaked, with an explicit human gate. Config/env changes must have safe defaults so merging the code before the config is applied does not break startup.
6. **Switch / cutover tasks (type F)** are separate from build tasks: the one-line config flip, flag default change, or route registration that makes new behavior live, one flow at a time, each with its own rollback (unset the variable, flip the flag back) and verification. Building and switching must never be the same PR for risky flows.
7. **Contract / cleanup tasks (type C)**: delete old code, old columns, fallbacks, flags, only after a soak gate with measurable conditions (e.g. N days or one working day on the new path, evals/smoke within baseline, the user says go).
8. **Docs tasks (type D)** only for reconciliation; behavior docs change in the same PR as the behavior (spec first).
9. **Per task** record: ID, type, service, endpoint/slice (the route/RPC/page, or the skeleton/infra item), scope (layers/files), estimate in hours, dependencies, **merge safety** (state of the base branch after merge: how it stays dark or backward compatible, and what proves existing behavior unchanged), **e2e test** (a real call through the entry point against a running stack; unit tests are in addition), **rollback** (revert alone is enough, or the exact switch to undo).
10. **Order and parallelism.** Dependencies must be real (a task needs another's code or data, not just its place in a list). Mark what can run in parallel (different services/endpoints). Producer before consumer: the service that provides an endpoint merges and is deployed before the one that calls it; additive contract first.
11. **Rollout and release order** feeds `release-docs`: the merge order is not necessarily the go-live order; list when each switch is flipped per environment.
12. **Test plan**: map every acceptance criterion to an e2e step, a smoke script, or an eval; per task say which one gates its merge. Regression guard: baselines recorded *before* the first behavior-touching task (smoke replay, eval, test inventory) so "unchanged" is measurable.
13. Run `plan-reviewer`; fix findings; gate: user approves. `build-task` follows the plan; deviations are written back to it.

## DB design coverage (required whenever `db-design.md` exists and is not skipped)

`plan.md` carries a table with one row per item of `db-design.md` (each table/column/index/constraint change, each migration step expand/backfill/contract, each checklist box, each `force-allow-breaking` exception) naming the task that delivers it, the proof (test, lint, dry run, query) and the rollout step per environment. An item with no task is a plan defect. Also: the order "migration before the code that reads or writes it" appears in Depends on; backfill and contract are separate tasks with their own gates (soak gate for contract); and the plan states how old and new application builds coexist while the migration is applied.

## Task types

| Type | Meaning | Merge-safe because |
| --- | --- | --- |
| S | Skeleton: structure, tooling, wiring, health | Nothing calls it; no behavior |
| E | One endpoint slice, all layers, e2e tested | Unreachable/flagged until complete, or purely additive and complete |
| M | Expand migration / backfill | Additive and defaulted; old code unaffected |
| I | One infra resource in one environment | Created unused; safe defaults; per-env order |
| F | Switch/cutover of one flow | One-line, reversible, verified; old path still there |
| C | Contract: delete old path | Only after soak gate; revert restores |
| D | Docs reconciliation | No runtime effect |

## Making a merge safe (pick per task, state it)

- **Additive only**: new route/RPC/table/column/field/env var; nothing existing changes shape. Contracts stay backward compatible (optional new fields; never repurpose, rename, or remove).
- **Dark launch**: code present but unreachable: route not registered, handler not wired, flag default off, env var unset selects the old path.
- **Strangler / parallel path**: new implementation beside the old one behind a switch; switch one flow at a time; delete the old one in a later C task after the soak.
- **Expand -> backfill -> contract** for data, spread over separate releases.
- **Tolerant readers first**: deploy code that handles both old and new shape before anything produces the new shape.
- **Producer before consumer** for service-to-service changes; consumer keeps a fallback until the producer is live in every environment.
- **Safe defaults**: merging code before its config/infra exists must not break startup or tests.
- **Moves in place**: refactors keep old import/API paths working (adapter) until callers are migrated, in separate PRs.

## Anti-patterns (reject these in review)

- A task per layer (usecases, then services, then repos, then handlers): nothing works until the last PR lands.
- A PR that is only safe if the next PR merges too, or merges in a fixed order not stated in the plan.
- Build and switch in one PR for a risky flow; migration and the code that needs it in one PR (non-additive).
- Infra for all environments in one task; prod changed in the same PR as dev.
- An endpoint merged with only unit tests and no call through the real entry point.
- Estimates that hide a 200+ line-of-logic PR inside one task; "refactor everything" tasks.
- A cleanup task with no soak gate; a switch with no rollback.
- Dependencies that are just list order.

## Checklist before asking for approval
- [ ] Base branch named; strategy says how new code stays dark and how it is switched on and off.
- [ ] Every endpoint/RPC/page/job in the design is exactly one slice (or a few behavior splits) in one deployable (never two in one PR).
- [ ] Skeleton tasks exist only where structure is missing, and have no behavior.
- [ ] Each task has merge safety, an e2e test, and a rollback; each reviewable in 15-30 min.
- [ ] Infra is per resource per environment, prod last with a gate; defaults make early merge safe.
- [ ] Migrations follow expand/backfill/contract in separate tasks; contract and deletes sit behind a soak gate.
- [ ] Every item of `db-design.md` appears in the "DB design coverage" table with a task, a proof and a rollout step per environment (seed `Truncate`, docs, dry run, old-build compatibility, restore point included).
- [ ] Merging in any dependency-respecting order keeps the base branch green and running (walk the merge sequence once and state the state after each merge).
- [ ] Baselines recorded before the first behavior-touching task.
