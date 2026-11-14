import {
  Alert,
  Anchor,
  Badge,
  Box,
  Button,
  Card,
  Center,
  Grid,
  Group,
  List,
  Loader,
  Modal,
  Paper,
  SegmentedControl,
  ScrollArea,
  Stack,
  Table,
  Tabs,
  Text,
  Textarea,
  ThemeIcon,
  Tooltip,
} from "@mantine/core";
import {
  IconAlertTriangle,
  IconArrowBackUp,
  IconCheck,
  IconDownload,
  IconEye,
  IconForms,
  IconHistory,
  IconListCheck,
  IconMessage,
  IconPencil,
  IconRestore,
  IconSend,
  IconShare,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, apiBlob, apiDownload, apiStream, type ExportReport, type Progress, type BlueprintDetail as Detail, type BlueprintVersionSummary } from "../api";
import { useAuth } from "../auth";
import PlanViewer from "../components/PlanViewer";
import ShareDialog from "../components/ShareDialog";
import { asPlanModel, type PlanOp } from "../blueprintModel";
import { canAct } from "../permissions";
import { confirmDanger, notifyError, PageHeader, run } from "../ui";
import { RenameBlueprintModal } from "./Blueprints";

const KIND_LABEL: Record<BlueprintVersionSummary["kind"], string> = { create: "created", revise: "chat", review: "AI review", edit: "edit", restore: "restore", import: "imported" };

/**
 * One blueprint: the drawing on the left; chat, a read-only summary, checks, and
 * versions on the right. Every write sends base_version, so a stale page
 * gets 409 instead of overwriting someone else's revision.
 */
export default function BlueprintDetail() {
  const { id: projectId = "", bid = "" } = useParams();
  const { me, tenantId } = useAuth();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<Detail | null>(null);
  const [versions, setVersions] = useState<BlueprintVersionSummary[]>([]);
  const [shown, setShown] = useState<number | null>(null); // null = current version
  const [svg, setSvg] = useState<string | null>(null);
  const [drawingError, setDrawingError] = useState(false);
  const [message, setMessage] = useState("");
  const [thinking, setThinking] = useState<Progress[]>([]); // live steps of the running chat revision; never stored
  const [thought, setThought] = useState(""); // the model's reasoning for the current attempt; never stored
  const thoughtView = useRef<HTMLDivElement>(null);
  const [mode, setMode] = useState<"sheet" | "viewer">("sheet");
  const [busy, setBusy] = useState<"" | "chat" | "restore" | "edit" | "convert">("");
  const [exported, setExported] = useState<ExportReport | null>(null); // what the last edited-DXF export wrote
  const [renaming, setRenaming] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [missing, setMissing] = useState(false);

  const load = useCallback(async () => {
    if (!me) return;
    try {
      const [d, vs] = await Promise.all([api<Detail>("GET", `/blueprints/${bid}`), api<BlueprintVersionSummary[]>("GET", `/blueprints/${bid}/versions`)]);
      setDetail(d);
      setVersions(vs);
      setMissing(false);
    } catch (e) {
      notifyError(e);
      setDetail((cur) => {
        if (!cur) setMissing(true); // first load failed: no spinner forever
        return cur;
      });
    }
  }, [bid, me, tenantId]);

  useEffect(() => {
    const el = thoughtView.current; // keep the newest reasoning in view
    if (el) el.scrollTop = el.scrollHeight;
  }, [thought]);

  useEffect(() => {
    void load();
  }, [load]);

  const version = shown ?? detail?.current_version ?? 0;

  // The drawing needs the auth headers, so it is fetched and shown as an object URL.
  useEffect(() => {
    if (!version) return;
    let url: string | null = null;
    let cancelled = false;
    setDrawingError(false);
    apiBlob(`/blueprints/${bid}/drawing?format=svg&version=${version}`).then(
      (blob) => {
        if (cancelled) return;
        url = URL.createObjectURL(blob);
        setSvg(url);
      },
      (e) => {
        if (cancelled) return;
        setSvg(null);
        setDrawingError(true);
        notifyError(e);
      },
    );
    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url);
    };
  }, [bid, version]);

  if (missing) {
    return (
      <Stack align="center" py="xl" gap="xs">
        <Text c="dimmed">Blueprint not found.</Text>
        <Anchor component={Link} to={`/projects/${projectId}?tab=blueprints`} size="sm">
          ← Back to project
        </Anchor>
      </Stack>
    );
  }
  if (!detail) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }

  const canEdit = canAct(detail, "blueprint", "revise");
  const base = detail.current_version;
  // an imported plan has no brief: its geometry came from a DXF, so the AI review and the brief form do not apply; chat edits the plan as drawn
  const imported = detail.version.brief === null;
  const planModel = asPlanModel(detail.version.model);

  /** Runs a versioned write, then shows the new current version. */
  const write = async (kind: typeof busy, fn: () => Promise<unknown>, success: string) => {
    setBusy(kind);
    const ok = await run(fn, success);
    setBusy("");
    if (ok) setShown(null);
    await load(); // also refreshes after a 409
    return ok;
  };

  const send = async (e: FormEvent) => {
    e.preventDefault();
    setThinking([]);
    setThought("");
    const ok = await write(
      "chat",
      () =>
        apiStream("/blueprints/" + bid + "/revisions/stream", { message: message.trim(), base_version: base }, (p) => {
          if (p.kind === "thinking") {
            setThought((t) => t + p.message);
            return;
          }
          if (p.stage === "ask_model") setThought(""); // a new attempt thinks afresh
          setThinking((t) => [...t, p]);
        }),
      "Revised",
    );
    setThinking([]);
    setThought("");
    if (ok) setMessage("");
  };

  // One direct edit of an imported plan = one request = one new version (Versions is the undo list).
  const editPlan = (ops: PlanOp[]) => write("edit", () => api("POST", `/blueprints/${bid}/plan-edits`, { ops: { ops }, base_version: base }), "Edited");

  // A blueprint made before DXF plans still has a brief: draw its plan into a DXF (no LLM) and keep that as its
  // original. The old versions stay; this is one new version, after which it is edited like any DXF plan.
  const convert = () => void write("convert", () => api("POST", `/blueprints/${bid}/convert`, { base_version: base }), "Converted to a DXF plan");

  // The DXF kept with the plan: the uploaded file for an import, the drawing the engine made for a created plan
  // (its source, when there is one, is the sketch).
  const originalName = detail.source_kind === "import" ? detail.source?.name ?? "original.dxf" : `${detail.name}.dxf`;
  const downloadOriginal = async () => {
    try {
      const blob = await apiBlob(`/blueprints/${bid}/source`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = originalName;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      notifyError(e);
    }
  };

  // The original file with this version's edits written into it; the report says what the file could not take.
  const downloadEdited = async () => {
    try {
      const { blob, headers } = await apiDownload(`/blueprints/${bid}/source/edited?version=${version}`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = originalName.replace(/\.dxf$/i, "") + `-edited-v${version}.dxf`;
      a.click();
      URL.revokeObjectURL(url);
      const raw = headers.get("X-Export-Report");
      if (raw) setExported(JSON.parse(atob(raw)) as ExportReport);
    } catch (e) {
      notifyError(e);
    }
  };

  const download = async (format: "pdf" | "dxf" | "ifc") => {
    try {
      const blob = await apiBlob(`/blueprints/${bid}/drawing?format=${format}&version=${version}`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${detail.name.replace(/[^A-Za-z0-9_-]+/g, "-")}-v${version}.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      notifyError(e);
    }
  };

  const remove = () =>
    confirmDanger(`Delete blueprint “${detail.name}”?`, "Every version, its tuples, and its grants are deleted. This cannot be undone.", "Delete", () => {
      void run(() => api("DELETE", `/blueprints/${bid}`), `Deleted ${detail.name}`).then((ok) => {
        if (ok) navigate(`/projects/${projectId}?tab=blueprints`);
      });
    });

  const checks = detail.version.checks ?? [];
  const failed = checks.filter((c) => !c.ok);
  // oldest first: the prompt the blueprint was created from, then every chat revision
  const chat = [...versions].reverse().filter((v) => v.kind !== "create" || v.message);

  return (
    <>
      <Anchor component={Link} to={`/projects/${projectId}?tab=blueprints`} size="sm">
        ← Back to project
      </Anchor>
      <PageHeader
        title={
          <Group gap="sm">
            {detail.name}
            <Badge variant="light">v{detail.current_version}</Badge>
            {detail.version.ok ? (
              <Badge color="teal" variant="light">
                checks pass
              </Badge>
            ) : (
              <Badge color="orange" variant="light">
                {failed.length} check{failed.length === 1 ? "" : "s"} failing
              </Badge>
            )}
          </Group>
        }
        description={imported
          ? `${detail.source_kind === "import" ? `Imported from ${detail.source?.name ?? "a DXF"}. The plan is shown as drawn in the file` : `Created from your ${detail.source_kind}${detail.source ? ` (${detail.source.name})` : ""} and drawn into a DXF. The plan is that drawing`}; edit it in the Viewer or by chat: rename rooms, change room types, move, turn, add or remove fixtures, slide, resize or remove doors and windows (not walls yet). “Edited DXF” writes those edits back into the original file.`
          : `From ${detail.source_kind}${detail.source ? ` (${detail.source.name})` : ""}. An older blueprint: chat redraws it from its stored brief. Press “Make editable” to turn it into a DXF plan you can edit in the Viewer and export.`}
        actions={
          <>
            <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={() => void download("dxf")}>
              DXF
            </Button>
            <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={() => void download("pdf")}>
              PDF
            </Button>
            <Tooltip label="IFC4 BIM model: walls, doors, windows, spaces, slab, fixtures">
              <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={() => void download("ifc")}>
                IFC
              </Button>
            </Tooltip>
            {!imported && canEdit && (
              <Tooltip label="Draw this plan into a DXF and keep that as its original. You can then edit it in the Viewer and by chat, and download the edited DXF. Earlier versions stay in Versions.">
                <Button size="xs" leftSection={<IconPencil size={14} />} loading={busy === "convert"} disabled={busy !== "" || shown !== null} onClick={convert}>
                  Make editable
                </Button>
              </Tooltip>
            )}
            {imported && canEdit && (
              <>
                <Tooltip label="The DXF this plan was read from or drawn into (editors only)">
                  <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={() => void downloadOriginal()}>
                    Original DXF
                  </Button>
                </Tooltip>
                <Tooltip label="The original file with this version's edits written into it; everything else in the file is untouched">
                  <Button variant="default" size="xs" leftSection={<IconDownload size={14} />} onClick={() => void downloadEdited()}>
                    Edited DXF
                  </Button>
                </Tooltip>
              </>
            )}
            {canAct(detail, "blueprint", "rename") && (
              <Button variant="default" size="xs" leftSection={<IconPencil size={14} />} onClick={() => setRenaming(true)}>
                Rename
              </Button>
            )}
            {canAct(detail, "blueprint", "share") && (
              <Button variant="default" size="xs" leftSection={<IconShare size={14} />} onClick={() => setSharing(true)}>
                Share
              </Button>
            )}
            {canAct(detail, "blueprint", "delete") && (
              <Button color="red" variant="light" size="xs" leftSection={<IconTrash size={14} />} onClick={remove}>
                Delete
              </Button>
            )}
          </>
        }
      />

      <Grid gap="lg">
        <Grid.Col span={{ base: 12, lg: 8 }}>
          <Card withBorder padding="xs">
            {shown !== null && (
              <Alert mb="xs" variant="light" color="yellow" p="xs">
                <Group justify="space-between">
                  <Text size="sm">Viewing version {shown}.</Text>
                  <Button size="compact-xs" variant="subtle" onClick={() => setShown(null)}>
                    Back to current
                  </Button>
                </Group>
              </Alert>
            )}
            <SegmentedControl size="xs" mb="xs" value={mode} disabled={shown !== null} onChange={(v) => setMode(v as "sheet" | "viewer")}
              data={[{ label: "Drawing sheet", value: "sheet" }, { label: "Viewer", value: "viewer" }]} />
            {mode === "viewer" && planModel && shown === null ? (
              <PlanViewer key={bid} model={planModel} busy={busy !== ""} onEdit={imported && canEdit ? editPlan : undefined} />
            ) : (
            /* The sheet is drawn on white paper in both color schemes. */
            <Paper bg="white" radius="sm" style={{ overflow: "hidden" }}>
              {svg ? (
                <img src={svg} alt={`${detail.name}, version ${version}`} style={{ display: "block", width: "100%", height: "auto" }} />
              ) : (
                <Center h={360}>{drawingError ? <Text c="dimmed">The drawing could not be rendered.</Text> : <Loader />}</Center>
              )}
            </Paper>
            )}
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, lg: 4 }}>
          <Tabs defaultValue="chat" keepMounted={false}>
            <Tabs.List mb="md">
              <Tabs.Tab value="chat" leftSection={<IconMessage size={14} />}>
                Chat
              </Tabs.Tab>
              {planModel && (
                <Tabs.Tab value="summary" leftSection={<IconForms size={14} />}>
                  Summary
                </Tabs.Tab>
              )}
              <Tabs.Tab value="checks" leftSection={<IconListCheck size={14} />}>
                Checks
              </Tabs.Tab>
              <Tabs.Tab value="versions" leftSection={<IconHistory size={14} />}>
                Versions
              </Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="chat">
              <Stack>
                <ScrollArea.Autosize mah={360}>
                  <Stack gap="xs">
                    {chat.every((v) => v.kind === "create") && (
                      <Text size="sm" c="dimmed">
                        {imported
                          ? "No revisions yet. Ask for a change, for example “rename Room 3 to Study”, “add a desk to the study” or “make door d2 0.8 m wide”."
                          : "No revisions yet. Ask for a change, for example “add a study” or “make the kitchen bigger”."}
                      </Text>
                    )}
                    {chat.map((v) => (
                      <Paper key={v.version} withBorder p="xs" radius="md">
                        <Group justify="space-between" gap="xs" wrap="nowrap">
                          <Text size="sm">{v.message || KIND_LABEL[v.kind]}</Text>
                          <Badge size="xs" variant="light" color={v.kind === "revise" ? "indigo" : v.kind === "review" ? "grape" : "gray"}>
                            v{v.version} · {KIND_LABEL[v.kind]}
                          </Badge>
                        </Group>
                      </Paper>
                    ))}
                  </Stack>
                </ScrollArea.Autosize>
                {busy === "chat" && (
                  <Paper withBorder p="xs" radius="md" bg="var(--mantine-color-gray-light)">
                    <Group gap="xs" mb={thinking.length ? 4 : 0}>
                      <Loader size="xs" />
                      <Text size="xs" fw={500}>
                        Working…
                      </Text>
                    </Group>
                    <Stack gap={2}>
                      {thinking.map((t, i) => (
                        <Text key={i} size="xs" c={t.stage === "retry" ? "orange" : i === thinking.length - 1 ? undefined : "dimmed"}>
                          {t.message}
                        </Text>
                      ))}
                    </Stack>
                    {thought && (
                      <ScrollArea.Autosize mah={140} mt={6} viewportRef={thoughtView}>
                        <Text size="xs" c="dimmed" fs="italic" style={{ whiteSpace: "pre-wrap" }}>
                          {thought}
                        </Text>
                      </ScrollArea.Autosize>
                    )}
                  </Paper>
                )}
                {detail.version.notes.length > 0 && (
                  <Alert variant="light" color="gray" p="xs" title={`Notes on v${detail.current_version}`}>
                    <List size="xs">
                      {detail.version.notes.map((n, i) => (
                        <List.Item key={i}>{n}</List.Item>
                      ))}
                    </List>
                  </Alert>
                )}
                {canEdit ? (
                  <form onSubmit={send}>
                    <Stack gap="xs">
                      <Textarea
                        placeholder={imported ? "Rename the hall to Corridor, add a sofa to the living room, remove window w3…" : "Move the kitchen to the north side, add a laundry, make bedroom 2 bigger…"}
                        autosize
                        minRows={2}
                        maxLength={2000}
                        value={message}
                        onChange={(e) => setMessage(e.currentTarget.value)}
                        disabled={busy !== ""}
                      />
                      <Group justify="space-between">
                        <Text size="xs" c="dimmed">
                          {busy === "chat" ? imported ? "The local model is editing the plan…" : "The local model is revising the plan…" : "Unchanged rooms stay where they are."}
                        </Text>
                        <Button type="submit" size="xs" leftSection={<IconSend size={14} />} loading={busy === "chat"} disabled={!message.trim() || busy !== ""}>
                          Send
                        </Button>
                      </Group>
                    </Stack>
                  </form>
                ) : (
                  <Text size="sm" c="dimmed">
                    You can view this blueprint but not revise it.
                  </Text>
                )}
              </Stack>
            </Tabs.Panel>

            {planModel && (
              <Tabs.Panel value="summary">
                <Stack gap="xs">
                  <Text size="xs" c="dimmed">
                    {imported ? "Read from the plan as it is now. Edit it in the Viewer or by chat." : "Read from the plan as it is now. This older blueprint is not a DXF plan yet: press “Make editable” to edit it in the Viewer."}
                  </Text>
                  <Table withTableBorder fz="xs">
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>Room</Table.Th>
                        <Table.Th>Type</Table.Th>
                        <Table.Th ta="right">m²</Table.Th>
                        <Table.Th>Fixtures</Table.Th>
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {planModel.rooms.map((r) => (
                        <Table.Tr key={r.id}>
                          <Table.Td>{r.name}</Table.Td>
                          <Table.Td>{r.type}</Table.Td>
                          <Table.Td ta="right">{r.net_area.toFixed(1)}</Table.Td>
                          <Table.Td>{planModel.fixtures.filter((f) => f.room_id === r.id).map((f) => f.kind.replace(/_/g, " ")).join(", ") || "–"}</Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                  <Text size="xs" c="dimmed">
                    {planModel.rooms.length} rooms, {planModel.rooms.filter((r) => r.type !== "balcony").reduce((a, r) => a + r.net_area, 0).toFixed(1)} m² net (balconies excluded),{" "}
                    {planModel.openings.filter((o) => o.kind === "window").length} windows, {planModel.openings.filter((o) => o.kind !== "window").length} doors and openings.
                  </Text>
                </Stack>
              </Tabs.Panel>
            )}

            <Tabs.Panel value="checks">
              <Stack gap="xs">
                {detail.version.kind === "review" && detail.version.notes.length > 0 && (
                  <Alert variant="light" color="grape" p="xs" title={`AI review (v${detail.current_version})`}>
                    <List size="xs">
                      {detail.version.notes.map((n, i) => (
                        <List.Item key={i}>{n}</List.Item>
                      ))}
                    </List>
                  </Alert>
                )}
                {checks.length === 0 && (
                  <Text size="sm" c="dimmed">
                    No checks recorded.
                  </Text>
                )}
                {checks.map((c, i) => (
                  <Group key={i} gap="xs" wrap="nowrap" align="flex-start">
                    <ThemeIcon size="sm" variant="light" color={c.ok ? "teal" : c.severity === "error" ? "red" : "orange"}>
                      {c.ok ? <IconCheck size={12} /> : c.severity === "error" ? <IconX size={12} /> : <IconAlertTriangle size={12} />}
                    </ThemeIcon>
                    <Box>
                      <Text size="sm">{c.message}</Text>
                      <Text size="xs" c="dimmed">
                        {c.rule}
                        {c.category ? ` · ${c.category}` : ""}
                        {c.room_id ? ` · ${c.room_id}` : ""}
                      </Text>
                      {!c.ok && c.hint && (
                        <Text size="xs" c="dimmed" fs="italic">
                          Fix: {c.hint}
                        </Text>
                      )}
                    </Box>
                  </Group>
                ))}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="versions">
              <Card withBorder padding={0}>
                <Table verticalSpacing="xs" fz="sm">
                  <Table.Tbody>
                    {versions.map((v) => (
                      <Table.Tr key={v.version} bg={v.version === version ? "var(--mantine-primary-color-light)" : undefined}>
                        <Table.Td>
                          <Badge variant="light">v{v.version}</Badge>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm">{v.message || KIND_LABEL[v.kind]}</Text>
                          <Text size="xs" c="dimmed">
                            {KIND_LABEL[v.kind]} · {new Date(v.created_at).toLocaleString()}
                          </Text>
                        </Table.Td>
                        <Table.Td ta="right">
                          <Group gap={4} justify="flex-end" wrap="nowrap">
                            <Tooltip label="View drawing">
                              <Button
                                size="compact-xs"
                                variant="subtle"
                                leftSection={<IconEye size={12} />}
                                onClick={() => setShown(v.version === detail.current_version ? null : v.version)}
                              >
                                View
                              </Button>
                            </Tooltip>
                            {canEdit && v.version < detail.current_version && (
                              <Button
                                size="compact-xs"
                                variant="light"
                                leftSection={<IconRestore size={12} />}
                                loading={busy === "restore"}
                                disabled={busy !== ""}
                                onClick={() =>
                                  void write("restore", () => api("POST", `/blueprints/${bid}/versions/${v.version}/restore`, { base_version: base }), `Restored version ${v.version}`)
                                }
                              >
                                Restore
                              </Button>
                            )}
                          </Group>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Card>
              <Text size="xs" c="dimmed" mt="xs">
                <IconArrowBackUp size={12} /> Restore copies an older version into a new one; history is never rewritten.
              </Text>
            </Tabs.Panel>
          </Tabs>
        </Grid.Col>
      </Grid>

      <RenameBlueprintModal blueprint={renaming ? detail : null} onClose={() => setRenaming(false)} onDone={load} />
      {sharing && <ShareDialog objectType="blueprint" objectId={detail.id} objectName={detail.name} onClose={() => setSharing(false)} />}
          <Modal opened={exported !== null} onClose={() => setExported(null)} title="Edited DXF downloaded" size="lg">
        {exported && (
          <Stack gap="xs">
            <Text size="sm">
              {exported.applied.length + exported.added.length === 0 && exported.skipped.length === 0
                ? "No edits yet: the file is the original."
                : "The original file with this version's edits written into it. Everything else in the file is unchanged."}
            </Text>
            {[["Written into the file", exported.applied, "teal"], ["Added (new entities)", exported.added, "blue"], ["Not written (the file cannot take them)", exported.skipped, "orange"]].map(([title, items, color]) =>
              (items as string[]).length > 0 ? (
                <Alert key={title as string} variant="light" color={color as string} p="xs" title={title as string}>
                  <List size="xs">
                    {(items as string[]).map((m, i) => (
                      <List.Item key={i}>{m}</List.Item>
                    ))}
                  </List>
                </Alert>
              ) : null,
            )}
          </Stack>
        )}
      </Modal>
</>
  );
}
