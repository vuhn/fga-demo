# Clean-state checklist

Run before ending a session. A session is not done until every box holds.

- [ ] `make check` passes (vet, tests, architecture rules, docs/model sync, web build).
- [ ] If the change touched authorization: `make verify-live` passes.
- [ ] Code, `ARCHITECTURE.md` files, `docs/`, and `README.md` agree (spec first, then code).
- [ ] `api/fga/model.fga` equals the DSL block in `docs/authorization-model.md`.
- [ ] If you reseeded, the API was restarted (the store id changes).
- [ ] `feature-list.json` matches reality: no `passing` without a recorded run, at most one `in_progress`.
- [ ] `progress.md` has current state, blockers, and the next best action.
- [ ] No stray debug output, commented-out code, TODOs, or temp files.
- [ ] A fresh session can start with `make init` without manual fixes.
