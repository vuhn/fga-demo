# Decisions

Why the repo is shaped this way. One entry per decision; add new ones at the top.

## AI-native SDLC: committed specs/, skills for stages, scripts and hooks for guarantees
- **Reason:** the author's SDLC (requirements, design, DB design, plan, build, test, release, monitor) is run as an artifact chain in `specs/<feature>/` so each stage hands off a reviewed file and git is the audit trail. Skills are advisory, so the non-negotiable DB rule (never break the running system) is also a lint in `make check` and a hook that blocks edits to committed migrations.
- **Rejected:** keeping artifacts in the gitignored `prds/` (no history); Jira-style tracking; scheduled error-triage automation (release and monitoring stay docs and templates, a person releases).
- **Where:** `docs/harness/sdlc.md`, `.claude/skills/`, `.claude/settings.json`, `scripts/check-migrations.sh`, `scripts/check-specs.sh`.

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

## 2026-11: Blueprints product (prds/plans/text2blueprint-poc.md)

- **Brief is the only editable source.** The LLM produces or edits a JSON brief; deterministic engine code makes all geometry. Rejected: LLM-generated geometry, tool calling (flaky on small local models; one structured-output call instead).
- **Python services (Engine and Agent, gRPC), stateless, no auth.** Go does every Check, storage, and versioning; the engine never sees tenant or user ids. The engine is called after the Check and before any write. Rejected: Go-native geometry (no ezdxf/Shapely equivalent).
- **Blueprint sits directly in a project, no public chain.** Externals reach one only through a share or a custom-role `grant_*`. External admins who are project members may create (and own) blueprints: a deliberate exception to "externals contribute only in externally visible content", since blueprints have no visibility regime; they see only their own unless shared. Rejected: putting blueprints in folders (would couple to the documents product).
- **Versions are append-only Postgres rows, optimistic `base_version` (409).** No OpenFGA tuples per version. Restore copies an old version forward.
- **Picture and PDF bytes are transient.** An uploaded PDF or picture is converted to a DXF by the engine and discarded (name, type and size stay on the blueprint); the converted DXF is the one stored original (respects the "no file bytes" non-goal). The vision model and the sketch-to-brief path are gone: a model reading a picture into a brief gave a plan unlike the picture.
- **Seed enables blueprints on Acme only** so Globex shows the product-off state; no blueprints are seeded (seed must not depend on the engine).

## 2026-11: Cost estimation agent (prds/plans/cost-estimation.md)

- **An agent that asks, not a form with defaults.** A 2D plan cannot give heights, what exists or the quality level, and those move the cost more than the geometry does. The agent asks, and an assumption is recorded (`assumed`) only when the person says "I don't know / you decide".
- **Code owns the flow and the numbers, the LLM owns language.** Missing slots, takeoff, pricing and every guard are code. A small local model proved unreliable at flags and scope (it assumed unprompted and read "rooms exist" as "finished"), so its reading is gated by what the message actually says.
- **No price database yet.** Unit rates are researched on the web per estimate through a `PriceSource` interface (a database can replace it) and stored with source, quote and date inside the estimate; a price without a quote that is on the page is rejected, and a code with no price is shown as unpriced, never as zero.
- **Engine stays stateless.** The API stores the agent's state as opaque JSON and sends it each turn; sessions are guarded by a turn counter like blueprint versions by `base_version`. A turn is saved only if it completes.
- **Authorization reuses the model.** Sessions are `can_edit` (they spend LLM and search calls; a viewer gets 403 and cannot read the chat), finished estimates are `can_view`. No `model.fga` change.

## 2026-11: One layer structure for all services (docs/harness/layer-structure.md)

- **The "Layers" section of `api/ARCHITECTURE.md` became the common standard.** `docs/harness/layer-structure.md` holds the dependency direction, the layer table with its Go and Python paths, the rules and the enforcement map; each service's `ARCHITECTURE.md` keeps only its own instance. Rejected: a copy of the rules per service (they drift, as the Python and Go wording did before).
- **Input and output structs stay a Go rule** until the Python services adopt them by a separate decision.

## 2026-11: Usecase input/output structs and gateway params (specs/usecase-io-structs)

- **A usecase method is `Method(ctx, in input.X) (output.X, error)`.** `ctx` is the only param outside the struct (actor and tenant id move in); inputs live in `usecase/input/<domain>.go`, outputs in `usecase/output/<domain>.go` (one file per domain, like `usecase/<domain>.go`; a struct is named after its interface). Rejected: one file per usecase (about 240 files). Rejected: keeping `actor, tenantID` as loose params (the PRD example), which still leaves long positional lists.
- **Outputs are their own structs with copied fields, never `domain/model`.** A usecase no longer hands a model to the handler; a shape shared by several usecases sits in the same domain file and the usecase output embeds it. Rejected: a struct wrapping the model (callers would still reach the model).
- **Gateway methods with more than 3 params after `ctx` take `model.<Action><Model>Params`.** Applies to every gateway, including Authz, Agent and Engine; an existing fitting struct (`model.CheckRequest`) is reused.
- **`Authorizer` lives in `usecase/helper`.** It is a shared decision port, not an operation, so the usecase rule has no exceptions.
- **Business layer independence is a check, not a habit.** `domain`, `usecase` and `service` import only the standard library and each other.
- **One PR.** The refactor is one mechanical change, so it lands atomically (no ratchet allow-list); the cost is review size, handled with ordered commits.
