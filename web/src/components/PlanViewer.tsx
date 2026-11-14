import { ActionIcon, Badge, Box, Button, Checkbox, Divider, Group, Paper, ScrollArea, SimpleGrid, Stack, Text } from "@mantine/core";
import { IconEye, IconEyeOff, IconFocusCentered } from "@tabler/icons-react";
import { useMemo, useRef, useState, type PointerEvent, type WheelEvent } from "react";
import type { ModelOpening, ModelRoom, PlanModel, Pt } from "../blueprintModel";

/**
 * Read-only plan viewer: the model drawn as SVG (pan, zoom), layer toggles, a browser of every item with
 * show/hide, and click-to-select with a properties panel. Everything here is client state; nothing is sent
 * to the API. Items are keyed "kind:id".
 */
const LAYERS = [
  { key: "rooms", label: "Rooms" },
  { key: "walls", label: "Walls" },
  { key: "doors", label: "Doors" },
  { key: "windows", label: "Windows" },
  { key: "fixtures", label: "Fixtures" },
  { key: "labels", label: "Labels" },
] as const;
type Layer = (typeof LAYERS)[number]["key"];

const FILL: Record<string, string> = {
  bedroom: "#dbeafe", bathroom: "#cffafe", kitchen: "#fef3c7", living: "#dcfce7", dining: "#ecfccb", study: "#ede9fe",
  balcony: "#e0f2fe", entry: "#fce7f3", corridor: "#f1f5f9", storage: "#f5f5f4", laundry: "#fae8ff",
};
const SELECT = "#e03131";
const pretty = (s: string) => s.replace(/_/g, " ");

type View = { x: number; y: number; w: number; h: number };
const key = (kind: string, id: string) => `${kind}:${id}`;

function flip(p: Pt, top: number): Pt {
  return [p[0], top - p[1]]; // the model is y-up, SVG is y-down
}

/** Door leaf and swing arc, as the drawing sheet draws them: hinge on the room-side face, leaf square to the
 * wall, a quarter circle of the leaf's length back to the other jamb. Returns SVG path pieces in plan coordinates. */
function doorSwing(o: ModelOpening, ang: number, thick: number, rooms: Map<string, ModelRoom>, top: number) {
  const ux = Math.cos(ang), uy = Math.sin(ang);
  let nx = -uy, ny = ux;
  const target = o.swing_into ? rooms.get(o.swing_into) : undefined;
  if (target) {
    const rx = (target.net_rect[0] + target.net_rect[2]) / 2, ry = (target.net_rect[1] + target.net_rect[3]) / 2;
    if ((rx - o.center[0]) * nx + (ry - o.center[1]) * ny < 0) [nx, ny] = [-nx, -ny];
  }
  const h = o.width / 2, t = thick / 2;
  const hinge: Pt = [o.center[0] - ux * h + nx * t, o.center[1] - uy * h + ny * t];
  const leaf: Pt = [hinge[0] + nx * o.width, hinge[1] + ny * o.width];
  const jamb: Pt = [hinge[0] + ux * o.width, hinge[1] + uy * o.width];
  const [hx, hy] = flip(hinge, top), [lx, ly] = flip(leaf, top), [jx, jy] = flip(jamb, top);
  const cross = (lx - hx) * (jy - hy) - (ly - hy) * (jx - hx); // sign on screen (y down) picks the arc direction
  return { leaf: `M${hx} ${hy} L${lx} ${ly}`, arc: `M${lx} ${ly} A${o.width} ${o.width} 0 0 ${cross > 0 ? 1 : 0} ${jx} ${jy}` };
}

function openingRect(o: ModelOpening, ang: number, thick: number, top: number) {
  const [cx, cy] = flip(o.center, top);
  const deg = -(ang * 180) / Math.PI;
  return { transform: `translate(${cx} ${cy}) rotate(${deg})`, w: o.width, h: thick };
}

export default function PlanViewer({ model }: { model: PlanModel }) {
  const [x0, y0, x1, y1] = model.footprint;
  const top = y1 + y0; // flip about the footprint's middle so the plan keeps its place
  const pad = 1;
  const fit = useMemo<View>(() => ({ x: x0 - pad, y: top - y1 - pad, w: x1 - x0 + 2 * pad, h: y1 - y0 + 2 * pad }), [x0, y0, x1, y1, top]);
  const [view, setView] = useState<View>(fit);
  const [layers, setLayers] = useState<Record<Layer, boolean>>({ rooms: true, walls: true, doors: true, windows: true, fixtures: true, labels: true });
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<string | null>(null);
  const drag = useRef<{ x: number; y: number; moved: boolean } | null>(null);
  const svg = useRef<SVGSVGElement>(null);

  const walls = useMemo(() => new Map(model.walls.map((w) => [w.id, w])), [model.walls]);
  const rooms = useMemo(() => new Map(model.rooms.map((r) => [r.id, r])), [model.rooms]);
  const wallAngle = (id: string) => {
    const w = walls.get(id);
    return w ? Math.atan2(w.end[1] - w.start[1], w.end[0] - w.start[0]) : 0;
  };
  const show = (kind: string, id: string) => !hidden.has(key(kind, id));
  const toggle = (k: string) =>
    setHidden((h) => {
      const n = new Set(h);
      if (n.has(k)) n.delete(k);
      else n.add(k);
      return n;
    });

  function zoom(e: WheelEvent) {
    const r = svg.current?.getBoundingClientRect();
    if (!r) return;
    const f = e.deltaY > 0 ? 1.15 : 1 / 1.15;
    const px = view.x + ((e.clientX - r.left) / r.width) * view.w;
    const py = view.y + ((e.clientY - r.top) / r.height) * view.h;
    setView({ x: px - (px - view.x) * f, y: py - (py - view.y) * f, w: view.w * f, h: view.h * f });
  }
  function down(e: PointerEvent) {
    drag.current = { x: e.clientX, y: e.clientY, moved: false };
  }
  function move(e: PointerEvent) {
    const d = drag.current;
    const r = svg.current?.getBoundingClientRect();
    if (!d || !r) return;
    const dx = e.clientX - d.x, dy = e.clientY - d.y;
    if (!d.moved && Math.abs(dx) + Math.abs(dy) > 3) {
      d.moved = true;
      svg.current?.setPointerCapture(e.pointerId); // capture only once it is a drag, so plain clicks still reach items
    }
    if (!d.moved) return;
    setView((v) => ({ ...v, x: v.x - (dx / r.width) * v.w, y: v.y - (dy / r.height) * v.h }));
    d.x = e.clientX;
    d.y = e.clientY;
  }
  function up(e: PointerEvent) {
    if (drag.current && !drag.current.moved && e.target === svg.current) setSelected(null); // a click on empty paper clears the selection
    drag.current = null;
  }

  const sel = (k: string) => (selected === k ? { stroke: SELECT, strokeWidth: 3 } : {});
  const items = [
    ...model.rooms.map((r) => ({ k: key("room", r.id), label: r.name, group: "Rooms" })),
    ...model.openings.map((o) => ({ k: key("opening", o.id), label: `${o.id} · ${o.kind}`, group: "Doors and windows" })),
    ...model.fixtures.map((f) => ({ k: key("fixture", f.id), label: `${pretty(f.kind)} · ${rooms.get(f.room_id)?.name ?? f.room_id}`, group: "Fixtures" })),
  ];
  const groups = [...new Set(items.map((i) => i.group))];

  return (
    <SimpleGrid cols={{ base: 1, md: 3 }} spacing="xs">
      <Box style={{ gridColumn: "span 2" }}>
        <Paper bg="white" radius="sm" withBorder style={{ overflow: "hidden", touchAction: "none", position: "relative" }}>
          <svg
            ref={svg}
            viewBox={`${view.x} ${view.y} ${view.w} ${view.h}`}
            style={{ display: "block", width: "100%", height: 580, cursor: "grab", background: "white" }}
            onWheel={zoom}
            onPointerDown={down}
            onPointerMove={move}
            onPointerUp={up}
            role="img"
            aria-label="Floor plan viewer"
          >
            {layers.rooms &&
              model.rooms.filter((r) => show("room", r.id)).map((r) => {
                const k = key("room", r.id);
                const pts = (r.polygon && r.polygon.length ? r.polygon : [[r.net_rect[0], r.net_rect[1]], [r.net_rect[2], r.net_rect[1]], [r.net_rect[2], r.net_rect[3]], [r.net_rect[0], r.net_rect[3]]]) as Pt[];
                return (
                  <polygon key={k} points={pts.map((p) => flip(p, top).join(",")).join(" ")} fill={FILL[r.type] ?? "#f8fafc"}
                    stroke="#94a3b8" strokeWidth={1} vectorEffect="non-scaling-stroke" style={{ cursor: "pointer" }}
                    {...sel(k)} onClick={() => setSelected(k)} />
                );
              })}
            {layers.walls &&
              model.walls.filter((w) => show("wall", w.id)).map((w) => {
                const [ax, ay] = flip(w.start, top), [bx, by] = flip(w.end, top);
                const k = key("wall", w.id);
                return (
                  <line key={k} x1={ax} y1={ay} x2={bx} y2={by} stroke={selected === k ? SELECT : "#1f2937"} strokeWidth={w.thickness}
                    strokeLinecap="square" style={{ cursor: "pointer" }} onClick={() => setSelected(k)} />
                );
              })}
            {model.openings
              .filter((o) => show("opening", o.id) && (o.kind === "window" ? layers.windows : layers.doors))
              .map((o) => {
                const w = walls.get(o.wall_id);
                const t = openingRect(o, wallAngle(o.wall_id), (w?.thickness ?? 0.2) * 1.02, top);
                const k = key("opening", o.id);
                const isWin = o.kind === "window";
                const swing = isWin || o.kind === "open" ? null : doorSwing(o, wallAngle(o.wall_id), w?.thickness ?? 0.2, rooms, top);
                const ink = selected === k ? SELECT : "#374151";
                return (
                  <g key={k} style={{ cursor: "pointer" }} onClick={() => setSelected(k)}>
                    <g transform={t.transform}>
                      <rect x={-t.w / 2} y={-t.h / 2} width={t.w} height={t.h} fill={isWin ? "#bfdbfe" : "#ffffff"}
                        stroke={selected === k ? SELECT : isWin ? "#2563eb" : "#6b7280"} strokeWidth={selected === k ? 3 : 1.5} vectorEffect="non-scaling-stroke" />
                      {isWin && <line x1={-t.w / 2} x2={t.w / 2} y1={0} y2={0} stroke="#2563eb" strokeWidth={1} vectorEffect="non-scaling-stroke" />}
                    </g>
                    {swing && (
                      <>
                        <path d={swing.arc} fill="none" stroke={ink} strokeWidth={1} strokeDasharray="4 3" vectorEffect="non-scaling-stroke" />
                        <path d={swing.leaf} fill="none" stroke={ink} strokeWidth={selected === k ? 3 : 1.5} vectorEffect="non-scaling-stroke" />
                      </>
                    )}
                  </g>
                );
              })}
            {layers.fixtures &&
              model.fixtures.filter((f) => show("fixture", f.id)).map((f) => {
                const [a, b, c, d] = f.rect;
                const k = key("fixture", f.id);
                return (
                  <rect key={k} x={a} y={top - d} width={c - a} height={d - b} fill="#fde68a" fillOpacity={0.7}
                    stroke="#92400e" strokeWidth={1} vectorEffect="non-scaling-stroke" style={{ cursor: "pointer" }}
                    {...sel(k)} onClick={() => setSelected(k)} />
                );
              })}
            {layers.labels &&
              model.rooms.filter((r) => show("room", r.id)).map((r) => {
                const [a, b, c, d] = r.net_rect;
                return (
                  <text key={`l${r.id}`} x={(a + c) / 2} y={top - (b + d) / 2} fontSize={0.28} textAnchor="middle" fill="#111827" pointerEvents="none">
                    {r.name}
                  </text>
                );
              })}
          </svg>
          <ActionIcon variant="default" size="sm" style={{ position: "absolute", top: 8, right: 8 }} aria-label="Fit to view" onClick={() => setView(fit)}>
            <IconFocusCentered size={14} />
          </ActionIcon>
        </Paper>
        <Details model={model} selected={selected} rooms={rooms} walls={walls} hidden={hidden} onToggle={toggle} />
      </Box>

      <Stack gap="xs">
        <Paper withBorder p="xs">
          <Text size="xs" fw={600} mb={4}>
            Layers
          </Text>
          <SimpleGrid cols={2} spacing={4}>
            {LAYERS.map((l) => (
              <Checkbox key={l.key} size="xs" label={l.label} checked={layers[l.key]} onChange={(e) => setLayers({ ...layers, [l.key]: e.currentTarget.checked })} />
            ))}
          </SimpleGrid>
        </Paper>
        <Paper withBorder p="xs">
          <Group justify="space-between" mb={4}>
            <Text size="xs" fw={600}>
              Items
            </Text>
            <Button size="compact-xs" variant="subtle" disabled={hidden.size === 0} onClick={() => setHidden(new Set())}>
              Show all
            </Button>
          </Group>
          <ScrollArea.Autosize mah={300}>
            <Stack gap={2}>
              {groups.map((g) => (
                <Box key={g}>
                  <Divider label={g} labelPosition="left" my={4} />
                  {items.filter((i) => i.group === g).map((i) => (
                    <Group key={i.k} gap={4} wrap="nowrap" bg={selected === i.k ? "var(--mantine-color-red-light)" : undefined} style={{ borderRadius: 4 }}>
                      <ActionIcon size="xs" variant="subtle" color={hidden.has(i.k) ? "gray" : "blue"} aria-label={hidden.has(i.k) ? "Show" : "Hide"} onClick={() => toggle(i.k)}>
                        {hidden.has(i.k) ? <IconEyeOff size={12} /> : <IconEye size={12} />}
                      </ActionIcon>
                      <Text size="xs" truncate style={{ cursor: "pointer", flex: 1, opacity: hidden.has(i.k) ? 0.45 : 1 }} onClick={() => setSelected(i.k)}>
                        {i.label}
                      </Text>
                    </Group>
                  ))}
                </Box>
              ))}
            </Stack>
          </ScrollArea.Autosize>
        </Paper>
      </Stack>
    </SimpleGrid>
  );
}

function Details({ model, selected, rooms, walls, hidden, onToggle }: {
  model: PlanModel;
  selected: string | null;
  rooms: Map<string, PlanModel["rooms"][number]>;
  walls: Map<string, PlanModel["walls"][number]>;
  hidden: Set<string>;
  onToggle: (k: string) => void;
}) {
  if (!selected) {
    return (
      <Text size="xs" c="dimmed" mt="xs">
        Click an item to see its properties. Scroll to zoom, drag to pan.
      </Text>
    );
  }
  const [kind, id] = [selected.slice(0, selected.indexOf(":")), selected.slice(selected.indexOf(":") + 1)];
  const name = (rid: string) => (rid === "outside" ? "outside" : rooms.get(rid)?.name ?? rid);
  let title = id;
  let rows: [string, string][] = [];
  if (kind === "room") {
    const r = rooms.get(id);
    if (!r) return null;
    title = r.name;
    const [a, b, c, d] = r.net_rect;
    rows = [["Type", r.type], ["Net area", `${r.net_area.toFixed(1)} m²`], ["Bounding box", `${(c - a).toFixed(2)} × ${(d - b).toFixed(2)} m`],
      ["Fixtures", String(model.fixtures.filter((f) => f.room_id === id).length)], ["Outside walls", (r.exterior_sides ?? []).join(", ") || "none"]];
  } else if (kind === "wall") {
    const w = walls.get(id);
    if (!w) return null;
    title = `Wall ${w.id}`;
    rows = [["Kind", w.kind], ["Length", `${Math.hypot(w.end[0] - w.start[0], w.end[1] - w.start[1]).toFixed(2)} m`], ["Thickness", `${(w.thickness * 1000).toFixed(0)} mm`]];
  } else if (kind === "opening") {
    const o = model.openings.find((x) => x.id === id);
    if (!o) return null;
    title = `${o.kind} ${o.id}`;
    rows = [["Width", `${o.width.toFixed(2)} m`], ["Wall", o.wall_id], ["Connects", o.rooms.map(name).join(" ↔ ")]];
    if (o.swing_into) rows.push(["Swings into", name(o.swing_into)]);
  } else if (kind === "fixture") {
    const f = model.fixtures.find((x) => x.id === id);
    if (!f) return null;
    const [a, b, c, d] = f.rect;
    title = pretty(f.kind);
    rows = [["Room", name(f.room_id)], ["Size", `${(c - a).toFixed(2)} × ${(d - b).toFixed(2)} m`], ["Faces", f.facing]];
  }
  return (
    <Paper withBorder p="xs" mt="xs">
      <Group justify="space-between" mb={4}>
        <Group gap={6}>
          <Badge size="xs" variant="light">{kind}</Badge>
          <Text size="sm" fw={600}>{title}</Text>
        </Group>
        <Button size="compact-xs" variant="subtle" onClick={() => onToggle(selected)}>
          {hidden.has(selected) ? "Show" : "Hide"}
        </Button>
      </Group>
      <SimpleGrid cols={2} spacing={2}>
        {rows.map(([k, v]) => (
          <Group key={k} gap={6} wrap="nowrap">
            <Text size="xs" c="dimmed">{k}</Text>
            <Text size="xs">{v}</Text>
          </Group>
        ))}
      </SimpleGrid>
    </Paper>
  );
}
