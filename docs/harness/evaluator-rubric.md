# Evaluator rubric

Used by `.claude/agents/authz-reviewer.md` (a different role from the one that made the change). Score each 0-2 and cite `file:line` evidence for anything below 2.

| Dimension | 0 | 1 | 2 |
| --- | --- | --- | --- |
| Correctness | Behavior wrong or untested | Main path works | Main and edge paths verified (Check, ListObjects, write ordering) |
| Authorization invariants | Breaks a CLAUDE.md invariant | Minor deviation | Every invariant holds |
| Verification | No evidence | Static tier only | `make check` plus live tier when authz changed |
| Scope discipline | Unrelated changes, non-goals scaffolded | Small drift | Only what was asked |
| Spec alignment | Docs/README/model out of sync | One file stale | Spec updated first, all aligned |
| Handoff readiness | Checklist fails | Progress stale | `progress.md` and feature list current |

**Verdict:** Accept (no 0, total >= 10), Revise (any 0 or total 7-9), Block (any invariant score 0 or total < 7).

Tune against human judgment: when the reviewer and a person disagree, tighten the wording here and note the change below.

## Tuning log

- (none yet)
