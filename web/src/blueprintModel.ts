// The engine's BuildingModel as the viewer reads it (metres, y up). Only what the viewer draws.
export type Pt = [number, number];
export type ModelRoom = {
  id: string;
  name: string;
  type: string;
  net_rect: [number, number, number, number];
  net_area: number;
  polygon?: Pt[] | null;
  exterior_sides?: string[];
};
export type ModelWall = { id: string; start: Pt; end: Pt; thickness: number; kind: "exterior" | "interior" };
export type ModelOpening = {
  id: string;
  kind: "door" | "window" | "entrance" | "open";
  wall_id: string;
  center: Pt;
  width: number;
  rooms: string[];
  swing_into?: string | null;
};
/** Direct edits of an imported plan (the engine's plan_edits ops). Rooms and openings are named by id. */
export type PlanOp =
  | { op: "rename_room"; room: string; name: string }
  | { op: "set_room_type"; room: string; type: string }
  | { op: "add_fixture"; room: string; fixture: string }
  | { op: "delete_fixture"; fixture: string }
  | { op: "move_fixture"; fixture: string; dx: number; dy: number }
  | { op: "rotate_fixture"; fixture: string; facing: string }
  | { op: "resize_opening"; opening: string; width: number }
  | { op: "move_opening"; opening: string; offset: number }
  | { op: "remove_opening"; opening: string };

export type ModelFixture = { id: string; kind: string; room_id: string; rect: [number, number, number, number]; facing: string };
/** One CAD entity of an imported file kept as drawn (read-only): curves are already flattened to polylines. */
export type ModelDrawItem = {
  id: string;
  type: string; // DXF entity type: LINE, CIRCLE, SPLINE, HATCH, TEXT, POINT, ...
  layer: string;
  color: string; // "#rrggbb" or "" for the default ink
  paths: Pt[][];
  filled: boolean;
  at: Pt | null;
  text: string;
  height: number;
  rotation: number;
};
export type PlanModel = {
  footprint: [number, number, number, number];
  outline: Pt[];
  source: "generated" | "imported";
  rooms: ModelRoom[];
  walls: ModelWall[];
  openings: ModelOpening[];
  fixtures: ModelFixture[];
  drawing: ModelDrawItem[];
};

export function asPlanModel(m: unknown): PlanModel | null {
  const p = m as Partial<PlanModel> | null;
  return p && Array.isArray(p.rooms) && Array.isArray(p.walls) && Array.isArray(p.openings) && p.footprint
    ? ({ fixtures: [], drawing: [], outline: [], source: "generated", ...p } as PlanModel)
    : null;
}
