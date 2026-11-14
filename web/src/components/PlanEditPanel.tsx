import { ActionIcon, Button, Group, NumberInput, Select, Stack, Text, TextInput } from "@mantine/core";
import { IconArrowDown, IconArrowLeft, IconArrowRight, IconArrowUp, IconMinus, IconPlus, IconTrash } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { FIXTURE_KINDS, SIDES } from "../api";
import type { PlanModel, PlanOp } from "../blueprintModel";

const PLACED_TYPES = ["entry", "living", "kitchen", "dining", "study", "laundry", "storage", "bedroom", "bathroom", "corridor", "balcony", "other"];
const pretty = (s: string) => s.replace(/_/g, " ");

/**
 * Direct edits of the selected item of an imported plan. Each action is one request (one new version, so
 * Versions is the undo list); the engine applies it deterministically and refuses what does not fit
 * (a fixture leaving its room, a door off its wall). Nothing here decides anything: the API checks again.
 */
export default function PlanEditPanel({ model, selected, busy, onEdit }: {
  model: PlanModel;
  selected: string;
  busy: boolean;
  onEdit: (ops: PlanOp[]) => Promise<boolean>;
}) {
  const at = selected.indexOf(":");
  const kind = selected.slice(0, at);
  const id = selected.slice(at + 1);
  const [step, setStep] = useState<number>(0.1);
  const room = kind === "room" ? model.rooms.find((r) => r.id === id) : undefined;
  const fixture = kind === "fixture" ? model.fixtures.find((f) => f.id === id) : undefined;
  const opening = kind === "opening" ? model.openings.find((o) => o.id === id) : undefined;
  const [name, setName] = useState(room?.name ?? "");
  const [width, setWidth] = useState<number | string>(opening?.width ?? 0);
  const [add, setAdd] = useState<string | null>(null);
  useEffect(() => {
    setName(room?.name ?? "");
    setWidth(opening?.width ?? 0);
    setAdd(null);
  }, [room?.name, opening?.width, selected]);

  const go = (...ops: PlanOp[]) => void onEdit(ops);
  const nudge = (dx: number, dy: number) => fixture && go({ op: "move_fixture", fixture: fixture.id, dx: dx * step, dy: dy * step });
  const slide = (sign: number) => opening && go({ op: "move_opening", opening: opening.id, offset: sign * step });

  const stepInput = (
    <NumberInput size="xs" w={90} label="Step (m)" min={0.01} max={2} step={0.05} decimalScale={2} value={step} onChange={(v) => typeof v === "number" && setStep(v)} />
  );

  if (room) {
    return (
      <Stack gap={6}>
        <Group gap={6} align="flex-end" wrap="nowrap">
          <TextInput size="xs" label="Name" flex={1} maxLength={60} value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Button size="xs" disabled={busy || !name.trim() || name.trim() === room.name} onClick={() => go({ op: "rename_room", room: room.id, name: name.trim() })}>
            Rename
          </Button>
        </Group>
        <Select size="xs" label="Type" allowDeselect={false} disabled={busy} data={PLACED_TYPES} value={room.type}
          onChange={(v) => v && v !== room.type && go({ op: "set_room_type", room: room.id, type: v })} />
        <Group gap={6} align="flex-end" wrap="nowrap">
          <Select size="xs" label="Add a fixture" placeholder="Pick one" flex={1} searchable data={FIXTURE_KINDS.map((k) => ({ value: k, label: pretty(k) }))} value={add} onChange={setAdd} />
          <Button size="xs" leftSection={<IconPlus size={12} />} disabled={busy || !add} onClick={() => add && go({ op: "add_fixture", room: room.id, fixture: add })}>
            Add
          </Button>
        </Group>
        <Text size="xs" c="dimmed">Walls and room shapes cannot be moved yet.</Text>
      </Stack>
    );
  }

  if (fixture) {
    return (
      <Stack gap={6}>
        <Group gap="md" align="flex-end">
          {stepInput}
          <Group gap={2}>
            <ActionIcon variant="default" disabled={busy} aria-label="Move west" onClick={() => nudge(-1, 0)}><IconArrowLeft size={14} /></ActionIcon>
            <ActionIcon variant="default" disabled={busy} aria-label="Move north" onClick={() => nudge(0, 1)}><IconArrowUp size={14} /></ActionIcon>
            <ActionIcon variant="default" disabled={busy} aria-label="Move south" onClick={() => nudge(0, -1)}><IconArrowDown size={14} /></ActionIcon>
            <ActionIcon variant="default" disabled={busy} aria-label="Move east" onClick={() => nudge(1, 0)}><IconArrowRight size={14} /></ActionIcon>
          </Group>
        </Group>
        <Group gap={6} align="flex-end" wrap="nowrap">
          <Select size="xs" label="Faces" flex={1} allowDeselect={false} disabled={busy} data={[...SIDES]} value={fixture.facing}
            onChange={(v) => v && v !== fixture.facing && go({ op: "rotate_fixture", fixture: fixture.id, facing: v })} />
          <Button size="xs" color="red" variant="light" leftSection={<IconTrash size={12} />} disabled={busy} onClick={() => go({ op: "delete_fixture", fixture: fixture.id })}>
            Delete
          </Button>
        </Group>
      </Stack>
    );
  }

  if (opening) {
    return (
      <Stack gap={6}>
        <Group gap={6} align="flex-end" wrap="nowrap">
          <NumberInput size="xs" label="Width (m)" w={110} min={0.5} max={4} step={0.05} decimalScale={2} value={width} onChange={setWidth} />
          <Button size="xs" disabled={busy || typeof width !== "number" || Math.abs(width - opening.width) < 0.005}
            onClick={() => typeof width === "number" && go({ op: "resize_opening", opening: opening.id, width })}>
            Set width
          </Button>
        </Group>
        <Group gap="md" align="flex-end">
          {stepInput}
          <Group gap={2}>
            <ActionIcon variant="default" disabled={busy} aria-label="Slide towards the start of the wall" onClick={() => slide(-1)}><IconMinus size={14} /></ActionIcon>
            <ActionIcon variant="default" disabled={busy} aria-label="Slide towards the end of the wall" onClick={() => slide(1)}><IconPlus size={14} /></ActionIcon>
          </Group>
          <Button size="xs" color="red" variant="light" leftSection={<IconTrash size={12} />} disabled={busy || opening.kind === "entrance"} onClick={() => go({ op: "remove_opening", opening: opening.id })}>
            Remove
          </Button>
        </Group>
      </Stack>
    );
  }

  return (
    <Text size="xs" c="dimmed">
      Walls cannot be moved or resized yet.
    </Text>
  );
}
