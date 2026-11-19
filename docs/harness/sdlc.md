# AI-native SDLC

How a non-trivial feature moves through this repo. Based on Claude's AI-native SDLC playbook, mapped onto our own stages. Small simple tasks skip it (use CLAUDE.md and the matching skill directly).

Principle: the agent drafts and checks; a person approves at each gate. Skills guide, hooks and scripts guarantee, and a separate reviewer checks the maker. The agent may act up to the release gate, never through it.

| # | Stage | Skill | Artifact (`specs/<feature>/`) | Checker | Human gate |
| --- | --- | --- | --- | --- | --- |
| 1 | Clarify requirements | `clarify-requirements` | `requirements.md` | - | Requester / PO approves |
| 2 | System design | `system-design` | `system-design.md` (sequence + data flow) | `design-reviewer` | Approve design |
| 3 | DB design | `db-design` | `db-design.md` + migration | `design-reviewer`, `check-migrations.sh`, hook | Approve schema |
| 4 | Plan | `plan-tasks` | `plan.md` (tasks, hours, deps, release order) | `design-reviewer` | Approve plan |
| 5 | Build | `build-task` + `go-api` | PR per task, tests | `make check`, `authz-reviewer` | Code review (15-30 min) |
| 6 | Test | `test-plan` | test section of `plan.md` | `make verify-live` | Confirm manual results |
| 7 | Release | `release-docs` | `release.md` | - | A person releases from the doc |
| 8 | Monitor | `monitor-triage` | new `requirements.md`, `lessons.md` | - | Owner triages |

Entry point: `/feature <name or prds/file.md> [--until <stage>]`; the run can stop after any stage (see `using-the-harness.md`). Stages that do not apply are `skipped: <reason>` in the feature README.

## Enforcement layers
- Skills (advisory): `.claude/skills/*`.
- Deterministic: `make check` runs `scripts/check-architecture.sh`, `check-migrations.sh` (breaking-change lint for migrations above baseline 11; opt out per line with `-- breaking-ok: <reason>`), `check-specs.sh` (artifact structure). `.claude/settings.json` hooks block edits to committed migrations and lint after migration edits.
- Review: `design-reviewer` (artifacts), `authz-reviewer` (code). The agent that wrote something does not approve it.

## Metrics to watch (by hand from git history for now)
Time from first requirement to approved design; requirements/design edits after planning started (rework); share of tasks passing `make check` on the first pass; PR size and review time; rollbacks; repeat incidents of one class (each gets a lesson and a check).

## Maintenance
`CLAUDE.md`, skills, hooks, and agents are code: change them in reviewed PRs. When the agent repeats a mistake, add the correction to the relevant skill or CLAUDE.md, or better a check to `make check`; log it in `lessons.md`.
