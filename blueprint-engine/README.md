# blueprint-engine

This is the stateless engine for the Blueprints product. It turns a JSON **brief** into a floor plan and renders that plan to DXF, SVG and PDF, and exports it as an IFC4 BIM model.

- It has no auth and no storage. The Go API (deterministic operations) and the Agent (its tools) call it over gRPC, after the Go API has run its own OpenFGA Checks. It has no LLM, no web access and no HTTP server.
- The design and the review of the original proposal are in `../prds/plans/text2blueprint-poc.md`.

## The rule

**The LLM only produces or edits the brief. Code does every piece of geometry.**

| Step | Who | Module |
|---|---|---|
| Prompt → brief draft | the Agent's LLM; the Engine cleans the draft up in code (`NormalizeDraft`) | `domain/draft.py`, `service/draft_normalizer.py` |
| PDF / PNG / JPEG → DXF | no LLM: pdfium paths (vector PDF) or OpenCV tracing (picture), then `infrastructure/dxf_plan.py` reads it | `infrastructure/convert.py` |
| Edit ops (from the Agent's chat) → brief | `apply_ops` (room names fuzzy-matched in code) | `domain/edits.py`, `service/brief_ops_applier.py` |
| Brief too big → shrink toward minimums | code | `pipeline.generate_fitted` |
| Brief → zoned slicing layout (front band, hall, back band) | code, deterministic, stable vs. previous version | `domain/layout.py` |
| Layout → walls, doors, windows, net areas | code | `domain/building.py` |
| Fixtures (bed, toilet, kitchen run…) placed in each room | code, deterministic | `domain/fixtures.py` |
| Rule checks | code (Shapely) | `domain/checks.py` |
| Brief and plan operations (generate, create plan, draw plan, apply plan ops, run checks) | usecase Protocols in `usecase/`, one service per op in `service/`, graph built only in `wire.py`; the gRPC handlers in `handler/grpc/` call them; DXF drawing/reading sits behind `domain/gateway` (`infrastructure/plan_drawing.py`) | `usecase/`, `service/`, `wire.py` |
| DXF, SVG, PDF from one ezdxf document | code | `infrastructure/draw.py` |
| IFC4 BIM model (ifcopenshell) | code | `infrastructure/ifc.py` |

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

### IFC export

`format: "ifc"` writes an IFC4 file: project, site, building and one storey; an `IfcWall` per wall, extruded 2.7 m about its centerline; an `IfcOpeningElement` per door, window and entrance, filled by an `IfcDoor` (2.1 m) or `IfcWindow` (sill 0.9 m, 1.2 m high); an `IfcSpace` per room with `Qto_SpaceBaseQuantities.NetFloorArea`; a 0.2 m floor `IfcSlab` under the footprint; and each fixture as `IfcFurniture`, `IfcSanitaryTerminal` or `IfcElectricAppliance`, contained in its room's space. It opens in any IFC viewer (BIMvision, the IfcOpenShell/Bonsai viewer, web-ifc).

Conventions:

- Units are metres and the origin is the lot's bottom-left corner. +y is north.
- Dimensions are measured to wall centerlines.
- Exterior walls are 0.30 m and interior walls are 0.12 m.
- Sheets are drawn at 1:50 on the smallest landscape ISO sheet that fits the house and its dimensions, from A3 up to A0. Only a house too big for A0 drops to 1:100. The title block states the scale and the paper.

## Run

```bash
make engine                     # gRPC on :8092 (GRPC_ADDR, GRPC_WORKERS), reloads on source changes; health: grpc.health.v1
make engine-test                # pytest (no LLM, no network)
```

The contract is `../proto/fga/demo/engine/v1/engine.proto` (summary in `../docs/service-contracts.md`): ListTools, GetCatalog, NormalizeDraft, Generate, ApplyBriefOps, CreatePlan, DrawPlan, ApplyPlanOps, RunChecks, SummarizePlan, Takeoff, PriceEstimate, ImportDXF, ConvertToDXF, PatchDXF, Render. A refusal is `INVALID_ARGUMENT` with an `ErrorDetail` in the `error-detail-bin` trailer; messages up to 32 MB are accepted (a DXF is at most 20 MB).

## DXF import (`infrastructure/dxf_plan.py`)

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

## Estimate tools (`service/takeoff.py`, `service/price_estimate.py`, `service/plan_summarizer.py`)

The cost-estimation chat lives in the Agent (`../agent`). The Engine only measures and prices, deterministically, from the facts the Agent sends (`EstimateFacts`: value and source per fact, room scope, `fixtures_existing`, `excluded`):

- `SummarizePlan` lists rooms, fixtures, openings and the fixtures still to ask about.
- `Takeoff` turns the plan and the facts into quantity lines (areas, wall and ceiling surfaces, openings, fixtures, structure and MEP when in scope); a line that rests on an assumed fact is flagged assumed.
- `PriceEstimate` multiplies lines by low/base/high rates the Agent researched, then adds site overhead, profit, design and supervision, contingency (raised with the share of assumed or low-confidence cost) and VAT, each on a stated base, and cascades to the dong. A drawing with no rooms is unsupported, not zero. Lines with no rate stay out of the total and are listed.
- `GetCatalog` hands the Agent the rate catalog (code, description, unit, material, `quality_sensitive`), the priced fixtures and the minimum areas.
