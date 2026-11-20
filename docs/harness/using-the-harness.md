# Using the harness

Short answer: **you do not call the stage skills one by one.** Call `/feature` once; it runs the stages in order, calls the right skill for each, and stops at every gate for your approval. You only answer questions and say "approved" or "change X".

## When to use `/feature` (and when not)

`/feature` is the heavy path: several artifacts, reviewer agents and approval gates. It is configured so Claude never starts it by itself; only you do, by typing `/feature`. Use it for a **big feature**: at least two of (two or more deployables, a new table/contract/authorization decision, more than one PR, an unclear requirement). For anything smaller (a bug fix, a field, one endpoint tweak, a doc edit) just ask normally: Claude loads the service skill (`go-api`, `python-agent`, `python-engine`, `db-design`) and follows CLAUDE.md. If you do type `/feature` for a small ask, it tells you so and offers the direct route.

## The normal flow

1. Write the raw ask in `prds/new-requirement.md` (rough is fine: problem, who, what you want; `prds/` is gitignored scratch).
2. Start a fresh session in the repo and run:
   ```text
   /feature prds/new-requirement.md
   ```
3. Claude then, by itself:

| Step | What Claude does | What you do |
| --- | --- | --- |
| Triage | Proposes which stages apply or are skipped (table only -> no sequence diagram; infra only -> no DB design) | Confirm or correct |
| Requirements | Reads code, asks 1-4 batched questions, writes `specs/<feature>/requirements.md` | Answer; check items marked "to confirm with PO"; say "approved" |
| System design | Components to change, Mermaid sequence + data-flow, authz impact; runs `design-reviewer` | Review the diagram; approve |
| DB design | Non-breaking migration design (expand/backfill/contract); runs `design-reviewer` | Check the "backward compatible because" column; approve |
| Plan | Per-service tasks (15-30 min review each), hours, dependencies, release order; runs `design-reviewer` | Check sizes and order; approve |
| Build | One task at a time with tests, `make check`, `authz-reviewer` where needed | Review each PR-sized diff; say "next task" |
| Test | Checklist from acceptance criteria, runs what it can, records real results | Do the manual parts; confirm |
| Release docs | `release.md`: note, checklist, ordered guideline, rollback, what to watch | Approve; a person releases from it |

4. After release, when errors appear: `/monitor-triage` with the logs. It writes a new `requirements.md` and the loop starts again.

Progress lives in `specs/<feature>/README.md` (status table) and `docs/harness/progress.md`, so you can stop and resume: start a new session and say `/feature <feature-name>`; it reads the status table and continues from the first unapproved stage.

## Stop at any stage (`--until`)

You choose how far a run goes. Stages: `requirements`, `system-design`, `db-design`, `plan`, `build`, `test`, `release` (`design` = system-design + db-design).

```text
/feature prds/new-requirement.md --until requirements   # 1. only make the requirement clear
/feature prds/new-requirement.md --until design         # 2. requirements + system design + db design
/feature prds/new-requirement.md --until plan           # 3. requirements, design, and the plan
/feature invoice-export --until build                   # continue an existing feature further
/feature invoice-export --only db-design                # run one stage alone (reads earlier artifacts)
```

Plain words work too ("only clarify the requirements for prds/x.md"). With no stop point, Claude asks how far to go. The stop point is saved in `specs/<feature>/README.md` (`Stop after:`); when it stops it prints the command to continue. Continuing starts at the first unapproved stage and never redoes approved artifacts unless you ask; if you change an earlier artifact, Claude lists which later ones became stale.

Typical uses: `requirements` to align with the PO before anyone designs; `design` to review architecture and schema with the team; `plan` to hand tasks to people or other sessions, then `/build-task Tn` per task.

## When to call a skill directly

| Situation | Call |
| --- | --- |
| Big feature, new requirement | `/feature` (default) |
| Resume the build of one planned task | `/build-task T3` |
| Only a schema change, no feature | `/db-design` |
| Small Go change (a route, a field, a fix) with no spec | `/go-api` |
| Small Python Agent change (a use case, a prompt, an RPC) with no spec | `/python-agent` |
| Small Python Engine change (a rule, a tool, a handler) with no spec | `/python-engine` |
| Re-run one stage after changing an earlier artifact | that stage's skill, e.g. `/system-design` |
| Production errors | `/monitor-triage` |
| Just need release docs for finished work | `/release-docs` |

Small simple tasks (typo, one-line fix, tiny bug) skip the chain entirely: ask normally, Claude follows CLAUDE.md, and `make check` still applies.

## Your approval points (the only things that need you)

1. Requirements: is this what we mean, and is the PO-facing scope right?
2. System design: right components, right interactions, right authorization?
3. DB design: will the running system keep working during rollout?
4. Plan: right split, order, and estimates?
5. Each build diff: normal code review.
6. Test: manual results.
7. Release: you or a teammate runs it; Claude never does.

## Tips for effective use

- Put in the raw ask what only you know: why, who, deadlines, constraints, what must not change. Claude can read the code, so do not describe it.
- Answer clarifying questions fully at the requirements stage; changes after planning started are rework and get logged.
- Keep one feature per `/feature`. If the ask has two independent outcomes, split it into two files.
- Approve explicitly ("approved") so the status table stays true. Corrections are cheaper at the gate than in code.
- For parallel work, use one git worktree per planned task (tasks that do not depend on each other), each with its own session running `/build-task Tn`.
- When Claude repeats a mistake, tell it to add a lesson (`docs/harness/lessons.md`) and, if mechanical, a check to `make check`. Skills, hooks and CLAUDE.md are code: change them through a reviewed PR.
- Safe-by-default guards you will meet: committed migrations cannot be edited (add a new one); breaking migration statements fail `make check` unless marked `-- force-allow-breaking: <reason>` and justified in `db-design.md`; `make verify-live` RESETS local data.
- Claude does not commit or push unless you ask. When it does: you are the sole author, nothing goes to `main`, PR bodies follow `.github/pull_request_template.md`, temp `wt-*` branches are never pushed and are removed with `scripts/clean-temp-branches.sh` after the PR. `scripts/sync-base.sh` pulls the integration branch at clock-in; a PreToolUse hook enforces the git rules.

## Example

```text
you:     /feature prds/new-requirement.md
claude:  Feature name "invoice-export"? Stages: requirements, system design, plan, build, test needed; DB design skipped (no schema change); ok?
you:     yes
claude:  3 questions (who can export, format, size limit) ...
you:     answers
claude:  specs/invoice-export/requirements.md ready (4 acceptance criteria). Approve?
you:     approved
claude:  ... system-design.md (+ design-reviewer: Accept). Approve?
...
claude:  plan.md: T1 api (2h), T2 web (1.5h, after T1). Approve?
you:     approved
claude:  T1 done: make check green, authz-reviewer Accept. Review the diff, then say "next".
```

Map of all stages, checkers and gates: [sdlc.md](sdlc.md).
