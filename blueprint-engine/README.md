# blueprint-engine

This is the stateless engine for the Blueprints product. It turns a JSON **brief** into a floor plan and renders that plan to DXF, SVG and PDF, and exports it as an IFC4 BIM model.

- It has no auth and no storage. Only the Go API calls it, after the Go API has run its own OpenFGA Checks.
- The design and the review of the original proposal are in `../prds/plans/text2blueprint-poc.md`.

## The rule

**The LLM only produces or edits the brief. Code does every piece of geometry.**

| Step | Who | Module |
|---|---|---|
| Prompt (→ requirements, with the opt-in Enhancer) → brief | LLM (Ollama, JSON-schema output) + clean-up in code | `llm_steps.py`, `prompts.py` |
| Sketch → brief | vision LLM + clean-up in code | `llm_steps.py` |
| Chat message (+ earlier messages) → edit ops → brief | LLM + `apply_ops` (room names fuzzy-matched in code) | `llm_steps.py`, `edits.py` |
| Failed checks → edit ops (Reviewer) | LLM, kept only when the issue score drops | `llm_steps.review` |
| Brief too big → shrink toward minimums | code | `pipeline.generate_fitted` |
| Brief → zoned slicing layout (front band, hall, back band) | code, deterministic, stable vs. previous version | `layout.py` |
| Layout → walls, doors, windows, net areas | code | `building.py` |
| Fixtures (bed, toilet, kitchen run…) placed in each room | code, deterministic | `fixtures.py` |
| Rule checks | code (Shapely) | `checks.py` |
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
ollama pull qwen2.5vl:7b        # vision model, only for sketch upload
make engine                     # http://localhost:8090 serves a dev harness page
make engine-test                # pytest (no LLM needed)
make engine-eval                # golden prompts against the live model
```

Environment variables:

| Variable | Default |
|---|---|
| `OLLAMA_URL` | `http://localhost:11434` |
| `BLUEPRINT_TEXT_MODEL` | `qwen3:latest` (the 8B; `qwen3:14b` with 32 GB) |
| `BLUEPRINT_VISION_MODEL` | `qwen2.5vl:7b` |

Ollama runs natively, not in Docker, because Docker on macOS has no GPU access.

## HTTP

| Route | Body | Returns |
|---|---|---|
| `POST /v1/brief/from-text` | `{prompt, enhance?: true}` | `{brief, notes}` |
| `POST /v1/brief/from-sketch` | multipart `file` (PNG, JPG, PDF or DXF, ≤10 MB; PDF first page only; DXF read from text labels, no vision model), `note` | `{brief, notes}` |
| `POST /v1/generate` | `{brief, previous?, repair?}` | `{result: {brief, model, checks, issues, ok}, notes, attempts, llm_attempts, review_rounds}` |
| `POST /v1/revise` | `{brief, previous?, message, history?}` | the same as generate, plus `ops` |
| `POST /v1/revise/stream` | the same as revise | NDJSON, one JSON object per line: `{"event":"progress","kind","stage","message"}` while it works (`kind` is `step`, or `thinking` with a piece of the model's reasoning as `message`; while streaming the LLM call runs with `think: true` and the reasoning is forwarded in chunks of about 150 ms, `BLUEPRINT_STREAM_THINKING=0` turns that off), then exactly one `{"event":"result","data":<revise response>}` or `{"event":"error","status","error","message"}`. A bad request (empty message, malformed body) is the usual 4xx before any line. Stages: `start`, `ask_model`, `retry`, `ops`, `layout`, `checks`, `review`. Closing the connection cancels the run at the next step. Progress lines are never stored; they carry no tenant or user ids. |
| `POST /v1/review` | `{brief, previous?, history?, max_rounds?: 1-3}` | the same as generate |
| `POST /v1/render` | `{model, format: svg\|dxf\|pdf\|ifc, title_block}` | the file bytes |

Errors:

- An edit that still cannot be applied after 3 attempts, or an unsupported request, returns `422 {"error":"invalid_edit","message"}`.
- An LLM that is down or returns bad output returns `502 {"error":"llm_unavailable"}`.
