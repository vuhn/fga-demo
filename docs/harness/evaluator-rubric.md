# Evaluator rubric

Used by `.claude/agents/authz-reviewer.md` (a different role from the one that made the change). Score each 0-2 and cite `file:line` evidence for anything below 2.

| Dimension | 0 | 1 | 2 |
| --- | --- | --- | --- |
| Correctness | Behavior wrong or untested | Main path works | Main and edge paths verified (Check, ListObjects, write ordering) |
| Authorization invariants | Breaks a CLAUDE.md invariant | Minor deviation | Every invariant holds |
| Verification | No evidence | Static tier only | `make check` plus live tier when authz changed |
| Scope discipline | Unrelated changes, non-goals scaffolded | Small drift | Only what was asked |
| Spec alignment | Docs/README/model out of sync | One file stale | Spec updated first, all aligned |
| Handoff readiness | Checklist fails | Progress stale | `progress.md` current |

**Verdict:** Accept (no 0, total >= 10), Revise (any 0 or total 7-9), Block (any invariant score 0 or total < 7).

Tune against human judgment: when the reviewer and a person disagree, tighten the wording here and note the change below.

## Tuning log

- (none yet)

## Python services layering checklist (agent, engine)

Scripts check names, imports and sizes (`scripts/check-architecture-python.sh`, `scripts/check-python-shape.py`). A reviewer checks what they cannot:

- Is each handler method really decode, ONE use case, encode? No rule, default or formula computed in it.
- Does every gateway, use case, service and model carry the domain's word (`BriefDrafter`, `PlanCheck`), not the mechanism's (`Tool`, `Registry`, `Loader` of a protocol)?
- Does each mapper convert both ways with no decisions in it? Is a DTO field leaking into a model (empty string for "absent", JSON text instead of a value)?
- Is a prompt a pure function in `service/prompts/`, with model-specific tuning left in the adapter?
- Does a use case read as business steps (validate, ask the Engine, ask the model, retry with feedback), with its dependencies passed in?
- Go: does a handler hold a `...View` function or a `...JSON` type, or a service import `handler`? (`mapper/<domain>_mapper.go` holds the first two.) Is `entity` used for anything but a technology's own row or wire shape?
- Would a new Engine operation need only a proto method, a gateway method, one `@rpc` adapter method and a mapper pair?

## Go usecase and gateway shape checklist (api)

Review what `scripts/check-architecture.sh` and `scripts/check-go-shape.go` cannot judge (they check the method shape, the input/output structs, no model field in an output, the gateway param count and the business-layer imports):
- `input.X` carries everything the caller supplies (actor, tenant id, ids, body fields) and nothing derived; a handler builds it in `mapper/<domain>_mapper.go`, not inline.
- `output.X` has its own copied fields (no `domain/model` field, no embedded model, no alias of a model); inputs and outputs live in one file per domain (`input/folder.go`, `output/folder.go`) and a shared shape sits in that file; nothing is dropped or renamed against the old JSON view (compare the replay diff).
- Services keep every Check, `PlatformOr` and `WriteGuard` call; the only diff on those lines is `in.Actor` / `in.TenantID` instead of loose params.
- A gateway method with more than 3 params after `ctx` takes `model.<Action><Model>Params` (name is the action plus the model, e.g. `NewFolderParams`); no struct is a bag of unrelated fields.
- `usecase/helper` holds shared rule interfaces only; no operation hides there to avoid the input/output rule.
