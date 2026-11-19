---
name: feature
description: Heavy, user-invoked only. Runs the full AI-native SDLC (requirements, system design, DB design, plan, build, test, release docs) for a BIG feature that spans several deployables or needs design and a plan. The user starts it explicitly with /feature; never start it on your own, never for a small task, bug fix, refactor of one file, doc edit, or question.
disable-model-invocation: true
---

# /feature <name or prds/file.md> [--until <stage>]

If the argument is a path (e.g. `prds/new-requirement.md`), read it as the raw ask and derive the feature name (kebab-case) from it; confirm the name with the user in the first message.

Orchestrates the chain in `specs/README.md`. You draft, the user approves; never start a stage before the previous artifact is approved.

## Only for big features (check first, before creating anything)

This skill is heavy: several artifacts, reviewers and approval gates. It runs only because the user typed `/feature`. Never invoke it yourself, and do not suggest it for small work.

A feature is big when at least two hold: it touches two or more deployables (`api/`, `web/`, `agent/`, `blueprint-engine/`, infra); it adds or changes a table, a contract (proto/HTTP), or an authorization decision; it needs more than one PR; its requirement is ambiguous or needs product input.

If the ask is small (one deployable, no schema or contract change, one PR, clear requirement: a bug fix, a field, a text change, a single endpoint tweak), do not run the chain. Say so in one or two sentences, name why, and offer the direct route instead (the matching service skill: `go-api`, `python-agent`, `python-engine`, `db-design` for a lone schema change, or just do it with plan mode), then stop and let the user decide whether to continue with `/feature` anyway. If it is borderline, ask with AskUserQuestion: full chain, or `--until requirements`/`--until plan`, or direct.

## Stop point (`--until`)

The user can stop the chain at any stage. Stages, in order: `requirements`, `system-design`, `db-design`, `plan`, `build`, `test`, `release`. Aliases: `design` = through `db-design`; `clear` = `requirements`.

- Parse `--until <stage>` from the arguments, or from plain words ("only clarify the requirements", "up to the plan", "design only"). Also `--only <stage>` to run just that one stage (it still reads the earlier approved artifacts).
- No stop point given: ask once with AskUserQuestion how far to go (requirements only / through design / through plan / through build / all the way), recommending the smallest that fits the ask.
- Run stages up to and including the stop stage, with the usual gate on each artifact. Skipped stages count as passed.
- Record it in the feature README as a line `Stop after: <stage>` above the status table, mark later rows `not started (stopped)`, then finish with a summary of the artifacts and the exact command to continue: `/feature <name> --until <next stage>`.
- Continuing later: read the README status table and start at the first unapproved stage; a new `--until` replaces the old stop point. Never redo an approved artifact unless the user asks (then also re-check the later artifacts that depend on it and say which became stale).

## Steps

1. Create `specs/<name>/` (kebab-case) from `specs/_templates/README.md`. Read `docs/harness/progress.md` first.
2. Triage: for each stage decide needed or `skipped: <reason>` and tell the user (e.g. only a table -> skip system-design sequence diagram; infra only -> skip db-design). Record in the status table.
3. Run stages in order, each by its skill, writing the artifact from `specs/_templates/`:
   `clarify-requirements` -> `system-design` -> `db-design` -> `plan-tasks` -> `build-task` (one task at a time) -> `test-plan` -> `release-docs`.
4. Gate after every artifact: show a short summary + path, ask for approval (or corrections), set the status to `approved`. Ask questions with AskUserQuestion when something is genuinely the user's call; otherwise pick the sensible default and say so.
5. For `system-design.md`, `db-design.md`, `plan.md` run the `design-reviewer` subagent before asking for approval and fix what it finds. After build changes in `service/`, `fga/`, `model.fga`, `permissions.tsx` run `authz-reviewer`.
6. Do not commit or push unless asked. Update `docs/harness/progress.md` at the end of each stage.
7. After release, monitoring starts a new loop: `monitor-triage` produces a new `requirements.md`.

Spec first: if the feature changes behavior, update `docs/` and the `ARCHITECTURE.md` files (see CLAUDE.md "Source of truth") as part of the plan, not afterwards.
