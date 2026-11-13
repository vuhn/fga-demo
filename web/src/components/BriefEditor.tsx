import { ActionIcon, Button, Card, Group, MultiSelect, NumberInput, Select, SimpleGrid, Stack, Switch, Table, Text, TextInput, Tooltip } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { DEFAULT_FIXTURES, FIXTURE_KINDS, NESTABLE_TYPES, ROOM_TYPES, SIDES, type Brief, type BriefRoom, type FixtureKind, type RoomType } from "../api";

/** A blank brief for "fill the form" creation. The engine validates every field. */
export function emptyBrief(): Brief {
  return {
    title: "Single-storey house",
    lot: { width: 16, depth: 24, setbacks: { front: 3, back: 3, sides: 1.5 } },
    entrance_side: "south",
    rooms: [
      { id: "living", name: "Living", type: "living", target_area: 24, adjacent_to: [] },
      { id: "kitchen", name: "Kitchen", type: "kitchen", target_area: 10, adjacent_to: [] },
      { id: "bedroom_1", name: "Bedroom 1", type: "bedroom", target_area: 12, adjacent_to: [] },
      { id: "bathroom", name: "Bathroom", type: "bathroom", target_area: 5, adjacent_to: [] },
    ],
  };
}

/** Typical net areas (m²) used when a room is added in the form. */
const DEFAULT_AREA: Record<RoomType, number> = {
  entry: 5, living: 24, kitchen: 10, dining: 12, study: 9, laundry: 4, storage: 3, bedroom: 12, bathroom: 5,
};

const sideOptions = SIDES.map((s) => ({ value: s, label: s }));
const fixtureOptions = FIXTURE_KINDS.map((f) => ({ value: f, label: f.replace("_", " ") }));
const fixturesOf = (r: BriefRoom): FixtureKind[] => r.fixtures ?? DEFAULT_FIXTURES[r.type];
const sameList = (a: FixtureKind[], b: FixtureKind[]) => a.length === b.length && a.every((x, i) => x === b[i]);

function newRoomId(rooms: BriefRoom[], type: RoomType): string {
  const taken = new Set(rooms.map((r) => r.id));
  for (let n = 1; ; n++) {
    const id = `${type}_${n}`;
    if (!taken.has(id)) return id;
  }
}

/**
 * Form editor for a blueprint brief: lot, entrance, and a room table. The
 * brief is the only editable source of a blueprint; geometry is always
 * regenerated from it by the engine.
 */
export default function BriefEditor({ value, onChange, readOnly }: { value: Brief; onChange: (b: Brief) => void; readOnly?: boolean }) {
  const set = (patch: Partial<Brief>) => onChange({ ...value, ...patch });
  const setLot = (patch: Partial<Brief["lot"]>) => set({ lot: { ...value.lot, ...patch } });
  const setSetbacks = (patch: Partial<Brief["lot"]["setbacks"]>) => setLot({ setbacks: { ...value.lot.setbacks, ...patch } });
  const setRoom = (id: string, patch: Partial<BriefRoom>) => set({ rooms: value.rooms.map((r) => (r.id === id ? { ...r, ...patch } : r)) });
  const removeRoom = (id: string) =>
    set({
      rooms: value.rooms
        .filter((r) => r.id !== id)
        .map((r) => ({ ...r, adjacent_to: r.adjacent_to.filter((a) => a !== id), parent: r.parent === id ? null : r.parent })),
    });
  // A type change swaps untouched default fixtures for the new type's defaults and drops a nesting the new type cannot have.
  const setType = (r: BriefRoom, type: RoomType) =>
    setRoom(r.id, {
      type,
      fixtures: sameList(fixturesOf(r), DEFAULT_FIXTURES[r.type]) ? [...DEFAULT_FIXTURES[type]] : fixturesOf(r),
      parent: NESTABLE_TYPES.includes(type) ? r.parent : null,
    });
  // Rooms a nestable room can go inside: top-level rooms other than itself, holding fewer than two others.
  const hostOptions = (r: BriefRoom) =>
    value.rooms
      .filter((o) => o.id !== r.id && !o.parent && !value.rooms.some((c) => c.parent === r.id))
      .filter((o) => o.id === r.parent || value.rooms.filter((c) => c.parent === o.id).length < 2)
      .map((o) => ({ value: o.id, label: o.name }));
  const addRoom = () => {
    const type: RoomType = "bedroom";
    const id = newRoomId(value.rooms, type);
    set({ rooms: [...value.rooms, { id, name: `Bedroom ${value.rooms.filter((r) => r.type === type).length + 1}`, type, target_area: DEFAULT_AREA[type], adjacent_to: [], fixtures: [...DEFAULT_FIXTURES[type]] }] });
  };
  const num = (v: string | number, fallback: number) => (typeof v === "number" ? v : Number.parseFloat(v) || fallback);
  const total = value.rooms.reduce((s, r) => s + r.target_area, 0);

  return (
    <fieldset disabled={readOnly} style={{ border: 0, padding: 0, margin: 0, minWidth: 0 }}>
      <Stack gap="md">
        <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="sm">
          <TextInput label="Title" value={value.title} onChange={(e) => set({ title: e.currentTarget.value })} />
          <NumberInput label="Lot width (m, east-west)" min={4.1} max={200} decimalScale={1} value={value.lot.width} onChange={(v) => setLot({ width: num(v, value.lot.width) })} />
          <NumberInput label="Lot depth (m, north-south)" min={4.1} max={200} decimalScale={1} value={value.lot.depth} onChange={(v) => setLot({ depth: num(v, value.lot.depth) })} />
          <Select label="Entrance (street) side" allowDeselect={false} data={sideOptions} value={value.entrance_side} onChange={(v) => v && set({ entrance_side: v as Brief["entrance_side"] })} />
          <NumberInput label="Front setback (m)" min={0} max={50} decimalScale={1} value={value.lot.setbacks.front} onChange={(v) => setSetbacks({ front: num(v, 0) })} />
          <NumberInput label="Back setback (m)" min={0} max={50} decimalScale={1} value={value.lot.setbacks.back} onChange={(v) => setSetbacks({ back: num(v, 0) })} />
          <NumberInput label="Side setbacks (m)" min={0} max={50} decimalScale={1} value={value.lot.setbacks.sides} onChange={(v) => setSetbacks({ sides: num(v, 0) })} />
          <Switch label="Open plan" description="No walls between the hall, entry, living, kitchen and dining" checked={!!value.open_plan} onChange={(e) => set({ open_plan: e.currentTarget.checked })} />
        </SimpleGrid>

        <Card withBorder padding={0}>
          <Table.ScrollContainer minWidth={980}>
            <Table verticalSpacing={4} fz="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Room</Table.Th>
                  <Table.Th w={130}>Type</Table.Th>
                  <Table.Th w={100}>Area m²</Table.Th>
                  <Table.Th w={110}>Wall side</Table.Th>
                  <Table.Th>Next to</Table.Th>
                  <Table.Th w={150}>Inside</Table.Th>
                  <Table.Th>Fixtures</Table.Th>
                  <Table.Th w={40} />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {value.rooms.map((r) => (
                  <Table.Tr key={r.id}>
                    <Table.Td>
                      <TextInput size="xs" aria-label="Room name" value={r.name} onChange={(e) => setRoom(r.id, { name: e.currentTarget.value })} />
                    </Table.Td>
                    <Table.Td>
                      <Select
                        size="xs"
                        aria-label="Room type"
                        allowDeselect={false}
                        data={[...ROOM_TYPES]}
                        value={r.type}
                        onChange={(v) => v && setType(r, v as RoomType)}
                      />
                    </Table.Td>
                    <Table.Td>
                      <NumberInput size="xs" aria-label="Target area" min={1} max={200} decimalScale={1} value={r.target_area} onChange={(v) => setRoom(r.id, { target_area: num(v, r.target_area) })} />
                    </Table.Td>
                    <Table.Td>
                      <Select size="xs" aria-label="Wall side" placeholder={r.parent ? "parent's" : "any"} clearable disabled={!!r.parent} data={sideOptions} value={r.side ?? null} onChange={(v) => setRoom(r.id, { side: (v as BriefRoom["side"]) ?? null })} />
                    </Table.Td>
                    <Table.Td>
                      <MultiSelect
                        size="xs"
                        aria-label="Adjacent rooms"
                        data={value.rooms.filter((o) => o.id !== r.id).map((o) => ({ value: o.id, label: o.name }))}
                        value={r.adjacent_to}
                        onChange={(adjacent_to) => setRoom(r.id, { adjacent_to })}
                      />
                    </Table.Td>
                    <Table.Td>
                      {NESTABLE_TYPES.includes(r.type) ? (
                        <Select
                          size="xs"
                          aria-label="Inside room"
                          placeholder="off the hall"
                          clearable
                          data={hostOptions(r)}
                          value={r.parent ?? null}
                          onChange={(v) => setRoom(r.id, { parent: v, side: v ? null : r.side })}
                        />
                      ) : (
                        <Text size="xs" c="dimmed">-</Text>
                      )}
                    </Table.Td>
                    <Table.Td>
                      <MultiSelect
                        size="xs"
                        aria-label="Fixtures"
                        placeholder={fixturesOf(r).length ? undefined : "none"}
                        data={fixtureOptions}
                        maxValues={8}
                        value={fixturesOf(r)}
                        onChange={(v) => setRoom(r.id, { fixtures: v as FixtureKind[] })}
                      />
                    </Table.Td>
                    <Table.Td>
                      <Tooltip label="Remove room">
                        <ActionIcon variant="subtle" color="red" aria-label={`Remove ${r.name}`} disabled={readOnly || value.rooms.length <= 1} onClick={() => removeRoom(r.id)}>
                          <IconTrash size={14} />
                        </ActionIcon>
                      </Tooltip>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        </Card>
        <Group justify="space-between">
          <Text size="xs" c="dimmed">
            {value.rooms.length} rooms, {total.toFixed(1)} m² net. The layout adds the hall and walls. A bathroom, storage or laundry can sit inside another room (ensuite, walk-in closet); its only door opens into that room.
          </Text>
          {!readOnly && (
            <Button size="xs" variant="default" leftSection={<IconPlus size={14} />} onClick={addRoom} disabled={value.rooms.length >= 16}>
              Add room
            </Button>
          )}
        </Group>
      </Stack>
    </fieldset>
  );
}
