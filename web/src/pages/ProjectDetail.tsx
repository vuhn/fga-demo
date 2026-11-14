import { ActionIcon, Tabs, Button, Card, Center, Checkbox, Group, Loader, Modal, Select, Stack, Table, Text, TextInput, Title, Tooltip } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconTrash, IconUserPlus } from "@tabler/icons-react";
import { lazy, Suspense, useCallback, useEffect, useState, type FormEvent } from "react";

import { useParams, useSearchParams } from "react-router-dom";
import { api, CUSTOM_PERMISSIONS, memberRoleOptions, type Member, type Project, type ProjectMember, type ProjectRole } from "../api";
import { useAuth } from "../auth";
import { canProject, Can, useCan } from "../permissions";
import { AffiliationBadge, CompanyBadge, confirmDanger, notifyError, PageHeader, RoleBadge, run } from "../ui";

const Documents = lazy(() => import("./Documents"));
const Photos = lazy(() => import("./Photos"));
const Blueprints = lazy(() => import("./Blueprints"));

/**
 * Project page: documents and photos of the project, plus its members and roles. A member's effective permissions in the project
 * are the union of their tenant role and their project role. Only tenant
 * members can be added, and a role must match the member's affiliation.
 */
export default function ProjectDetail() {
  const { id = "" } = useParams();
  const { me } = useAuth();
  const [project, setProject] = useState<Project | null>(null);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [roles, setRoles] = useState<ProjectRole[]>([]);
  const [adding, { open: openAdd, close: closeAdd }] = useDisclosure();
  const [creating, { open: openRole, close: closeRole }] = useDisclosure();
  const canRoles = useCan("project.roles.manage");
  const docs = useCan("nav.documents");
  const photos = useCan("nav.photos");
  const blueprints = useCan("nav.blueprints");
  const [params, setParams] = useSearchParams();

  const load = useCallback(async () => {
    if (!me) return;
    try {
      const p = await api<Project>("GET", `/projects/${id}`);
      setProject(p);
      if (canProject(p, "manageMembers")) {
        const [ms, rs] = await Promise.all([api<ProjectMember[]>("GET", `/projects/${id}/members`), api<ProjectRole[]>("GET", `/projects/${id}/roles`)]);
        setMembers(ms);
        setRoles(rs);
      } else {
        setMembers([]);
        setRoles([]);
      }
    } catch (e) {
      notifyError(e);
    }
  }, [id, me]);

  useEffect(() => {
    void load();
  }, [load]);

  const act = async (fn: () => Promise<unknown>, success: string) => {
    const ok = await run(fn, success);
    if (ok) await load();
    return ok;
  };

  if (!project) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }
  const manage = canProject(project, "manageMembers");
  const visible = [docs && "documents", photos && "photos", blueprints && "blueprints", "members"].filter(Boolean) as string[];
  const requested = params.get("tab");
  const tab = requested && visible.includes(requested) ? requested : visible[0];

  return (
    <>
      <PageHeader title={project.name} description="Documents, photos, blueprints, and members of this project. Member permissions add to their tenant role." />
      <Tabs
        value={tab}
        onChange={(v) => setParams(v ? { tab: v } : {}, { replace: true })}
        keepMounted={false}
      >
        <Tabs.List mb="md">
          {docs && <Tabs.Tab value="documents">Documents</Tabs.Tab>}
          {photos && <Tabs.Tab value="photos">Photos</Tabs.Tab>}
          {blueprints && <Tabs.Tab value="blueprints">Blueprints</Tabs.Tab>}
          <Tabs.Tab value="members">Members &amp; roles</Tabs.Tab>
        </Tabs.List>
        {docs && (
          <Tabs.Panel value="documents">
            <Suspense fallback={<Loader />}>
              <Documents project={project} />
            </Suspense>
          </Tabs.Panel>
        )}
        {photos && (
          <Tabs.Panel value="photos">
            <Suspense fallback={<Loader />}>
              <Photos project={project} />
            </Suspense>
          </Tabs.Panel>
        )}
        {blueprints && (
          <Tabs.Panel value="blueprints">
            <Suspense fallback={<Loader />}>
              <Blueprints project={project} />
            </Suspense>
          </Tabs.Panel>
        )}
        <Tabs.Panel value="members">
          {manage && (
            <Group justify="flex-end" mb="md">
              <Button leftSection={<IconUserPlus size={16} />} onClick={openAdd}>
                Add member
              </Button>
            </Group>
          )}
      {!manage ? (
        <Text c="dimmed">Members are visible to project and tenant admins.</Text>
      ) : (
        <>
          <Card withBorder padding={0} mb="xl">
            <Table verticalSpacing="sm" highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Member</Table.Th>
                  <Table.Th>Company</Table.Th>
                  <Table.Th>Affiliation</Table.Th>
                  <Table.Th>Project role</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {members.map((m) => (
                  <Table.Tr key={m.user_id}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {m.name}
                      </Text>
                      <Text size="xs" c="dimmed">
                        {m.email}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <CompanyBadge company={m.company} affiliation={m.affiliation} />
                    </Table.Td>
                    <Table.Td>
                      <AffiliationBadge affiliation={m.affiliation} />
                    </Table.Td>
                    <Table.Td>
                      <Select
                        size="xs"
                        w={200}
                        allowDeselect={false}
                        value={m.role}
                        data={memberRoleOptions(m.affiliation, roles)}
                        onChange={(role) => role && role !== m.role && act(() => api("PATCH", `/projects/${id}/members/${m.user_id}`, { role }), `Updated ${m.name}`)}
                      />
                    </Table.Td>
                    <Table.Td ta="right">
                      <Tooltip label="Remove from project">
                        <ActionIcon
                          variant="subtle"
                          color="red"
                          aria-label={`Remove ${m.name}`}
                          onClick={() =>
                            confirmDanger(
                              `Remove ${m.name} from ${project.name}?`,
                              "Their tenant membership stays; only the project role and access through it are removed.",
                              "Remove",
                              () => void act(() => api("DELETE", `/projects/${id}/members/${m.user_id}`), `Removed ${m.name}`),
                            )
                          }
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Tooltip>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Card>

          <Group justify="space-between" mb="sm">
            <Title order={4}>Project roles</Title>
            <Can feature="project.roles.manage">
              <Button size="xs" variant="default" onClick={openRole}>
                New role
              </Button>
            </Can>
          </Group>
          <Stack>
            {roles.length === 0 && (
              <Text size="sm" c="dimmed">
                No project roles yet. The four built-in roles are always available.
              </Text>
            )}
            {roles.map((r) => (
              <Card key={r.id} withBorder padding="md">
                <Group justify="space-between" mb="xs">
                  <Group gap="xs">
                    <Text fw={600}>{r.name}</Text>
                    <AffiliationBadge affiliation={r.affiliation} />
                    <RoleBadge role="" name={`${r.assignees.length} assigned`} />
                  </Group>
                  {canRoles && (
                    <Button
                      size="compact-xs"
                      color="red"
                      variant="light"
                      onClick={() =>
                        confirmDanger(`Delete role ${r.name}?`, "Its tuples are deleted. A role held by members cannot be deleted.", "Delete", () =>
                          void act(() => api("DELETE", `/projects/${id}/roles/${r.id}`), `Deleted ${r.name}`),
                        )
                      }
                    >
                      Delete
                    </Button>
                  )}
                </Group>
                <Checkbox.Group
                  value={r.permissions}
                  onChange={(permissions) => canRoles && void act(() => api("PUT", `/projects/${id}/roles/${r.id}/permissions`, { permissions }), `Saved ${r.name}`)}
                >
                  <Group gap="md">
                    {CUSTOM_PERMISSIONS.map((p) => (
                      <Checkbox key={p.value} value={p.value} label={p.label} disabled={!canRoles} size="xs" />
                    ))}
                  </Group>
                </Checkbox.Group>
              </Card>
            ))}
          </Stack>
        </>
      )}
        </Tabs.Panel>
      </Tabs>

      <AddProjectMemberModal opened={adding} onClose={closeAdd} projectId={id} roles={roles} onDone={load} />
      <NewRoleModal opened={creating} onClose={closeRole} projectId={id} onDone={load} />
    </>
  );
}

/** Picks from the tenant members who are not in the project yet (a narrow endpoint). */
function AddProjectMemberModal({ opened, onClose, projectId, roles, onDone }: { opened: boolean; onClose: () => void; projectId: string; roles: ProjectRole[]; onDone: () => Promise<void> }) {
  const [candidates, setCandidates] = useState<Member[]>([]);
  const [userId, setUserId] = useState<string | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const person = candidates.find((m) => m.user_id === userId);

  useEffect(() => {
    if (!opened) return;
    setUserId(null);
    setRole(null);
    api<Member[]>("GET", `/projects/${projectId}/member-candidates`).then(setCandidates, notifyError);
  }, [opened, projectId]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (userId && role && (await run(() => api("POST", `/projects/${projectId}/members`, { user_id: userId, role }), "Member added"))) {
      onClose();
      await onDone();
    }
  };

  return (
    <Modal opened={opened} onClose={onClose} title="Add project member">
      <form onSubmit={submit}>
        <Stack>
          <Select
            label="Tenant member"
            placeholder="Choose a member"
            searchable
            value={userId}
            onChange={(v) => {
              setUserId(v);
              const m = candidates.find((c) => c.user_id === v);
              setRole(m ? `${m.affiliation}_user` : null);
            }}
            data={candidates.map((m) => ({ value: m.user_id, label: `${m.name} · ${m.company?.name ?? "?"}` }))}
            nothingFoundMessage="Every tenant member is already in this project"
          />
          {person && (
            <Select
              label={`Role (${person.affiliation})`}
              allowDeselect={false}
              value={role}
              onChange={setRole}
              data={memberRoleOptions(person.affiliation, roles)}
            />
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!userId || !role}>
              Add
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}

function NewRoleModal({ opened, onClose, projectId, onDone }: { opened: boolean; onClose: () => void; projectId: string; onDone: () => Promise<void> }) {
  const [name, setName] = useState("");
  const [affiliation, setAffiliation] = useState<string | null>("external");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("POST", `/projects/${projectId}/roles`, { name: name.trim(), affiliation }), `Created ${name}`)) {
      setName("");
      onClose();
      await onDone();
    }
  };
  return (
    <Modal opened={opened} onClose={onClose} title="New project role">
      <form onSubmit={submit}>
        <Stack>
          <TextInput label="Name" required data-autofocus value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Select
            label="Affiliation"
            description="Internal roles go to internal members only, external roles to external members only."
            allowDeselect={false}
            value={affiliation}
            onChange={setAffiliation}
            data={[
              { value: "internal", label: "Internal" },
              { value: "external", label: "External" },
            ]}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim()}>
              Create
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
