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
  SegmentedControl,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { IconDots, IconForms, IconPencil, IconPlus, IconRuler2, IconShare, IconTrash, IconUpload, IconWand } from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api, apiUpload, type Blueprint, type BlueprintDetail, type BlueprintDraft, type BlueprintSource, type Brief, type Project } from "../api";
import { useAuth } from "../auth";
import BriefEditor, { emptyBrief } from "../components/BriefEditor";
import ShareDialog from "../components/ShareDialog";
import { canAct, canProject } from "../permissions";
import { confirmDanger, notifyError, run } from "../ui";

const SOURCE_LABEL: Record<Blueprint["source_kind"], string> = { prompt: "prompt", sketch: "sketch", form: "form" };

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
          Floor plans drafted by a local AI from a description or a sketch. The AI edits only the brief; the drawing is computed from it.
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

type Mode = "prompt" | "sketch" | "form";

/**
 * Create flow: describe or upload, review the drafted brief, then generate.
 * Drafting writes nothing; only Generate creates the blueprint.
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
  const [mode, setMode] = useState<Mode>("prompt");
  const [prompt, setPrompt] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [note, setNote] = useState("");
  const [brief, setBrief] = useState<Brief | null>(null);
  const [notes, setNotes] = useState<string[]>([]);
  const [source, setSource] = useState<BlueprintSource | null>(null);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!opened) return;
    setMode("prompt");
    setPrompt("");
    setFile(null);
    setNote("");
    setBrief(null);
    setNotes([]);
    setSource(null);
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
      let d: BlueprintDraft;
      if (mode === "prompt") {
        d = await api<BlueprintDraft>("POST", "/blueprint-drafts", { project_id: project.id, prompt });
      } else {
        const form = new FormData();
        form.set("project_id", project.id);
        form.set("file", file as File);
        form.set("note", note);
        d = await apiUpload<BlueprintDraft>("/blueprint-drafts/sketch", form);
        setSource(d.source ?? null);
      }
      setBrief(d.brief);
      setNotes(d.notes);
      if (!name) setName(d.brief.title);
    } catch (err) {
      notifyError(err);
    } finally {
      setBusy(false);
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
        source: mode === "sketch" ? source : undefined,
        // the description (or sketch note) becomes version 1's message: the first chat entry
        prompt: mode === "prompt" ? prompt.trim() : mode === "sketch" ? note.trim() : undefined,
      });
    }, "Blueprint created");
    setBusy(false);
    if (out.created) {
      onClose();
      onCreated(out.created);
    }
  };

  const canDraft = mode === "form" || (mode === "prompt" ? prompt.trim().length > 0 : file !== null);

  return (
    <Modal opened={opened} onClose={onClose} title="New blueprint" size={brief ? "xl" : "lg"} closeOnClickOutside={!busy}>
      {!brief ? (
        <form onSubmit={draft}>
          <Stack>
            <SegmentedControl
              value={mode}
              onChange={(v) => setMode(v as Mode)}
              data={[
                { value: "prompt", label: "Describe" },
                { value: "sketch", label: "Upload sketch" },
                { value: "form", label: "Fill the form" },
              ]}
            />
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
            {mode === "sketch" && (
              <>
                <FileInput
                  label="Sketch or existing plan"
                  description="PNG, JPG, or PDF (first page), at most 10 MB. Read into a brief, then discarded; only its name and size are kept."
                  accept="image/png,image/jpeg,application/pdf"
                  leftSection={<IconUpload size={16} />}
                  clearable
                  value={file}
                  onChange={setFile}
                />
                <Textarea label="Note (optional)" placeholder="Lot is 15 x 25, entrance on the east" autosize minRows={2} maxLength={1000} value={note} onChange={(e) => setNote(e.currentTarget.value)} />
              </>
            )}
            {mode === "form" && <Text size="sm">Start from a blank brief and enter the rooms yourself.</Text>}
            {busy && (
              <Alert variant="light" icon={<Loader size="xs" />}>
                The local model is drafting the brief. This can take up to a minute.
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button type="submit" loading={busy} disabled={!canDraft} leftSection={mode === "form" ? <IconForms size={16} /> : <IconWand size={16} />}>
                {mode === "form" ? "Open form" : "Draft brief"}
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
            Review the brief. Generate lays out the rooms; if they do not fit the lot, rooms shrink toward their minimum sizes.
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
