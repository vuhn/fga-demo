# Tests

What the unit and integration tests assert.


`tuples_test.go` asserts the pure relation builders: role assignment writes one role and deletes the other three; a crossed affiliation/role pair emits nothing; a product toggle is only the wildcard; a created document or photo emits `tenant`, its folder or album, and `owner`; a created folder or album emits `root` at top level or `parent` below, plus `public` only when public; `Public` is the single visibility marker; grants map `external_users`/`external_admins` to their usersets, reject `internal_users` and editor for `external_users`; remove member deletes the four role relations and the member's direct grants and never a group or marker tuple.

`role_relations_test.go` asserts custom-role tuples: two anchors on create, one `grant_*` userset per permission and no implied permission, and removal of anchors, grants, and assignees. `seed_roles_test.go` asserts the seeded Auditor and Vendor tuples exist and that the seed writes no `assignee` tuple.

`project_integration_test.go` asserts the union (tenant role a+b plus project role c), that external members see nothing in a project they are not in, that internal members reach every project, and that stale project membership fails closed. `entity/company_test.go` asserts name keys and derived affiliation; `seed/world_test.go` asserts seed consistency.

`model_integration_test.go` (build tag `integration`, skips without `OPENFGA_API_URL`) creates a throwaway store, writes `api/fga/model.fga` and the seed tuples from `api/internal/seed`, and asserts with Check and ListObjects:

- Internal members (Bob, Cara) list every Acme folder, including private Company, Board, Specs, and Partners. Cara can edit Secret but not delete it, and cannot change visibility; Bob can.
- External users (Dan) list exactly Client and the shared Partners, and their documents. Legal (public flag under private Board) and Specs/Pricing (private under public Client) stay hidden. Dan cannot edit or create.
- External admins (Fay) create folders but not private ones, upload and edit in Client, edit Client Brief but not delete or share it, delete their own Visit Agenda, and cannot see Specs, Partners, or Company.
- Bob as a Globex external user sees only public Globex content and cannot create.
- Sharing private Board with all external admins lets Fay view Board, Q3 Plan, and Legal but not edit; Dan still cannot.
- Making Client private hides it and its content from Dan and Fay (Fay still sees her own file); making Board and Specs public reveals Board, Legal, Legal Hold, and Pricing.
- Disabling photos blocks every photo check while the owner tuple stays; re-enabling restores access.
- Removing Dan's role tuple removes all access, including the stale Partners share.

`blueprint_integration_test.go` creates two blueprints with `BlueprintRelations.Created` (none are seeded) and asserts: internal members view and edit, only the owner or an internal admin manages; externals see nothing without a share (no public chain); a share outside the external's projects fails closed; an editor tuple to an external user still cannot edit and the `external_users` editor group is refused (`cannot_upload`); `can_create_blueprint` follows `can_contribute` (Fay yes, Dan no) and is false in Globex, where the product is off; disabling the wildcard denies every blueprint check while other products stay; removing a member drops a stale share.

`infrastructure/blueprintengine/client_test.go` asserts the engine client against a fake server: 4xx becomes `validation` with the engine's message (or FastAPI's field errors), 5xx or no connection becomes `engine_unavailable`, and a generate result without a model is kept as not built.

`blueprint-engine/tests/` (pytest, run by `make check`; no LLM) asserts the deterministic engine on hand-written briefs: layout stability across revisions, entrance rotation, shrink-to-fit, rule checks (including the geometry, collision, information and design rules), edit ops, the agent loop with a scripted fake LLM (retry with error feedback, chat history, reviewer acceptance and guardrails, the opt-in Enhancer), DXF/SVG/PDF output, and the IFC export (element counts, space areas, fixtures contained in their space). `make engine-eval` runs the golden prompts against the live Ollama model (not part of `make check`; `--enhance` turns the Enhancer on). The engine client test in `api/internal/infrastructure/blueprintengine` checks that revise and review send the chat history. The review route reuses `can_edit`, which `TestBlueprintAccess` already covers (viewer share: no edit; editor share and internal members: edit).

`role_integration_test.go` asserts that a bare internal admin is allowed every permission an internal user is, on every Acme object. It also assigns Dan to Acme's Auditor role and asserts he can view private Company, Secret, and All-hands and can view products, and still cannot edit, manage, contribute, manage members, or create a private folder.

`scripts/check-architecture.sh` (run by `make check`) asserts the architecture rules: no Gin, pgx, or OpenFGA SDK imports in `domain`, `usecase`, or `service`; no tuple or object-id literals outside `infrastructure/fga`; no role/affiliation branching in services (stored-invariant files allowlisted); no `but not` or `condition` in `model.fga`; `model.fga` equals the DSL block in `docs/authorization-model.md`; no OpenFGA relation names in web code outside `permissions.tsx` and the Inspector page. Each failure prints what, why, and fix.
