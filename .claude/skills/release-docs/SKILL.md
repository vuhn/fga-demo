---
name: release-docs
description: Stage 7 of the feature SDLC. Write the release note, release checklist and release guideline for a manual release; writes specs/<feature>/release.md. Use when a feature's tasks are merged and tested.
---

# Release docs

A person releases by following these docs, so they must be exact and ordered.

1. Input: `plan.md` (release order, infra changes, migrations), `db-design.md`, `docs/operations.md`. Output: `specs/<feature>/release.md` from the template.
2. Release note: user-facing summary, who is affected, behavior changes.
3. Checklist: merged tasks, `make check`/`make verify-live` results, migration review and backup/restore point, env/config/infra changes applied, feature flags.
4. Guideline, numbered in release order: how migrations apply (embedded, on API start), restart order (API, engine; `cmd/api` after any reseed; never run `make seed` in production, it RESETS data), config changes, verification commands after each step.
5. Rollback: exact steps per component; state what is irreversible (data migrations, contract steps).
6. What to watch: error logs/endpoints, expected rates, duration, who is on call.
7. Gate: user approves; the person releasing ticks the checklist. Do not run release steps yourself.
