# Decisions

Why the repo is shaped this way. One entry per decision; add new ones at the top.

## Allow-only model, no `but not`, no conditions
- **Reason:** "Private wins" is an intersection over the ancestor chain, so deny semantics are unnecessary and every tuple is auditable in the Playground.
- **Rejected:** deny tuples / `but not`: makes list results and ListObjects harder to reason about.
- **Enforced by:** `scripts/check-architecture.sh`.

## Platform admin OR tenant relation is two Checks in the service
- **Reason:** system admins must not become tenant members or see files.
- **Rejected:** a model relation that makes platform admins tenant members.

## Grants write Postgres first, revocations delete OpenFGA first
- **Reason:** both orders fail closed: a crash never leaves access in OpenFGA that Postgres cannot explain.
- **Rejected:** outbox/workers (explicit non-goal).
- **Where:** `WriteGuard.Grant` / `WriteGuard.Revoke` in `api/internal/service/helper.go`.

## Each content domain is fully separate (folder, document, album, photo)
- **Reason:** behavior is identical today, but each domain evolves independently and stays readable without a `Kind` abstraction.
- **Rejected:** generic service/handler keyed by kind.
- **Exception:** shared cross-domain rules only in `service/helper.go` and `postgres/grant_rows.go`.

## Postgres is the display record; OpenFGA decides
- **Reason:** teaches the split; collections use ListObjects then `WHERE tenant_id = $1 AND id = ANY($2)`.
- **Rejected:** select whole tenant and filter in Go.

## No repo-level feature list
- **Reason:** the integration tests and README script already are the feature list; a hand-maintained status file drifts and costs context at scale.
- **Rejected:** `feature-list.json` (removed). For a multi-session task, keep a temporary task list in the plan and delete it when done.

## Harness: one verification command, rules as scripts
- **Reason:** prose rules were not checkable by an agent (`docs/harness` follows Learn Harness Engineering lectures 2, 9, 10).
- **Rejected:** a Stop hook running `make check` on every session (opt-in, not added).

## 2026-11: companies, projects, project roles (prds/update-tenant-structure.md)

- Open TODO(Vu) default: internal members reach every project in their tenant; external members need project membership. Needs confirmation.
- Claim = platform admin merges unclaimed companies into the claiming tenant's own company (409 on conflicts). Proof is out of band.
- Affiliation derived from company; FGA relation chosen at add time.
- Go names kept for tenant roles (`Role*`); project roles are `ProjectRole*`. Roles and project roles are created by the platform admin.
