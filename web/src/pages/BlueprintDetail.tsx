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
  Paper,
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
  IconSparkles,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, apiBlob, type BlueprintDetail as Detail, type BlueprintVersionSummary, type Brief } from "../api";
import { useAuth } from "../auth";
import BriefEditor from "../components/BriefEditor";
import ShareDialog from "../components/ShareDialog";
import { canAct } from "../permissions";
import { confirmDanger, notifyError, PageHeader, run } from "../ui";
import { RenameBlueprintModal } from "./Blueprints";

const KIND_LABEL: Record<BlueprintVersionSummary["kind"], string> = { create: "created", revise: "chat", review: "AI review", edit: "form edit", restore: "restore" };

/**
 * One blueprint: the drawing on the left; chat, brief form, checks, and
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
  const [brief, setBrief] = useState<Brief | null>(null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState<"" | "chat" | "brief" | "restore" | "review">("");
  const [renaming, setRenaming] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [missing, setMissing] = useState(false);

  const load = useCallback(async () => {
    if (!me) return;
    try {
      const [d, vs] = await Promise.all([api<Detail>("GET", `/blueprints/${bid}`), api<BlueprintVersionSummary[]>("GET", `/blueprints/${bid}/versions`)]);
      setDetail(d);
      setVersions(vs);
      setBrief(d.version.brief);
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
  if (!detail || !brief) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }

  const canEdit = canAct(detail, "blueprint", "revise");
  const base = detail.current_version;
  const dirty = JSON.stringify(brief) !== JSON.stringify(detail.version.brief);

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
    if (await write("chat", () => api("POST", `/blueprints/${bid}/revisions`, { message: message.trim(), base_version: base }), "Revised")) setMessage("");
  };

  const review = () => void write("review", () => api("POST", `/blueprints/${bid}/reviews`, { base_version: base }), "Reviewed");

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
        description={`From ${detail.source_kind}${detail.source ? ` (${detail.source.name})` : ""}. The AI edits only the brief; the drawing is computed from it.`}
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
        <Grid.Col span={{ base: 12, lg: 7 }}>
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
            {/* The sheet is drawn on white paper in both color schemes. */}
            <Paper bg="white" radius="sm" style={{ overflow: "hidden" }}>
              {svg ? (
                <img src={svg} alt={`${detail.name}, version ${version}`} style={{ display: "block", width: "100%", height: "auto" }} />
              ) : (
                <Center h={360}>{drawingError ? <Text c="dimmed">The drawing could not be rendered.</Text> : <Loader />}</Center>
              )}
            </Paper>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, lg: 5 }}>
          <Tabs defaultValue="chat" keepMounted={false}>
            <Tabs.List mb="md">
              <Tabs.Tab value="chat" leftSection={<IconMessage size={14} />}>
                Chat
              </Tabs.Tab>
              <Tabs.Tab value="brief" leftSection={<IconForms size={14} />}>
                Brief
              </Tabs.Tab>
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
                        No revisions yet. Ask for a change, for example “add a study” or “make the kitchen bigger”.
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
                        placeholder="Move the kitchen to the north side, add a laundry, make bedroom 2 bigger…"
                        autosize
                        minRows={2}
                        maxLength={2000}
                        value={message}
                        onChange={(e) => setMessage(e.currentTarget.value)}
                        disabled={busy !== ""}
                      />
                      <Group justify="space-between">
                        <Text size="xs" c="dimmed">
                          {busy === "chat" ? "The local model is revising the brief…" : "Unchanged rooms stay where they are."}
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

            <Tabs.Panel value="brief">
              <Stack>
                <BriefEditor value={brief} onChange={setBrief} readOnly={!canEdit || busy !== ""} />
                {canEdit && (
                  <Group justify="flex-end">
                    <Button variant="default" size="xs" disabled={!dirty || busy !== ""} onClick={() => setBrief(detail.version.brief)}>
                      Reset
                    </Button>
                    <Button
                      size="xs"
                      loading={busy === "brief"}
                      disabled={!dirty || busy !== ""}
                      onClick={() => void write("brief", () => api("PUT", `/blueprints/${bid}/brief`, { brief, base_version: base }), "Brief saved")}
                    >
                      Save and regenerate
                    </Button>
                  </Group>
                )}
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="checks">
              <Stack gap="xs">
                {canEdit && failed.length > 0 && (
                  <Group justify="space-between" wrap="nowrap">
                    <Text size="xs" c="dimmed">
                      {busy === "review"
                        ? "The local model is reviewing the issues (up to 3 rounds)…"
                        : "The AI proposes brief edits for these issues and keeps only rounds that reduce them. Rooms are never removed."}
                    </Text>
                    <Button size="xs" variant="light" leftSection={<IconSparkles size={14} />} loading={busy === "review"} disabled={busy !== ""} onClick={review}>
                      Fix issues with AI
                    </Button>
                  </Group>
                )}
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
    </>
  );
}
