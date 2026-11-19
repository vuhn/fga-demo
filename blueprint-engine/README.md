# blueprint-engine

This is the stateless engine for the Blueprints product. It turns a JSON **brief** into a floor plan and renders that plan to DXF, SVG and PDF, and exports it as an IFC4 BIM model.

- It has no auth and no storage. Only the Go API calls it, after the Go API has run its own OpenFGA Checks.
- The design and the review of the original proposal are in `../prds/plans/text2blueprint-poc.md`.

## The rule

**The LLM only produces or edits the brief. Code does every piece of geometry.**

| Step | Who | Module |
|---|---|---|
| Prompt (→ requirements, with the opt-in Enhancer) → brief | LLM (Ollama, JSON-schema output) + clean-up in code | `llm_steps.py`, `prompts.py` |
| PDF / PNG / JPEG → DXF | no LLM: pdfium paths (vector PDF) or OpenCV tracing (picture), then `dxf_plan.py` reads it | `convert.py` |
| Chat message (+ earlier messages) → edit ops → brief | LLM + `apply_ops` (room names fuzzy-matched in code) | `llm_steps.py`, `domain/edits.py` |
| Failed checks → edit ops (Reviewer) | LLM, kept only when the issue score drops | `llm_steps.review` |
| Brief too big → shrink toward minimums | code | `pipeline.generate_fitted` |
| Brief → zoned slicing layout (front band, hall, back band) | code, deterministic, stable vs. previous version | `domain/layout.py` |
| Layout → walls, doors, windows, net areas | code | `domain/building.py` |
| Fixtures (bed, toilet, kitchen run…) placed in each room | code, deterministic | `domain/fixtures.py` |
| Rule checks | code (Shapely) | `domain/checks.py` |
| Brief and plan operations (generate, create plan, draw plan, apply plan ops, run checks) | usecase Protocols in `usecase/`, one service per op in `service/`, graph built only in `wire.py`; the legacy FastAPI handlers in `app.py` call them; DXF drawing/reading sits behind `domain/gateway` (`infrastructure/plan_drawing.py`) | `usecase/`, `service/`, `wire.py` |
| DXF, SVG, PDF from one ezdxf document | code | `draw.py` |
| IFC4 BIM model (ifcopenshell) | code | `ifc.py` |

### Rooms inside rooms, and fixtures

- A room can name a `parent`: an ensuite or a walk-in closet inside a master bedroom, or a pantry inside a kitchen.
  - Only `bathroom`, `storage` and `laundry` rooms can be inside another room. The parent must be a top-level room, and it can hold at most 2 of them.
  - A nested room cannot set `side`. The layout treats the parent and its nested rooms as one block. It then carves the nested rooms off one end of the parent as a full-depth strip; if the parent holds two nested rooms, they are stacked in that strip.
  - A nested room's only door opens into its parent; it never opens onto the hall. Reachability checks therefore go through the parent.
- Each room has a list of `fixtures`, which are basic housing components: `bed`, `wardrobe`, `desk`, `shelves`, `sofa`, `tv_unit`, `dining_table`, `kitchen_counter`, `sink`, `stove`, `fridge`, `toilet`, `basin`, `shower`, `bathtub` and `washer`.
  - If a room omits the list, it gets its type's defaults. For example, a bedroom gets a bed and a wardrobe, and a bathroom gets a toilet, a basin and a shower. An empty list means no fixtures.
  - Code places each fixture deterministically: against a wall, or centred for a dining table. Fixtures stay clear of door swings and of each other, and tall items avoid windows.
  - The fixtures are drawn on per-kind layers (`A-FURN-BEDS`, `A-FURN-STOR`, `P-FIXT-WC`, ...; see the DXF layers below).
  - Chat revisions can use `add_room` with `inside`, `set_inside`, `add_fixture` and `remove_fixture`. For example, "add an ensuite to the master bedroom" or "put a desk in bedroom 2". If a fixture does not fit, the `fixture_fit` check gives a warning; the fixture is never moved outside its room.

### DXF layers
AIA-style names so a CAD user can toggle, colour and plot each kind separately: `C-PROP` (lot); `A-WALL-EXTR`, `A-WALL-INTR` (wall outlines) and `A-WALL-PATT` (poche); `A-DOOR` (leaves), `A-DOOR-SWNG` (arcs), `A-GLAZ` (windows); `A-AREA-BNDY` (one closed polyline per room, net area) and `A-AREA-IDEN` (names and areas); furniture `A-FURN-BEDS`, `A-FURN-STOR`, `A-FURN-DESK`, `A-FURN-SEAT`, `A-FURN-TABL`, `A-FURN-EQPM`, `A-FLOR-CASE` (counters), `A-EQPM-APPL` (stove, fridge, washer); sanitary `P-FIXT-WC`, `P-FIXT-BASN`, `P-FIXT-BATH`; `A-ANNO-DIMS`, `A-ANNO-DIMS-OVAL`, `A-ANNO-NARW`, `A-ANNO-BORD`, `A-ANNO-TTLB`. Each layer has a description. Doors, windows and fixtures are not loose lines: each is one named block reference (`dining_table-Dining-<id>`, `door-<id>`) on its kind's layer, with invisible attributes (`ID`, `KIND`, `ROOM`, `WIDTH`, `CONNECTS`), so the dining table and the living-room table are separate objects you can select, move or count. The SVG and PDF come from the same document.

### Open plan

- `open_plan: true` on the brief removes the walls between the hall, entry, living, kitchen and dining rooms (top-level rooms only). Each removed wall becomes an opening of kind `open`: no door, no leaf drawn, a full-height unfilled `IfcOpeningElement` in IFC. A short return stays where a remaining wall meets it. Bedrooms, bathrooms and nested rooms keep their doors, and reachability counts the open passages.
- The hall is never a room of the brief, so it cannot be removed. Chat requests such as "open-plan living" or "remove the hall partitions" map to the `set_open_plan {open}` op, and an op that names the hall fails with an error that points the model at `set_open_plan` (the retry fixes it). The reviewer may not change `open_plan`, because it is the user's intent.

### Agent loop (after Text2BIM, arXiv 2408.08054)

The plan is `prds/plans/text2bim-adoption.md`. The LLM still only edits the brief; these steps make that more reliable.

- **Architect knowledge pack** (`prompts.py`): conventions, architectural rules (zoning, wet-room cluster, minimum areas and widths, default fixtures), one few-shot example per step, and the op documentation rendered from the `EditOps` schema (op docstrings and field names), so prompts cannot drift from what `apply_ops` accepts.
- **Enhancer** (opt-in, `{"enhance": true}`): `brief_from_text` first expands the prompt into a requirements list with at most 5 assumptions, then drafts the brief from it. The assumptions come back as `assumed: …` notes. It is off by default because on qwen3:latest (8B) it invents rooms and lowered the golden eval end-to-end score from 13/15 to 8/15, the long-prompt hallucination the paper reports; try it with a larger model (`make engine-eval` with `--enhance`).
- **Semantic retry**: when the LLM's output fails validation or `apply_ops` (an unknown room, a bad nesting), the failed output and the error go back to the model, up to 3 attempts in total (`llm_attempts`). An `unsupported` request is never retried. The caller gets a 422 only after the third failure.
- **Issues**: every failed check has a `category` (geometry, collision, information, program, design), a stable `issue_id` (`rule:room[:n]`), the `elements` involved and a `hint` naming an edit op. Besides the program rules (area, width, window, side, reachability, nesting, fixture fit) there are defensive geometry and collision rules (room overlap, footprint coverage, fixture overlap, fixtures in door swings or under windows, door swings that clash), information rules (names, element data) and design warnings (aspect ratio over 2.5, a bedroom on the street side, kitchen away from dining, a bedroom far from any bathroom).
- **Reviewer** (`POST /v1/review`): up to 3 rounds. Each round sends the failed issues with their hints, the brief and the earlier rounds to the LLM and applies the returned ops. The reviewer may not remove or rename rooms or change the lot or the entrance, may not move the total area by more than 15%, and must change something. A round is kept only when the score (errors × 10 + warnings) drops and no new kind of error appears; otherwise it is discarded and the loop stops. Whatever is left is reported as "N error(s) remain for you to decide". `review_rounds` lists each round's ops, scores and outcome.
- **Chat memory**: `revise` and `review` take `history`, the earlier chat messages (the last 6 are used), so "a bit bigger please" refers to the room of the previous request. A chat revise that introduces a new kind of error runs one review round automatically.

### IFC export

`format: "ifc"` writes an IFC4 file: project, site, building and one storey; an `IfcWall` per wall, extruded 2.7 m about its centerline; an `IfcOpeningElement` per door, window and entrance, filled by an `IfcDoor` (2.1 m) or `IfcWindow` (sill 0.9 m, 1.2 m high); an `IfcSpace` per room with `Qto_SpaceBaseQuantities.NetFloorArea`; a 0.2 m floor `IfcSlab` under the footprint; and each fixture as `IfcFurniture`, `IfcSanitaryTerminal` or `IfcElectricAppliance`, contained in its room's space. It opens in any IFC viewer (BIMvision, the IfcOpenShell/Bonsai viewer, web-ifc).

Conventions:

- Units are metres and the origin is the lot's bottom-left corner. +y is north.
- Dimensions are measured to wall centerlines.
- Exterior walls are 0.30 m and interior walls are 0.12 m.
- Sheets are drawn at 1:50 on the smallest landscape ISO sheet that fits the house and its dimensions, from A3 up to A0. Only a house too big for A0 drops to 1:100. The title block states the scale and the paper.

## Run

```bash
ollama pull qwen3                # text model (8B); qwen3:14b if you have 32 GB
make engine                     # http://localhost:8090 serves a dev harness page
make engine-test                # pytest (no LLM needed)
make engine-eval                # golden prompts against the live model
```

Environment variables:

| Variable | Default |
|---|---|
| `OLLAMA_URL` | `http://localhost:11434` |
| `BLUEPRINT_TEXT_MODEL` | `qwen3:latest` (the 8B; `qwen3:14b` with 32 GB) |

Ollama runs natively, not in Docker, because Docker on macOS has no GPU access.

## HTTP

| Route | Body | Returns |
|---|---|---|
| `POST /v1/brief/from-text` | `{prompt, enhance?: true}` | `{brief, notes}` |
| `POST /v1/convert/dxf` | multipart `file` (PNG, JPEG or PDF, ≤20 MB; PDF first page), `scale` (N of 1:N, 0 = not given), `dpi`, `longest_m` (real length of the longest side, wins over `scale`) | `{dxf: base64 (metres), report: {method, scale_source, metres_across, walls, openings, texts, skipped, warnings}}`; vector PDF: filled dark shapes are walls (the biggest cluster, so title blocks and legends drop out), curved strokes door swings, text kept; picture: thick strokes are walls (OpenCV opening, outlines as closed polylines), door-sized gaps between wall ends get a door mark and are bridged; OCR (rapidocr-onnxruntime) reads room names as text and the `xx m²` labels set the scale when none is given (else the overall dimension, else door width; none readable = 422); gaps are classified door or window (leaf stroke, outside wall), furniture is recognised by rules on size, proportion and room name, outlines are squared up; door swing direction and furniture against a wall may be missed; 422 for an unreadable file |
| `POST /v1/plan/revise` (+ `/stream`) | `{model, message, history}` | `{result: {model, checks, ok, issues, notes}, notes, ops, llm_attempts}`; chat edit of an imported plan: LLM emits `PlanOps` (`domain/plan_edits.py`: rename_room, set_room_type, add/remove_fixture, move/rotate/delete_fixture, resize/move/remove_opening, unsupported), code applies them, `run_imported_checks` re-runs; 422 for unresolvable or unsupported ops |
| `POST /v1/plan/edit` | `{model, ops: {ops: [...]}}` | same shape as `plan/revise` (`llm_attempts: 0`); the same `PlanOps` applied directly, no LLM (rename_room, set_room_type, add_fixture, delete_fixture, move_fixture, rotate_fixture, resize_opening, move_opening, remove_opening); 422 for an op that does not fit |
| `POST /v1/export/dxf-patch` | multipart `file` (the original DXF), `base` (the plan as imported), `current` (JSON models) | `{dxf: base64, report: {applied, added, skipped}}`; `dxf_patch.py` applies the differences to the original through the handles in each element's `src` and the plan's `origin`; nothing else in the file changes; nothing kept |
| `POST /v1/import/dxf` | multipart `file` (DXF, ≤20 MB), `unit?` | `{plans: [{index, name, model, room_count, net_area, checks, ok, svg}], report}`; no LLM, deterministic, nothing kept |
| `POST /v1/plan/draw` | `{model, title?}` | same as `/v1/generate/plan`, for a stored plan made from a brief: drawn into our DXF and read back (422 when it is already a CAD plan or would lose area or fixtures); an open plan's merged rooms are named in `result.notes` |
| `POST /v1/generate/plan` | `{brief}` | `{result: {model, checks, issues, ok, notes}, notes, dxf}`: the brief laid out, drawn into our own DXF and read back like an import (`source: "imported"`, `src` handles, no brief); `dxf` is its base64 original, null when the brief cannot be built |
| `POST /v1/generate` | `{brief, previous?, repair?}` | `{result: {brief, model, checks, issues, ok}, notes, attempts, llm_attempts, review_rounds}` |
| `POST /v1/revise` | `{brief, previous?, message, history?}` | the same as generate, plus `ops` |
| `POST /v1/estimate/agent/turn` | `{model, state?, message?, messages?, rate_overrides?}` | NDJSON, the cost-estimation agent's turn (stateless: the API sends the whole `state` back each turn and stores the new one). Events: `progress`, `token` `{text}` (the reply, streamed), `fact` `{name, value, source}`, `question` `{id, text, options, allow_unknown}`, `rate` (one researched unit price), `estimate`, then one `{"event":"done","data":{state, reply, question, estimate}}` or an `error`. See "Cost estimation agent" below. Closing the connection cancels the run at the next step. |
| `POST /v1/revise/stream` | the same as revise | NDJSON, one JSON object per line: `{"event":"progress","kind","stage","message"}` while it works (`kind` is `step`, or `thinking` with a piece of the model's reasoning as `message`; while streaming the LLM call runs with `think: true` and the reasoning is forwarded in chunks of about 150 ms, `BLUEPRINT_STREAM_THINKING=0` turns that off), then exactly one `{"event":"result","data":<revise response>}` or `{"event":"error","status","error","message"}`. A bad request (empty message, malformed body) is the usual 4xx before any line. Stages: `start`, `ask_model`, `retry`, `ops`, `layout`, `checks`, `review`. Closing the connection cancels the run at the next step. Progress lines are never stored; they carry no tenant or user ids. |
| `POST /v1/review` | `{brief, previous?, history?, max_rounds?: 1-3}` | the same as generate |
| `POST /v1/render` | `{model, format: svg\|dxf\|pdf\|ifc, title_block}` | the file bytes |

Errors:

- An edit that still cannot be applied after 3 attempts, or an unsupported request, returns `422 {"error":"invalid_edit","message"}`.
- An LLM that is down or returns bad output returns `502 {"error":"llm_unavailable"}`.

## DXF import (`dxf_plan.py`)

Reads a CAD floor plan into a `BuildingModel` with `source: "imported"` (polygon rooms, any-angle walls, no brief). No LLM.

1. **Units:** `$INSUNITS`, else guessed from the wall extent (3-80 m after conversion) and cross-checked with `DIMENSION` values; `unit` overrides.
2. **Layers:** wall layers by name (`wall`, `mur`, `tuong`, ...; `patt`/`hatch`/`dim`/`furn` excluded); door and window layers or blocks; otherwise the longest unclassified layer. Used and ignored layers are reported.
3. **Rooms:** wall linework is noded and polygonized; thin faces (inscribed width < 0.6 m) are wall solids, the rest are rooms. Single-line walls get a 0.2 m default thickness. A bar is added across each doorway so an exterior door does not leak the room into the outside, and the doorway is filled into the wall solids so it splits the rooms it joins.
4. **Walls and openings:** wall centerlines come from the room edges (thickness measured through the wall solids, collinear pieces merged; the end cap of a wall that ends free is skipped, since a ray cast from it runs along the wall and would measure a bogus 0.6 m); each door/window symbol is fitted to its wall. A door drawn as loose pieces (a leaf line from the hinge and a swing arc centred on it) is anchored at the middle of its doorway, the midpoint of hinge and opposite jamb, which lies on the wall line; the bounding box of leaf and arc lies in the room and can fit the wrong wall.
5. **Labels:** TEXT/MTEXT/attributes inside a room, English and Vietnamese keywords; several labels in one space (open-plan living/kitchen/dining) give one room named "Living / Dining / Kitchen". Unrecognised labels are listed.
6. **Several plans on one sheet** are split by cluster; the caller picks one.
7. **Everything else is kept as drawing items** (`dxf_drawing.py`, `BuildingModel.drawing`, read-only): every entity not used above (not on a wall, door or window layer, not a fixture or opening block, not a room label) is flattened to polylines, points or text in metres and attached to the nearest plan. Dimensions, leaders, furniture linework, hatches, annotation blocks and title blocks are no longer dropped.

### Files with no floor plan

A file whose walls enclose no room (a part, a sketch, a map, a file with no wall layer) is not refused: it imports as one plan named "Drawing" with no rooms, walls or openings and all its entities as `drawing` items (`room_count` 0, no checks, `report.warnings` says why no floor plan was found). Units: `unit` override, else `$INSUNITS`, else guessed from the extent (3-80 m), else 1 unit = 1 m. Only a file with nothing drawable (an empty model space, only hidden layers) is a 422.

`dxf_drawing.py` reads: LINE, ARC, CIRCLE, ELLIPSE, SPLINE, LWPOLYLINE, POLYLINE (2D, 3D, polyface, mesh), HATCH (any boundary), SOLID, TRACE, 3DFACE, POINT, TEXT, MTEXT, block attributes; it explodes INSERT (nested, arrays; one degenerate nested reference loses only itself), DIMENSION, LEADER, MLEADER, MLINE, TABLE. Entities on frozen or off layers are left out. Counted in `report.skipped`, never silent: images, underlays, wipeouts, viewports, rays and construction lines, 3D solids, bodies, regions and surfaces. Curves are flattened with a tolerance of 1/4000 of the drawing's size; colours (ACI or true colour, ACI 7 drawn dark) and layers are kept. At most 120,000 items (warning when cut). The sheet drawing (SVG, PDF, DXF) draws the items on `X-<layer>` layers; a pattern hatch is drawn as an outline. The imported blueprint's original DXF is unchanged, so `source/edited` returns it byte for byte when only the plan is edited.

Fixtures: INSERT blocks whose `KIND` attribute, name or layer matches one of the 16 fixture kinds (0.2-3.5 m) become `PlacedFixture`s in the room holding their centre, facing away from the nearest wall. `ImportReport.skipped` lists what the model cannot hold (spaces under 1.2 m² or 0.6 m wide, fixtures outside every room). Not read as walls: curved walls (flattened to segments), HATCH/SOLID wall fills, xrefs, 3D. Tests: `tests/test_drawing.py` imports the jscad `dxf-parser` sample files in `tests/samples/jscad/` (every file except the three over 100 KB) and a synthetic file with every entity kind; `tests/sample_dxf.py` rebuilds the three apartment plans of the sample sheet from their printed dimensions, `tests/test_import.py` compares rooms, areas, openings and the angled wall with the known answer.

## Cost estimation agent (`estimate_agent.py`, `takeoff.py`, `estimate.py`, `price_source.py`, `price_research.py`)

A conversational agent that estimates what a plan costs to build. Spec: `prds/plans/cost-estimation.md`. A DXF is 2D: it has no heights, structure, finishes or what already exists, so the agent **asks** instead of assuming. A fact the person gives is `user`; an `assumed` fact exists only when they say "I don't know / you decide", and it is shown in the result.

- **Division of work.** Code decides the flow: a fixed state machine (interview, research, compute, present, follow-up), with the list of still-missing slots computed in code (`missing_slots`), so the interview is stable and testable. The LLM does three narrow jobs: reads the person's message into fields (`Extracted`, structured, no live thinking), writes the reply (`LLMClient.stream_text`, plain text streamed as `token` events; a code fallback text is used if the model fails before any text), and, inside the price research, writes search wording and reads pages. The LLM never does quantities, totals or markups.
- **Code is authoritative over a small model.** "Use typical rates" and "I don't know" count only when the words are in the message; a room condition is accepted only when the message talks about conditions ("the rooms already exist" is not one) and only for rooms the message refers to by name or type; the agent's own chips, a kind or quality keyword and a direct answer to the question just asked are read by code.
- **Takeoff** (math in `service/takeoff.py` over `EstimateFacts` (facts with value and source, scope, `fixtures_existing`, `excluded`); `service/price_estimate.py` prices it; `service/plan_summarizer.py` lists rooms, fixtures, openings, `pending_fixtures` and `structure_in_scope`; `takeoff.py` and `estimate.py` are legacy adapters that convert the session with `estimate_models.facts_of`): per room by condition (`empty` shell: finishes only; `existing`: nothing; `to_replace`: demolition plus finishes; `new`: partitions too). Plaster and paint from each room's perimeter times the floor height minus openings, floor and ceiling from the room polygons, wall tile and waterproofing (floor plus a 0.3 m upturn) in wet rooms, skirting, doors and windows from the openings, electrical and plumbing points from room area and fixtures; after an estimate is ready the person can take an item out or put it back ("the doors already exist"; `excluded` rate codes, read from explicit words and never from the model alone), which re-prices from the cached rates and lists "Left out at your request"; a message that changes nothing gets a reply that says nothing changed; `ESTIMATE_SEARCH_PROVIDER=demo` (`demo_search.py`) serves invented price pages so the pipeline runs without a search key; its estimates carry a "DEMO DATA" warning; fixtures the plan draws (toilet, basin, shower, bathtub, sink, kitchen counter and wardrobe by metre run) are priced as supply-and-fit **unless the person says they already exist**: a floor plan shows where things go, not what is installed, so the agent states that it assumes none exist and asks which do (`fixtures_existing`; rooms already finished are skipped; loose furniture and appliances are not priced); structure (columns on a 3.5 m grid, beams, slab, strip footing, rebar at 110-120 kg/m³, formwork 7 m²/m³, excavation, roof) only when in scope. Every line has `basis` and a confidence: `measured` (from geometry), `derived` (geometry plus a fact) or `assumed` (an assumed fact or a structure rule of thumb). A plan with no rooms is `unsupported`.
- **Price research** (`price_source.PriceSource`, `price_research.LLMWebPriceSource`): no price tables. For each rate code the takeoff needs, the LLM writes 1-2 queries (code appends the person's region and drops any query naming another city), a `SearchProvider` (Brave or Serper) returns pages, the engine fetches them (robots.txt, text only), the LLM extracts `{price, currency, unit, quote}` and code accepts a price only if its quote is on the page and carries the number, converts the unit (m², m³, kg, t, m, each) and VND/USD, drops outliers beyond 3x the median and cites only the sources the rate rests on. Three or more prices give the 25th/median/75th percentile; fewer is `low_confidence` (+-). A code with no price is an `unpriced` line, out of the total and listed in warnings; the person can enter a rate (`rate_overrides`, `source: user`). In-memory cache per (code, region, quality) for 7 days. Queries carry no tenant, project, user or plan data. See `docs/operations.md` for the environment variables.
- **Pricing** (`estimate.py`): lines times low/base/high rates, then site overhead 6.5%, profit 7.5%, design/supervision/permits 5.5%, contingency 7.5% (raised by up to 5 points with the share of assumed or low-confidence cost) and VAT 8%, each on a stated base; the defaults (the PRD's mid-range) are used only when the person agrees.
