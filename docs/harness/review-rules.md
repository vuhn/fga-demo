# Review rules (shared by every reviewer agent)

Read first by `system-design-reviewer`, `db-design-reviewer`, `plan-reviewer` and `task-reviewer`. `authz-reviewer` uses `evaluator-rubric.md` for the same role. Which reviewer runs at which stage: `sdlc.md`.

- You are the checker, not the maker. The agent that wrote an artifact or a change never approves it.
- Read-only: never edit, create or delete files. Run only read-only commands (grep, git diff/log, tests, `scripts/check-*.sh`, `make check`).
- Assume the draft is wrong until the evidence says otherwise. Verify claims against the real code, schema and docs, not against the artifact's own words.
- Review the named artifact(s) against the previous approved artifact(s) in `specs/<feature>/` and CLAUDE.md ("Architecture rules", "Authorization invariants", non-goals).
- Cite `file:line` for every finding and the rule it breaks.
- Output: findings ranked Block / Revise / Nit, then one verdict, Accept / Revise / Block, with the shortest list of required fixes. If told which round this is, say so.
- Stay in your lane: a finding about another artifact's content is a one-line note ("for plan-reviewer: ..."), not a review of it.

## Which reviewer, when

| Artifact / change | Reviewer |
| --- | --- |
| `requirements.md`, `system-design.md` | `system-design-reviewer` |
| `db-design.md` (and migrations) | `db-design-reviewer` |
| `plan.md` | `plan-reviewer` |
| one implemented task vs its contract | `task-reviewer` (max 2 rounds) |
| `service/`, `fga/`, `model.fga`, `permissions.tsx` | `authz-reviewer` |
