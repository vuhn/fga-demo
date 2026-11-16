import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Center,
  EmptyState,
  FileInput,
  Group,
  List,
  Loader,
  Menu,
  Modal,
  NumberInput,
  Radio,
  SegmentedControl,
  Select,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { IconDots, IconForms, IconPencil, IconPlus, IconRuler2, IconShare, IconTrash, IconUpload, IconWand } from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api, apiUpload, IMPORT_UNITS, type Blueprint, type BlueprintDetail, type BlueprintDraft, type Brief, type ImportPreview, type Project } from "../api";
import { useAuth } from "../auth";
import BriefEditor, { emptyBrief } from "../components/BriefEditor";
import ShareDialog from "../components/ShareDialog";
import { canAct, canProject } from "../permissions";
import { confirmDanger, notifyError, run } from "../ui";

const SOURCE_LABEL: Record<Blueprint["source_kind"], string> = { prompt: "prompt", sketch: "sketch", form: "form", import: "uploaded plan" };

/**
 * Blueprints tab of a project. Blueprints sit directly in the project (no
 * folders, no visibility), so this is its own page rather than ContentPage.
 */
export default function Blueprints({ project }: { project: Project }) {
  const { me, tenantId } = useAuth();
  const navigate = useNavigate();
  const [rows, setRows] = useState<Blueprint[] | null>(null);
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState<Blueprint | null>(null);
  const [sharing, setSharing] = useState<Blueprint | null>(null);

  const load = useCallback(async () => {
    if (!me || !tenantId) return;
    try {
      setRows(await api<Blueprint[]>("GET", `/blueprints?project_id=${project.id}`));
    } catch (e) {
      notifyError(e);
      setRows([]);
    }
  }, [me, tenantId, project.id]);

  useEffect(() => {
    setRows(null);
    void load();
  }, [load]);

  const open = (b: Blueprint) => navigate(`/projects/${project.id}/blueprints/${b.id}`);
  const remove = (b: Blueprint) =>
    confirmDanger(`Delete blueprint “${b.name}”?`, "Every version, its tuples, and its grants are deleted. This cannot be undone.", "Delete", () => {
      void run(() => api("DELETE", `/blueprints/${b.id}`), `Deleted ${b.name}`).then((ok) => {
        if (ok) void load();
      });
    });

  if (rows === null) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }

  return (
    <>
      <Group justify="space-between" mb="md">
        <Text size="sm" c="dimmed">
          Upload a floor plan (DXF, PDF, PNG or JPG) and it is kept as a DXF you can view and edit. Plans can also be drafted by a local AI from a description.
        </Text>
        {canProject(project, "createBlueprint") && (
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>
            New blueprint
          </Button>
        )}
      </Group>

      {rows.length === 0 ? (
        <EmptyState mt="xl" icon={<IconRuler2 size={24} />} title="No blueprints" description="Nothing here is visible with the current relationships." />
      ) : (
        <Card withBorder padding={0}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Version</Table.Th>
                <Table.Th>Source</Table.Th>
                <Table.Th>Updated</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((b) => {
                const actions = [
                  canAct(b, "blueprint", "rename") && (
                    <Menu.Item key="rename" leftSection={<IconPencil size={16} />} onClick={() => setRenaming(b)}>
                      Rename
                    </Menu.Item>
                  ),
                  canAct(b, "blueprint", "share") && (
                    <Menu.Item key="share" leftSection={<IconShare size={16} />} onClick={() => setSharing(b)}>
                      Share
                    </Menu.Item>
                  ),
                  canAct(b, "blueprint", "delete") && (
                    <Menu.Item key="delete" color="red" leftSection={<IconTrash size={16} />} onClick={() => remove(b)}>
                      Delete
                    </Menu.Item>
                  ),
                ].filter(Boolean);
                return (
                  <Table.Tr key={b.id} style={{ cursor: "pointer" }} onClick={() => open(b)}>
                    <Table.Td>
                      <Group gap="xs">
                        <IconRuler2 size={16} />
                        <Text size="sm" fw={500}>
                          {b.name}
                        </Text>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Badge variant="light">v{b.current_version}</Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">
                        {SOURCE_LABEL[b.source_kind]}
                        {b.source ? ` · ${b.source.name}` : ""}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">
                        {new Date(b.updated_at).toLocaleString()}
                      </Text>
                    </Table.Td>
                    <Table.Td ta="right" onClick={(e) => e.stopPropagation()}>
                      {actions.length > 0 && (
                        <Menu position="bottom-end" withinPortal>
                          <Menu.Target>
                            <ActionIcon variant="subtle" aria-label={`Actions for ${b.name}`}>
                              <IconDots size={16} />
                            </ActionIcon>
                          </Menu.Target>
                          <Menu.Dropdown>{actions}</Menu.Dropdown>
                        </Menu>
                      )}
                    </Table.Td>
                  </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        </Card>
      )}

      <NewBlueprintModal
        opened={creating}
        project={project}
        onClose={() => setCreating(false)}
        onCreated={(b) => navigate(`/projects/${project.id}/blueprints/${b.id}`)}
      />
      <RenameBlueprintModal blueprint={renaming} onClose={() => setRenaming(null)} onDone={load} />
      {sharing && (
        <ShareDialog
          objectType="blueprint"
          objectId={sharing.id}
          objectName={sharing.name}
          onClose={() => {
            setSharing(null);
            void load();
          }}
        />
      )}
    </>
  );
}

type Mode = "import" | "prompt" | "form";

const svgUrl = (svg: string) => `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

/**
 * Create flow. Upload (DXF, PDF, PNG, JPG): the plan is read as drawn and stored with its DXF. Describe or form: review
 * the drafted brief, then generate. Reading and drafting write nothing; only the last button creates the blueprint.
 */
function NewBlueprintModal({
  opened,
  project,
  onClose,
  onCreated,
}: {
  opened: boolean;
  project: Project;
  onClose: () => void;
  onCreated: (b: BlueprintDetail) => void;
}) {
  const [mode, setMode] = useState<Mode>("import");
  const [prompt, setPrompt] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [scale, setScale] = useState<string | number>("");
  const [longest, setLongest] = useState<string | number>("");
  const [brief, setBrief] = useState<Brief | null>(null);
  const [notes, setNotes] = useState<string[]>([]);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [unit, setUnit] = useState<string>("auto");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [plan, setPlan] = useState("1");

  useEffect(() => {
    if (!opened) return;
    setUnit("auto");
    setPreview(null);
    setPlan("1");
    setMode("import");
    setPrompt("");
    setFile(null);
    setScale("");
    setLongest("");
    setBrief(null);
    setNotes([]);
    setName("");
  }, [opened]);

  const draft = async (e: FormEvent) => {
    e.preventDefault();
    if (mode === "form") {
      setBrief(emptyBrief());
      return;
    }
    setBusy(true);
    try {
      if (mode === "import") {
        const form = new FormData();
        form.set("project_id", project.id);
        form.set("file", file as File);
        appendImportOptions(form);
        const p = await apiUpload<ImportPreview>("/blueprint-drafts/dxf-import", form);
        setPreview(p);
        setPlan(String(p.plans[0]?.index ?? 1));
        if (!name) setName((file as File).name.replace(/\.[a-z]+$/i, ""));
        return;
      }
      const d = await api<BlueprintDraft>("POST", "/blueprint-drafts", { project_id: project.id, prompt });
      setBrief(d.brief);
      setNotes(d.notes);
      if (!name) setName(d.brief.title);
    } catch (err) {
      notifyError(err);
    } finally {
      setBusy(false);
    }
  };

  const isDxf = !file || /\.dxf$/i.test(file.name);
  // a DXF carries its own units; a PDF or picture carries no scale, so the user may state it
  const appendImportOptions = (form: FormData) => {
    if (isDxf) {
      if (unit !== "auto") form.set("unit", unit);
      return;
    }
    if (longest !== "") form.set("longest_m", String(longest));
    else if (scale !== "") form.set("scale", String(scale));
  };

  const importPlan = async () => {
    if (!file || !preview) return;
    setBusy(true);
    const out: { created?: BlueprintDetail } = {};
    await run(async () => {
      const form = new FormData();
      form.set("project_id", project.id);
      form.set("file", file);
      form.set("name", name.trim());
      form.set("plan", plan);
      appendImportOptions(form);
      out.created = await apiUpload<BlueprintDetail>("/blueprints/import", form);
    }, "Plan imported");
    setBusy(false);
    if (out.created) {
      onClose();
      onCreated(out.created);
    }
  };

  const generate = async () => {
    if (!brief) return;
    setBusy(true);
    const out: { created?: BlueprintDetail } = {};
    await run(async () => {
      out.created = await api<BlueprintDetail>("POST", "/blueprints", {
        project_id: project.id,
        name: name.trim() || brief.title,
        brief,
        source_kind: mode,
        // the description becomes version 1's message: the first chat entry
        prompt: mode === "prompt" ? prompt.trim() : undefined,
      });
    }, "Blueprint created");
    setBusy(false);
    if (out.created) {
      onClose();
      onCreated(out.created);
    }
  };

  const canDraft = mode === "form" || (mode === "prompt" ? prompt.trim().length > 0 : file !== null);
  const chosen = preview?.plans.find((p) => String(p.index) === plan);

  return (
    <Modal opened={opened} onClose={onClose} title="New blueprint" size={brief || preview ? "xl" : "lg"} closeOnClickOutside={!busy}>
      {preview ? (
        <Stack>
          <Alert variant="light" color="gray" title={`Read ${preview.source.name}`}>
            <List size="sm">
              <List.Item>
                Units: {preview.report.units} ({preview.report.unit_source}); {preview.report.plans_found} plan{preview.report.plans_found === 1 ? "" : "s"}, {preview.report.doors} doors, {preview.report.windows} windows, {preview.report.fixtures} fixtures
              </List.Item>
              {preview.conversion && (
                <List.Item>
                  Converted from a {preview.conversion.method === "pdf-vector" ? "vector PDF" : "picture"}: {preview.conversion.metres_across} m across ({preview.conversion.scale_source}), {preview.conversion.walls} wall shapes, {preview.conversion.openings} openings
                </List.Item>
              )}
              {preview.conversion?.skipped.map((t, i) => (
                <List.Item key={`c${i}`}>Not converted: {t}</List.Item>
              ))}
              {preview.conversion?.warnings.map((w, i) => (
                <List.Item key={`cw${i}`} c="orange">{w}</List.Item>
              ))}
              <List.Item>Wall layers used: {preview.report.layers_used.join(", ") || "none"}</List.Item>
              {preview.report.layers_ignored.length > 0 && <List.Item>Ignored layers: {preview.report.layers_ignored.join(", ")}</List.Item>}
              {preview.report.unlabeled_rooms > 0 && <List.Item>{preview.report.unlabeled_rooms} room(s) have no label and are named Room n.</List.Item>}
              {preview.report.unmapped_texts.map((t, i) => (
                <List.Item key={i}>Label not recognised: {t}</List.Item>
              ))}
              {preview.report.skipped.map((t, i) => (
                <List.Item key={`s${i}`}>Not imported: {t}</List.Item>
              ))}
              {preview.report.warnings.map((w, i) => (
                <List.Item key={`w${i}`}>{w}</List.Item>
              ))}
            </List>
          </Alert>
          <Radio.Group value={plan} onChange={setPlan} label="Plan to import" description="One blueprint per plan; import the file again for another.">
            <Group align="stretch" mt="xs" wrap="wrap">
              {preview.plans.map((p) => (
                <Card key={p.index} withBorder padding="xs" w={260} style={{ borderColor: String(p.index) === plan ? "var(--mantine-primary-color-filled)" : undefined }}>
                  <Radio value={String(p.index)} label={p.room_count === 0 ? `${p.name}: no rooms (drawing only)` : `${p.name}: ${p.room_count} rooms, ${p.net_area} m²`} mb="xs" />
                  <img src={svgUrl(p.svg)} alt={p.name} style={{ width: "100%", height: 150, objectFit: "contain", background: "#fff" }} />
                  <Text size="xs" c={p.ok ? "dimmed" : "orange"} mt={4}>
                    {p.ok ? "No failing checks" : `${p.checks.filter((c) => !c.ok && c.severity === "error").length} failing check(s)`}
                  </Text>
                </Card>
              ))}
            </Group>
          </Radio.Group>
          <Text size="sm" c="dimmed">
            The plan is stored as read; the original DXF is kept with it (for a PDF or picture, the DXF drawn from it) and can be downloaded by editors. Edit it in the viewer or by chat; AI review and the brief form do not apply to a DXF plan.
          </Text>
          <TextInput label="Name" required value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Group justify="space-between">
            <Button variant="subtle" onClick={() => setPreview(null)} disabled={busy}>
              Back
            </Button>
            <Group>
              <Button variant="default" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button onClick={importPlan} loading={busy} disabled={!name.trim() || !chosen}>
                Import plan
              </Button>
            </Group>
          </Group>
        </Stack>
      ) : !brief ? (
        <form onSubmit={draft}>
          <Stack>
            <SegmentedControl
              value={mode}
              onChange={(v) => setMode(v as Mode)}
              data={[
                { value: "import", label: "Upload plan" },
                { value: "prompt", label: "Describe" },
                { value: "form", label: "Fill the form" },
              ]}
            />
            {mode === "import" && (
              <>
                <FileInput
                  label="Floor plan file"
                  description="DXF, PDF, PNG or JPG, at most 20 MB. The plan is read as drawn, with no AI; the original (for a PDF or picture, the DXF drawn from it) is stored with the blueprint."
                  accept=".dxf,.pdf,.png,.jpg,.jpeg"
                  leftSection={<IconUpload size={16} />}
                  clearable
                  value={file}
                  onChange={setFile}
                />
                {file && isDxf && (
                  <Select label="Drawing units" description="Auto uses the file's own setting, or guesses from its size." data={["auto", ...IMPORT_UNITS]} value={unit} onChange={(v) => setUnit(v ?? "auto")} allowDeselect={false} />
                )}
                {file && !isDxf && (
                  <>
                    <NumberInput
                      label="Longest side of the drawing (m)"
                      description="Optional. Leave empty to read the scale from the picture (the room areas or the overall dimension written on it); fill it in when the picture has neither."
                      placeholder="e.g. 12.5"
                      min={1}
                      max={500}
                      decimalScale={2}
                      value={longest}
                      onChange={setLongest}
                    />
                    <NumberInput
                      label="Or paper scale 1:N"
                      description="Optional; used only when the length above is empty. A PDF defaults to 1:100."
                      placeholder="100"
                      min={1}
                      max={5000}
                      allowDecimal={false}
                      disabled={longest !== ""}
                      value={scale}
                      onChange={setScale}
                    />
                    <Text size="xs" c="dimmed">
                      A PDF drawn as vectors is read exactly. A picture is traced and its text is read (OCR): walls, openings and room names are found; door swings and furniture are not.
                    </Text>
                  </>
                )}
              </>
            )}
            {mode === "prompt" && (
              <Textarea
                label="Describe the house"
                description="Rooms, lot size, street side. Single storey."
                placeholder="Two bedroom house, one bathroom, open kitchen and living, 16 x 24 lot, entrance from the south"
                autosize
                minRows={3}
                maxLength={4000}
                data-autofocus
                value={prompt}
                onChange={(e) => setPrompt(e.currentTarget.value)}
              />
            )}
            {mode === "form" && <Text size="sm">Start from a blank brief and enter the rooms yourself.</Text>}
            {busy && (
              <Alert variant="light" icon={<Loader size="xs" />}>
                {mode === "import" ? (isDxf ? "Reading the DXF…" : "Converting the file to a DXF and reading it…") : "The local model is drafting the brief. This can take up to a minute."}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button type="submit" loading={busy} disabled={!canDraft} leftSection={mode === "form" ? <IconForms size={16} /> : mode === "import" ? <IconUpload size={16} /> : <IconWand size={16} />}>
                {mode === "form" ? "Open form" : mode === "import" ? "Read file" : "Draft brief"}
              </Button>
            </Group>
          </Stack>
        </form>
      ) : (
        <Stack>
          <TextInput label="Name" required value={name} onChange={(e) => setName(e.currentTarget.value)} />
          {notes.length > 0 && (
            <Alert variant="light" color="gray" title="Notes from the draft">
              <List size="sm">
                {notes.map((n, i) => (
                  <List.Item key={i}>{n}</List.Item>
                ))}
              </List>
            </Alert>
          )}
          <Text size="sm" c="dimmed">
            Review the brief. Generate lays out the rooms (if they do not fit the lot, rooms shrink toward their minimum sizes) and draws them into a DXF. From then on the plan is that drawing: you edit it in the viewer or by chat, and the brief is not kept.
          </Text>
          <BriefEditor value={brief} onChange={setBrief} readOnly={busy} />
          <Group justify="space-between">
            <Button variant="subtle" onClick={() => setBrief(null)} disabled={busy}>
              Back
            </Button>
            <Group>
              <Button variant="default" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button onClick={generate} loading={busy} disabled={!name.trim() && !brief.title.trim()}>
                Generate
              </Button>
            </Group>
          </Group>
        </Stack>
      )}
    </Modal>
  );
}

export function RenameBlueprintModal({ blueprint, onClose, onDone }: { blueprint: Blueprint | null; onClose: () => void; onDone: () => void | Promise<void> }) {
  const [name, setName] = useState("");
  useEffect(() => setName(blueprint?.name ?? ""), [blueprint]);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (blueprint && (await run(() => api("PATCH", `/blueprints/${blueprint.id}`, { name: name.trim() }), "Renamed"))) {
      onClose();
      await onDone();
    }
  };
  return (
    <Modal opened={blueprint !== null} onClose={onClose} title="Rename blueprint">
      <form onSubmit={submit}>
        <Stack>
          <TextInput label="Name" required data-autofocus value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim()}>
              Save
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
