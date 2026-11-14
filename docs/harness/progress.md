# Progress

Single place for "where are we". Read at session start, update at session end.

## Current verified state

- Static tier: `make check` passes (vet, unit tests incl. engine client and blueprint tuples, architecture rules, docs/model sync, engine pytest 29 passed, web build). Verified 2026-11-13 on `main`, uncommitted.
- Live tier: `make verify-live` passes, including the new TestBlueprintAccess. Verified 2026-11-13 (reseeded the local store).
- HTTP smoke (API + engine + qwen3:latest, 2026-11-13): draft 403 for Dan and for Globex (product off); Cara draft 19s, create 201, list, Dan get 404 and list [], SVG/PDF drawing, revise ("add a study") 201 v2 with rooms kept, stale base 409, form edit, restore, versions, viewer share to Dan, editor share 400 `cannot_upload`, Dan revise 403. Sketch draft reaches the engine; 502 because `qwen2.5vl:7b` is not pulled locally.
- Engine golden eval (qwen3:latest, 2026-11-13, after Text2BIM Phase 1, Enhancer off): extraction 15/15, end-to-end 14/15, avg 28s (was 14/15, 13/15, 37s). With `--enhance`: 13/15, 8/15, 112s. Known failure: narrow lots (10 m wide) need a front-to-back hall layout.

## Active task: Text2Blueprint POC (`prds/plans/text2blueprint-poc.md`)

- Part A engine: done (`blueprint-engine/`).
- Part B Go `blueprint` domain: done (model, migration 00006, all layers, wiring, seed: Acme only, integration tests, docs). authz-reviewer: Accept (10/12); its fixes applied: the external-admin-creates-blueprint exception is documented, the "externals via share or custom-role grant" wording is corrected, a custom-role row was added to TestBlueprintAccess, and the plan's route list was updated.
- Part C web: done. `Blueprints.tsx` tab (list, New blueprint modal: describe / upload sketch / fill form -> review brief -> Generate), `BlueprintDetail.tsx` (SVG preview; Chat, Brief, Checks, Versions tabs; DXF/PDF download; rename/share/delete), `components/BriefEditor.tsx`, ProjectDetail tab with `?tab=`, route `/projects/:id/blueprints/:bid` behind `nav.blueprints`, permissions/Inspector/TenantDetail/api.ts entries, Vite proxy 5-minute timeout. `make check` passes. authz-reviewer: Accept (10/12); fixes applied (web-ui ACTIONS/`revise` and 409 wording, not-found state instead of an endless loader, reload on tenant switch). Not yet walked in a browser.

- Follow-up (2026-11-13): nested rooms and fixtures. A brief room may set `parent` (bathroom/storage/laundry only, one level, max 2 per parent): the layout carves it from its parent's block and its only door opens into the parent (`nested` check). Rooms carry `fixtures` (16 kinds, type defaults when omitted), placed deterministically by `fixtures.py`, drawn on `A-FURN`, unfit ones reported as `fixture_fit` warnings. New edit ops `set_inside`, `add_fixture` (idempotent), `remove_fixture`, `add_room.inside`. Brief form has Inside and Fixtures columns. Engine pytest 28 passed; live qwen3 revise "add an ensuite with a bathtub inside the master bedroom" works. No Go/authz changes (brief is opaque JSON to the API).
- Follow-up (2026-11-13): drawings are 1:50 by default; the sheet grows A3 -> A2 -> A1 -> A0 to fit (`draw.Sheet`), 1:100 only beyond A0. Verified on the Tower A blueprint: 1:50 @ A3.
- Follow-up (2026-11-13): `POST /blueprints` takes optional `prompt` (≤2000 chars; the description or sketch note), stored as v1 `message`; the Chat tab lists it first. Smoke-tested via the API (v1 message stored, 2001 chars is 400). Blueprints created before this have no stored prompt.

- Follow-up (2026-11-13): Text2BIM adoption plan (`prds/plans/text2bim-adoption.md`), Phase 1 done, engine only (no Go/web/authz change; new response fields ignored by the Go client). `prompts.py` knowledge pack; Enhancer before drafting (`assumed: …` notes, `enhance` flag); semantic retry with error feedback (3 attempts, `llm_attempts`); checks carry `category`/`issue_id`/`elements`/`hint` plus geometry, collision, information and design rules; Reviewer `POST /v1/review` (≤3 rounds, forbidden ops, 15% area cap, keep only if the score drops); `history` on revise/review; a revise that adds a new error kind runs one review round. Engine pytest 42 passed; `make check` passes. Live qwen3: draft "3 bed … ensuite and walk-in closet in the master" correct (61s), review of conflicting sides 3 -> 0 issues, "a bit bigger please" after "make the kitchen bigger" resizes only the kitchen.

- Follow-up (2026-11-13): Text2BIM Phase 1 eval showed the Enhancer hurts on qwen3 8B, so it is opt-in (`enhance: true`). Phase 2: `POST /blueprints/:id/reviews` (can_edit, base_version; no-op review is 400 and stores nothing), migration 00007 (`kind` adds `review`), `BlueprintReviewer`, chat history (last 6 create/revise messages) sent on revise and review, web "Fix issues with AI" in Checks plus fix hints and an "AI review" badge. Phase 3: IFC4 export (`ifc.py`, ifcopenshell 0.9.0.post1), `GET …/drawing?format=ifc`, IFC download button. `make check` passes (engine pytest 43). HTTP smoke (cara, dan, throwaway blueprint deleted): review 201 fixes 3 -> 0 side errors in 69s, second review 400, stale 409, Dan 403, revise "a bit bigger please" after the create prompt "make the kitchen bigger" resizes only the kitchen, IFC 200 attachment. `make verify-live` not run (no model change; it would reset data).

- Follow-up (2026-11-13): open plan. "Remove Hall partitions to create a continuous, open-plan living/kitchen/dining layout" returned 422: the model tried `remove_room Hall`, and the hall is layout-made. Added `Brief.open_plan` (draft field too), op `set_open_plan {open}` (forbidden to the reviewer), opening kind `open` (walls between hall/entry/living/kitchen/dining become wall-less passages with short returns; no doors there; drawn as nothing; unfilled full-height IFC void; counted for reachability), a hall-specific `EditError` that points the retry at `set_open_plan`, prompt conventions, and an Open plan switch in the brief form. Engine pytest 47. Live qwen3: the prompt now gives `set_open_plan` on the first attempt and every error check still passes. Brief stays opaque JSON to Go; no API/authz change.

- Follow-up (2026-11-14): sketch upload returned 502 because `qwen2.5vl:7b` was not pulled (ran `ollama pull`). Added DXF upload (`image/vnd.dxf`, also by `.dxf` extension): the engine reads text labels, extents and closed-outline areas and uses the text model, no vision. Engine pytest 49; Go vet/tests and web build pass. Restart `make api` and `make engine` to pick it up.

- Follow-up (2026-11-14): DXF export now uses 25 AIA-style layers (exterior/interior walls and poche, door leaves vs swings, per-group furniture and sanitary layers, room boundary polylines, split dimensions/north arrow/border). Engine only; pytest 49. Restart `make engine`.

## Last session

- Built Part C (web); docs/web-ui.md and web/ARCHITECTURE.md aligned.
- Earlier: built Part B. Docs aligned: authorization-model, decision/tuple catalogs, http-api, database-schema, seed-data, testing, operations, web-ui (spec for Part C), ARCHITECTURE files, CLAUDE.md, README (step 12, engine run).

## Blockers

None.

## Next best action

1. Walk README steps 1-12 in the UI (restart `make api` after the reseed; `make engine`; `make web`).
2. Text2BIM Phase 4 (paper-style eval: 25 prompts x 3 runs, models, adherence; re-test the Enhancer on a larger model). Walk "Fix issues with AI" and the IFC download in the browser.
3. Engine follow-up: front-to-back hall layout for narrow lots (eval `tight_lot`); `ollama pull qwen2.5vl:7b` for sketches.
4. Commit (nothing from the harness, companies/projects, or blueprint work is committed yet).
