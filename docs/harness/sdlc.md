# AI-native SDLC

How a non-trivial feature moves through this repo. Based on Claude's AI-native SDLC playbook, mapped onto our own stages. Small simple tasks skip it (use CLAUDE.md and the matching skill directly).

Principle: the agent drafts and checks; a person approves at each gate. Skills guide, hooks and scripts guarantee, and a separate reviewer checks the maker. The agent may act up to the release gate, never through it.

| # | Stage | Skill | Artifact (`specs/<feature>/`) | Checker | Human gate |
| --- | --- | --- | --- | --- | --- |
| 1 | Clarify requirements | `clarify-requirements` | `requirements.md` | - | Requester / PO approves |
| 2 | System design | `system-design` | `system-design.md` (sequence + data flow) | `design-reviewer` | Approve design |
| 3 | DB design | `db-design` | `db-design.md` (DDL, ER diagram, indexes, access paths) + migration | `design-reviewer`, `check-migrations.sh`, hook | Approve schema |
| 4 | Plan | `plan-tasks` | `plan.md` (endpoint-sliced, merge-safe tasks, hours, deps, rollout) | `design-reviewer` | Approve plan |
| 5 | Build | `build-task` + the service's skill (`go-api`, `python-agent`, `python-engine`) | PR per task, tests | `make check`, `task-reviewer`, `authz-reviewer` | Task contract check, code review (15-30 min) |
| 6 | Test | `test-plan` | test section of `plan.md` | `make verify-live` | Confirm manual results |
| 7 | Release | `release-docs` | `release.md` | - | A person releases from the doc |
| 8 | Monitor | `monitor-triage` | new `requirements.md`, `lessons.md` | - | Owner triages |

Entry point: `/feature <name or prds/file.md> [--until <stage>]`; the run can stop after any stage (see `using-the-harness.md`). Stages that do not apply are `skipped: <reason>` in the feature README.

## Skill per service

Before editing code in a service, load that service's skill (the Skill tool, or `/<name>`). All of them apply the common structure in [layer-structure.md](layer-structure.md). It holds the layer map, the "touch exactly these" checklist for a new operation, the rules that bite and what "done" means. It does not replace CLAUDE.md; it points at it.

| You edit | Load | Deterministic check |
| --- | --- | --- |
| `api/` (Go: handlers, services, gateways, fga, postgres, wire) | `go-api` | `scripts/check-architecture.sh` (with `scripts/check-go-shape.go`), `go vet`, `go test` |
| `agent/` (Python Agent: LLM work, gRPC API) | `python-agent` | `scripts/check-architecture-python.sh`, `scripts/check-python-shape.py`, agent pytest |
| `blueprint-engine/` (Python Engine: geometry, DXF/IFC, plan ops) | `python-engine` | same Python scripts, engine pytest |
| `api/migrations/` | `db-design` | `scripts/check-migrations.sh` |
| `web/` | no skill; CLAUDE.md "Frontend permissions" and `web/ARCHITECTURE.md` | `npm run build` |
| `proto/` (a contract between services) | the skill of each side that implements it (`go-api` for the API client, `python-agent`, `python-engine`), contract first in `docs/service-contracts.md` | `make proto` |

A change that spans two services (a new Engine operation the Agent calls) loads both skills. When you notice a service has a repeated mistake or a new rule, add it to that service's skill (see Maintenance).

## Enforcement layers
- Skills (advisory): `.claude/skills/*`; one per service (see "Skill per service") plus one per SDLC stage.
- Deterministic: `make check` runs `scripts/check-architecture.sh` (Go shape rules in `scripts/check-go-shape.go`, planted-violation tests in `scripts/test-architecture-go.sh`), `check-migrations.sh` (breaking-change lint for migrations above baseline 11; opt out per line with `-- force-allow-breaking: <reason>`), `check-specs.sh` (artifact structure). `.claude/settings.json` hooks block edits to committed migrations and lint after migration edits.
- Review: `design-reviewer` (artifacts), `task-reviewer` (a task against its contract, max 2 rounds), `authz-reviewer` (authorization in code). The agent that wrote something does not approve it.

## Metrics to watch (by hand from git history for now)
Time from first requirement to approved design; requirements/design edits after planning started (rework); share of tasks passing `make check` on the first pass; PR size and review time; rollbacks; repeat incidents of one class (each gets a lesson and a check).

## Maintenance
`CLAUDE.md`, skills, hooks, and agents are code: change them in reviewed PRs. When the agent repeats a mistake, add the correction to the relevant skill or CLAUDE.md, or better a check to `make check`; log it in `lessons.md`.
