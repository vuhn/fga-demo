# Plan: <feature>

Base branch: <main (default) | integration branch: why a breaking change cannot be made safe, scope, exit, how it stays fresh>. Every task keeps it green and running when merged (see `.claude/skills/plan-tasks`).

## Strategy
- Delivery pattern: <additive / dark launch / strangler / expand-backfill-contract>, and how new code stays unreachable until complete.
- Cutover: <how and per flow/environment each switch is flipped>; rollback: <unset X / flip flag / revert>.
- Soak gate before contract tasks: <measurable conditions and the owner>.
- Baselines recorded before the first behavior-touching task: <smoke replay, eval, test inventory>.

## Endpoint inventory
| Deployable | Endpoint / RPC / page / job | New or changed | Slice task |
| --- | --- | --- | --- |

## Infrastructure / 3rd-party changes
One row per resource per environment (local, dev, staging, prod ...); prod last, behind a gate.
| Item | Environment | create / update / delete | Notes (safe default so early merge is harmless) | Task |
| --- | --- | --- | --- | --- |

## Tasks
Types: S skeleton, E endpoint slice, M migration, I infra (one env), F switch, C contract/cleanup, D docs. One task = one PR = one deployable (api, web, agent, blueprint-engine, or infra) x one endpoint, never two deployables in a PR; 15-30 min review. Slice by endpoint, not by layer.

| ID | Type | Deployable | Endpoint / slice | Scope (layers / files) | Est (h) | Depends on | Merge safety (state of base after merge) | E2E test | Rollback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| T1 | S | | | | | - | | | |

Total estimate: <h>. Critical path: <T1 -> T3>. Parallel: <tasks>.

## DB design coverage
One row per item of `db-design.md` (skip the section only when DB design is skipped). An item without a task is a plan defect.
| db-design item | Task | Proof (test / lint / dry run / query) | Rollout per environment (backup, apply, verify, rollback) |
| --- | --- | --- | --- |

## Merge sequence walk
After T1: <state>. After T2: <state>. (Check that any dependency-respecting order keeps `make check` green and existing behavior unchanged.)

## Release order
Merge order is not go-live order. List when each switch (F) and each environment's infra goes live, and verification after each.

## Test plan
| Step | Acceptance criteria | Gates which task |
| --- | --- | --- |

## Risks
