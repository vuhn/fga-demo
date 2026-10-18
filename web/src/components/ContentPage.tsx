import {
  ActionIcon,
  Badge,
  Box,
  Button,
  Card,
  Center,
  Collapse,
  EmptyState,
  Group,
  Loader,
  Menu,
  Modal,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
  ThemeIcon,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconChevronDown,
  IconChevronRight,
  IconEye,
  IconEyeOff,
  IconLock,
  IconWorld,
  IconDots,
  IconFile,
  IconFolder,
  IconLibraryPhoto,
  IconPencil,
  IconPhoto,
  IconPlus,
  IconShare,
  IconTrash,
  IconUpload,
  IconUsers,
} from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, type Container, type Item } from "../api";
import { useAuth } from "../auth";
import { canAct, Can, useCan, type Feature, type ObjectType } from "../permissions";
import { confirmDanger, formatBytes, notifyError, PageHeader, run } from "../ui";
import ShareDialog from "./ShareDialog";

/** Names for one product. Documents and photos share this page. */
export type Kind = {
  title: string;
  container: "folder" | "album";
  item: "document" | "photo";
  containerRef: "folder_id" | "album_id";
  /** Feature that allows creating a top-level container. */
  createFeature: Feature;
  /** Feature that allows creating a private container (internal members). */
  createPrivateFeature: Feature;
  defaultMime: string;
};

type Target = { type: ObjectType; id: string; name: string };

/** One modal at a time: create a container, add an item, or rename. */
type Dialog =
  | { mode: "container" }
  | { mode: "item"; containerId: string; containerName: string }
  | { mode: "rename"; target: Target }
  | null;

export default function ContentPage({ kind }: { kind: Kind }) {
  const { me, tenantId } = useAuth();
  const [containers, setContainers] = useState<Container[] | null>(null);
  const [items, setItems] = useState<Item[]>([]);
  const [share, setShare] = useState<Target | null>(null);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const canPrivate = useCan(kind.createPrivateFeature);

  const load = useCallback(async () => {
    if (!me || !tenantId) return;
    try {
      // Each row carries the caller's permissions, computed server-side.
      const [cs, its] = await Promise.all([api<Container[]>("GET", `/${kind.container}s`), api<Item[]>("GET", `/${kind.item}s`)]);
      setContainers(cs);
      setItems(its);
    } catch (e) {
      notifyError(e);
      setContainers([]);
    }
  }, [me, tenantId, kind]);

  useEffect(() => {
    setContainers(null);
    void load();
  }, [load]);

  const act = async (fn: () => Promise<unknown>, success: string) => {
    const ok = await run(fn, success);
    if (ok) await load();
    return ok;
  };

  if (!tenantId) {
    return <EmptyState mt="xl" icon={<IconUsers size={24} />} title="No tenant selected" description="Pick a tenant in the header to see its content." />;
  }
  if (containers === null) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }

  const ContainerIcon = kind.container === "folder" ? IconFolder : IconLibraryPhoto;
  const ItemIcon = kind.item === "document" ? IconFile : IconPhoto;
  const visible = new Set(containers.map((c) => c.id));
  const roots = containers.filter((c) => !c.parent_id || !visible.has(c.parent_id));
  const shared = items.filter((it) => !visible.has(it[kind.containerRef] as string));
  const empty = containers.length === 0 && items.length === 0;

  const toggle = (id: string) =>
    setCollapsed((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const remove = (t: Target) =>
    confirmDanger(`Delete ${t.type} “${t.name}”?`, "Its tuples and grants are deleted. This cannot be undone.", "Delete", () => {
      void act(() => api("DELETE", `/${t.type}s/${t.id}`), `Deleted ${t.name}`);
    });

  const byId = new Map(containers.map((c) => [c.id, c]));
  /** True when a private ancestor hides this container from externals despite its own public flag. */
  const hiddenByParent = (c: Container) => {
    for (let p = c.parent_id ? byId.get(c.parent_id) : undefined; p; p = p.parent_id ? byId.get(p.parent_id) : undefined) {
      if (p.is_private) return true;
    }
    return false;
  };

  const setVisibility = (c: Container, isPrivate: boolean) => {
    const apply = () =>
      void act(() => api("PUT", `/${kind.container}s/${c.id}/visibility`, { is_private: isPrivate }), `${c.name} is now ${isPrivate ? "private" : "public"}`);
    if (isPrivate) return apply();
    confirmDanger(
      `Make “${c.name}” public?`,
      `All external members of this tenant will be able to see it and everything inside, and external admins will be able to add and edit ${kind.item}s.`,
      "Make public",
      apply,
    );
  };

  /** Row menu built from the object's permissions; returns null when nothing is allowed. */
  const actions = (obj: Container | Item, t: Target) => {
    const container = "is_private" in obj ? obj : null;
    const entries = [
      container && canAct(container, t.type, "visibility") && (
        <Menu.Item
          key="visibility"
          leftSection={container.is_private ? <IconEye size={16} /> : <IconEyeOff size={16} />}
          onClick={() => setVisibility(container, !container.is_private)}
        >
          {container.is_private ? "Make public" : "Make private"}
        </Menu.Item>
      ),
      canAct(obj, t.type, "rename") && (
        <Menu.Item key="rename" leftSection={<IconPencil size={16} />} onClick={() => setDialog({ mode: "rename", target: t })}>
          Rename
        </Menu.Item>
      ),
      canAct(obj, t.type, "share") && (
        <Menu.Item key="share" leftSection={<IconShare size={16} />} onClick={() => setShare(t)}>
          Share
        </Menu.Item>
      ),
      canAct(obj, t.type, "delete") && (
        <Menu.Item key="delete" color="red" leftSection={<IconTrash size={16} />} onClick={() => remove(t)}>
          Delete
        </Menu.Item>
      ),
    ].filter(Boolean);
    if (entries.length === 0) return null;
    return (
      <Menu position="bottom-end" withinPortal shadow="md">
        <Menu.Target>
          <ActionIcon variant="subtle" color="gray" aria-label={`Actions for ${t.name}`}>
            <IconDots size={16} />
          </ActionIcon>
        </Menu.Target>
        <Menu.Dropdown>{entries}</Menu.Dropdown>
      </Menu>
    );
  };

  const itemRow = (it: Item) => (
    <Group key={it.id} justify="space-between" wrap="nowrap" px="xs" py={6} className="row-hover" style={{ borderRadius: 8 }}>
      <Group gap="sm" wrap="nowrap" style={{ minWidth: 0 }}>
        <ThemeIcon variant="light" color="gray" size="md">
          <ItemIcon size={16} />
        </ThemeIcon>
        <Box style={{ minWidth: 0 }}>
          <Text size="sm" fw={500} truncate>
            {it.name}
          </Text>
          <Text size="xs" c="dimmed">
            {it.mime_type} · {formatBytes(it.byte_size)}
          </Text>
        </Box>
      </Group>
      {actions(it, { type: kind.item, id: it.id, name: it.name })}
    </Group>
  );

  const node = (c: Container) => {
    const children = containers.filter((x) => x.parent_id === c.id);
    const own = items.filter((it) => it[kind.containerRef] === c.id);
    const open = !collapsed.has(c.id);
    return (
      <Box key={c.id}>
        <Group justify="space-between" wrap="nowrap" px="xs" py={6} className="row-hover" style={{ borderRadius: 8 }}>
          <Group gap="sm" wrap="nowrap" style={{ cursor: "pointer", minWidth: 0 }} onClick={() => toggle(c.id)}>
            {open ? <IconChevronDown size={14} /> : <IconChevronRight size={14} />}
            <ThemeIcon variant="light" size="md">
              <ContainerIcon size={16} />
            </ThemeIcon>
            <Text fw={600} size="sm" truncate>
              {c.name}
            </Text>
            <Badge size="xs" variant="light" color="gray">
              {own.length}
            </Badge>
            <VisibilityBadge container={c} hidden={hiddenByParent(c)} />
          </Group>
          <Group gap={4} wrap="nowrap">
            {canAct(c, kind.container, "addItem") && (
              <Button
                size="compact-xs"
                variant="light"
                leftSection={<IconUpload size={12} />}
                onClick={() => setDialog({ mode: "item", containerId: c.id, containerName: c.name })}
              >
                Add {kind.item}
              </Button>
            )}
            {actions(c, { type: kind.container, id: c.id, name: c.name })}
          </Group>
        </Group>
        <Collapse expanded={open}>
          {(own.length > 0 || children.length > 0) && (
            <Box className="content-tree">
              {own.map(itemRow)}
              {children.map(node)}
            </Box>
          )}
        </Collapse>
      </Box>
    );
  };

  return (
    <>
      <PageHeader
        title={kind.title}
        description={`${kind.container[0].toUpperCase()}${kind.container.slice(1)}s and ${kind.item}s you can see through your relationships.`}
        actions={
          <Can feature={kind.createFeature}>
            <Button leftSection={<IconPlus size={16} />} onClick={() => setDialog({ mode: "container" })}>
              New {kind.container}
            </Button>
          </Can>
        }
      />

      {empty ? (
        <Card withBorder>
          <EmptyState icon={<ContainerIcon size={24} />} title={`No ${kind.item}s`} description="Nothing here is visible with the current relationships." />
        </Card>
      ) : (
        <Stack>
          {roots.length > 0 && (
            <Card withBorder padding="sm">
              <Stack gap={2}>{roots.map(node)}</Stack>
            </Card>
          )}
          {shared.length > 0 && (
            <Card withBorder padding="sm">
              <Group gap="xs" px="xs" mb={4}>
                <IconShare size={16} />
                <Title order={5}>Shared with me</Title>
              </Group>
              <Text size="xs" c="dimmed" px="xs" mb="xs">
                Visible through a direct grant even though the parent {kind.container} is hidden.
              </Text>
              <Stack gap={2}>{shared.map(itemRow)}</Stack>
            </Card>
          )}
        </Stack>
      )}

      <NameModal
        kind={kind}
        dialog={dialog}
        parents={containers.filter((c) => canAct(c, kind.container, "addChild"))}
        canPrivate={canPrivate}
        onClose={() => setDialog(null)}
        act={act}
      />

      {share && (
        <ShareDialog
          objectType={share.type}
          objectId={share.id}
          objectName={share.name}
          onClose={() => {
            setShare(null);
            void load();
          }}
        />
      )}
    </>
  );
}

function VisibilityBadge({ container, hidden }: { container: Container; hidden: boolean }) {
  if (container.is_private) {
    return (
      <Tooltip label="Internal members only, plus anyone it is shared with">
        <Badge size="xs" variant="light" color="gray" leftSection={<IconLock size={10} />}>
          Private
        </Badge>
      </Tooltip>
    );
  }
  if (hidden) {
    return (
      <Tooltip label="Marked public, but a private parent hides it from external members">
        <Badge size="xs" variant="light" color="yellow" leftSection={<IconWorld size={10} />}>
          Public · hidden by parent
        </Badge>
      </Tooltip>
    );
  }
  return (
    <Tooltip label="Visible to all external members">
      <Badge size="xs" variant="light" color="teal" leftSection={<IconWorld size={10} />}>
        Public
      </Badge>
    </Tooltip>
  );
}

function NameModal({
  kind,
  dialog,
  parents,
  canPrivate,
  onClose,
  act,
}: {
  kind: Kind;
  dialog: Dialog;
  /** Containers the actor may create a child in. */
  parents: Container[];
  canPrivate: boolean;
  onClose: () => void;
  act: (fn: () => Promise<unknown>, success: string) => Promise<boolean>;
}) {
  const [name, setName] = useState("");
  const [parent, setParent] = useState<string | null>(null);
  const [isPrivate, setIsPrivate] = useState(canPrivate);

  useEffect(() => {
    setName(dialog?.mode === "rename" ? dialog.target.name : "");
    setParent(null);
    // Default to private when allowed: the safe choice. Others can only create public.
    setIsPrivate(canPrivate);
  }, [dialog, canPrivate]);

  if (!dialog) return <Modal opened={false} onClose={onClose} />;

  const title =
    dialog.mode === "container" ? `New ${kind.container}` : dialog.mode === "item" ? `Add ${kind.item} to ${dialog.containerName}` : `Rename ${dialog.target.type}`;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const n = name.trim();
    let ok = false;
    if (dialog.mode === "container") {
      ok = await act(() => api("POST", `/${kind.container}s`, { name: n, parent_id: parent, is_private: isPrivate }), `Created ${n}`);
    } else if (dialog.mode === "item") {
      // Metadata only: the demo stores name, type, and size, never file bytes.
      ok = await act(
        () => api("POST", `/${kind.item}s`, { name: n, mime_type: kind.defaultMime, byte_size: 1024, [kind.containerRef]: dialog.containerId }),
        `Added ${n}`,
      );
    } else {
      ok = await act(() => api("PATCH", `/${dialog.target.type}s/${dialog.target.id}`, { name: n }), "Renamed");
    }
    if (ok) onClose();
  };

  return (
    <Modal opened onClose={onClose} title={title}>
      <form onSubmit={submit}>
        <Stack>
          <TextInput label="Name" data-autofocus required value={name} onChange={(e) => setName(e.currentTarget.value)} />
          {dialog.mode === "container" && (
            <Select
              label={`Parent ${kind.container}`}
              placeholder="None (top level)"
              clearable
              value={parent}
              onChange={setParent}
              data={parents.map((c) => ({ value: c.id, label: c.name }))}
            />
          )}
          {dialog.mode === "container" &&
            (canPrivate ? (
              <Switch
                label="Private"
                description={
                  isPrivate
                    ? "Only internal members can see it, unless you share it."
                    : `All external members can see it; external admins can add ${kind.item}s.`
                }
                checked={isPrivate}
                onChange={(e) => setIsPrivate(e.currentTarget.checked)}
              />
            ) : (
              <Text size="xs" c="dimmed">
                This {kind.container} will be public: visible to all external members of the tenant.
              </Text>
            ))}
          {dialog.mode === "item" && (
            <Text size="xs" c="dimmed">
              Uploads store metadata only ({kind.defaultMime}, 1 KB). The owner tuple goes to you.
            </Text>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim()}>
              {dialog.mode === "rename" ? "Save" : "Create"}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
