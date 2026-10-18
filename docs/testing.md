# Tests

What the unit and integration tests assert.


`tuples_test.go` asserts the pure relation builders: role assignment writes one role and deletes the other three; a crossed affiliation/role pair emits nothing; a product toggle is only the wildcard; a created document or photo emits `tenant`, its folder or album, and `owner`; a created folder or album emits `root` at top level or `parent` below, plus `public` only when public; `Public` is the single visibility marker; grants map `external_users`/`external_admins` to their usersets, reject `internal_users` and editor for `external_users`; remove member deletes the four role relations and the member's direct grants and never a group or marker tuple.

`model_integration_test.go` (build tag `integration`, skips without `OPENFGA_API_URL`) creates a throwaway store, writes `api/fga/model.fga` and the seed tuples from `api/internal/seed`, and asserts with Check and ListObjects:

- Internal members (Bob, Cara) list every Acme folder, including private Company, Board, Specs, and Partners. Cara can edit Secret but not delete it, and cannot change visibility; Bob can.
- External users (Dan) list exactly Client and the shared Partners, and their documents. Legal (public flag under private Board) and Specs/Pricing (private under public Client) stay hidden. Dan cannot edit or create.
- External admins (Fay) create folders but not private ones, upload and edit in Client, edit Client Brief but not delete or share it, delete their own Visit Agenda, and cannot see Specs, Partners, or Company.
- Bob as a Globex external user sees only public Globex content and cannot create.
- Sharing private Board with all external admins lets Fay view Board, Q3 Plan, and Legal but not edit; Dan still cannot.
- Making Client private hides it and its content from Dan and Fay (Fay still sees her own file); making Board and Specs public reveals Board, Legal, Legal Hold, and Pricing.
- Disabling photos blocks every photo check while the owner tuple stays; re-enabling restores access.
- Removing Dan's role tuple removes all access, including the stale Partners share.

`role_integration_test.go` asserts that a bare internal admin is allowed every permission an internal user is, on every Acme object.

