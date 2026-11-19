# Feature specs

One directory per feature, committed. Each stage reads the artifact the previous stage wrote; a human approves each one before the next stage starts (the agent drafts, the person decides). Git history is the audit trail.

```text
requirements.md -> system-design.md -> db-design.md -> plan.md -> code + PRs -> release.md -> (monitor) new requirements.md
```

Start with `/feature <name>`. Templates are in `_templates/`. Stages that do not apply are marked `skipped: <reason>` (a feature that only adds a table skips most of system design; an infra-only change skips db design).

Each feature directory has a `README.md` with this status table (`scripts/check-specs.sh` checks it):

| Stage | Artifact | Status |
| --- | --- | --- |
| Requirements | requirements.md | draft / approved / skipped: why |
| System design | system-design.md | draft / approved / skipped: why |
| DB design | db-design.md | draft / approved / skipped: why |
| Plan | plan.md | draft / approved / skipped: why |
| Build | PRs per plan.md | n/N tasks done |
| Test | test section of plan.md | pending / passed |
| Release | release.md | draft / approved / released |

`prds/` is gitignored raw input; `specs/` is the committed record. The spec-first rule in CLAUDE.md still applies: when a feature changes behavior, update `docs/` and the architecture files in the same change.
