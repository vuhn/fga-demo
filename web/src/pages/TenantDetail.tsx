import {
  ActionIcon,
  Avatar,
  Badge,
  Button,
  Card,
  Center,
  EmptyState,
  Group,
  Loader,
  Modal,
  SegmentedControl,
  Select,
  SimpleGrid,
  Stack,
  Switch,
  Table,
  Text,
  TextInput,
  ThemeIcon,
  Title,
  Tooltip,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconFileText, IconPhoto, IconTrash, IconUserPlus, IconUsers } from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";
import { api, ROLES, type Member, type Product, type Tenant, type User } from "../api";
import { useAuth } from "../auth";
import { allows, RequireFeature, usePermissionSet } from "../permissions";
import { AffiliationBadge, confirmDanger, notifyError, PageHeader, RoleBadge, run } from "../ui";

// Permissions are evaluated for the tenant in the URL, which may differ from
// the active tenant (a system admin opens any tenant from the directory).
export default function TenantDetail() {
  const { id = "" } = useParams();
  const loaded = usePermissionSet(id);
  return (
    <RequireFeature feature="tenant.admin" set={loaded}>
      <TenantPage
        id={id}
        perms={{
          products: allows(loaded.set, "tenant.products.manage"),
          members: allows(loaded.set, "tenant.members.manage"),
          directory: allows(loaded.set, "nav.users"),
        }}
      />
    </RequireFeature>
  );
}

type PagePerms = { products: boolean; members: boolean; directory: boolean };

const PRODUCT_META: Record<string, { label: string; icon: typeof IconFileText }> = {
  documents: { label: "Documents", icon: IconFileText },
  photos: { label: "Photos", icon: IconPhoto },
};

function TenantPage({ id, perms }: { id: string; perms: PagePerms }) {
  const { me, refresh } = useAuth();
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [filter, setFilter] = useState("all");
  const [users, setUsers] = useState<User[] | null>(null);
  const [adding, { open: openAdd, close: closeAdd }] = useDisclosure();

  const load = useCallback(async () => {
    if (!me) return;
    try {
      const aff = filter === "all" ? "" : `?affiliation=${filter}`;
      const [t, ps, ms] = await Promise.all([
        api<Tenant>("GET", `/tenants/${id}`),
        api<Product[]>("GET", `/tenants/${id}/products`),
        api<Member[]>("GET", `/tenants/${id}/members${aff}`),
      ]);
      setTenant(t);
      setProducts(ps);
      setMembers(ms);
      // Only someone who can manage the platform can list identities; others invite by user id.
      if (perms.directory) api<User[]>("GET", "/users").then(setUsers, () => setUsers(null));
      else setUsers(null);
    } catch (e) {
      notifyError(e);
    }
  }, [id, filter, me, perms.directory]);

  useEffect(() => {
    void load();
  }, [load]);

  // Refresh /me afterwards so menus and permissions reflect the change.
  const act = async (fn: () => Promise<unknown>, success: string) => {
    if (await run(fn, success)) {
      await load();
      await refresh();
      return true;
    }
    return false;
  };

  if (!tenant) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }

  return (
    <>
      <PageHeader title={tenant.name} description={`Tenant · ${tenant.slug}`} />

      <Group justify="space-between" align="baseline" mb="sm">
        <Title order={4}>Products</Title>
        {!perms.products && (
          <Text size="xs" c="dimmed">
            Products are enabled or disabled by the platform admin.
          </Text>
        )}
      </Group>
      <SimpleGrid cols={{ base: 1, sm: 2 }} mb="xl">
        {products.map((p) => {
          const meta = PRODUCT_META[p.product];
          return (
            <Card key={p.product} withBorder padding="lg">
              <Group justify="space-between" wrap="nowrap">
                <Group wrap="nowrap">
                  <ThemeIcon size="lg" variant="light" color={p.enabled ? "teal" : "gray"}>
                    <meta.icon size={20} />
                  </ThemeIcon>
                  <div>
                    <Text fw={600}>{meta.label}</Text>
                    <Text size="xs" c="dimmed">
                      tenant#{p.product}_enabled@user:*
                    </Text>
                  </div>
                </Group>
                {perms.products ? (
                  <Switch
                    size="md"
                    checked={p.enabled}
                    onChange={() =>
                      act(
                        () => api("PUT", `/tenants/${id}/products/${p.product}`, { enabled: !p.enabled }),
                        `${meta.label} ${p.enabled ? "disabled" : "enabled"}`,
                      )
                    }
                    aria-label={`Toggle ${meta.label}`}
                  />
                ) : (
                  <Badge variant="light" color={p.enabled ? "teal" : "gray"}>
                    {p.enabled ? "enabled" : "disabled"}
                  </Badge>
                )}
              </Group>
            </Card>
          );
        })}
      </SimpleGrid>

      <Card withBorder padding={0}>
        <Group justify="space-between" p="md" wrap="wrap">
          <Title order={4}>Members</Title>
          <Group gap="sm">
            <SegmentedControl
              size="xs"
              value={filter}
              onChange={setFilter}
              data={[
                { value: "all", label: "All" },
                { value: "internal", label: "Internal" },
                { value: "external", label: "External" },
              ]}
            />
            {perms.members && (
              <Button size="xs" leftSection={<IconUserPlus size={14} />} onClick={openAdd}>
                Add member
              </Button>
            )}
          </Group>
        </Group>
        {members.length === 0 ? (
          <EmptyState mb="lg" icon={<IconUsers size={24} />} title="No members" description="Nothing here is visible with the current relationships." />
        ) : (
          <Table.ScrollContainer minWidth={560}>
            <Table verticalSpacing="sm" highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Member</Table.Th>
                  <Table.Th>Affiliation</Table.Th>
                  <Table.Th>Role</Table.Th>
                  {perms.members && <Table.Th />}
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {members.map((m) => (
                  <Table.Tr key={m.user_id}>
                    <Table.Td>
                      <Group gap="sm" wrap="nowrap">
                        <Avatar size="sm" radius="xl" color={m.affiliation === "internal" ? "indigo" : "orange"}>
                          {m.name[0]}
                        </Avatar>
                        <div>
                          <Text size="sm" fw={500}>
                            {m.name}
                          </Text>
                          <Text size="xs" c="dimmed">
                            {m.email}
                          </Text>
                        </div>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <AffiliationBadge affiliation={m.affiliation} />
                    </Table.Td>
                    <Table.Td>
                      {perms.members ? (
                        <Select
                          size="xs"
                          w={170}
                          allowDeselect={false}
                          value={m.role}
                          data={ROLES[m.affiliation]}
                          onChange={(role) =>
                            role && role !== m.role && act(() => api("PATCH", `/tenants/${id}/members/${m.user_id}`, { role }), `${m.name} is now ${role}`)
                          }
                        />
                      ) : (
                        <RoleBadge role={m.role} />
                      )}
                    </Table.Td>
                    {perms.members && (
                      <Table.Td ta="right">
                        <Tooltip label="Remove from tenant">
                          <ActionIcon
                            variant="subtle"
                            color="red"
                            aria-label={`Remove ${m.name}`}
                            onClick={() =>
                              confirmDanger(
                                `Remove ${m.name}?`,
                                "Their role tuple and every direct grant in this tenant are deleted. Library usersets stay; they simply drop out of them.",
                                "Remove",
                                () => void act(() => api("DELETE", `/tenants/${id}/members/${m.user_id}`), `Removed ${m.name}`),
                              )
                            }
                          >
                            <IconTrash size={16} />
                          </ActionIcon>
                        </Tooltip>
                      </Table.Td>
                    )}
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </Card>

      <AddMemberModal
        opened={adding}
        onClose={closeAdd}
        users={users}
        members={members}
        onAdd={async (b) => {
          if (await act(() => api("POST", `/tenants/${id}/members`, b), "Member added")) closeAdd();
        }}
      />
    </>
  );
}

function AddMemberModal({
  opened,
  onClose,
  users,
  members,
  onAdd,
}: {
  opened: boolean;
  onClose: () => void;
  users: User[] | null;
  members: Member[];
  onAdd: (b: { user_id: string; affiliation: string; role: string }) => void;
}) {
  const [affiliation, setAffiliation] = useState("internal");
  const [role, setRole] = useState("internal_user");
  const [userId, setUserId] = useState("");
  const candidates = users?.filter((u) => !members.some((m) => m.user_id === u.id)) ?? [];

  const submit = (e: FormEvent) => {
    e.preventDefault();
    onAdd({ user_id: userId.trim(), affiliation, role });
  };

  return (
    <Modal opened={opened} onClose={onClose} title="Add member">
      <form onSubmit={submit}>
        <Stack>
          <div>
            <Text size="sm" fw={500} mb={4}>
              Affiliation
            </Text>
            <SegmentedControl
              fullWidth
              value={affiliation}
              onChange={(v) => {
                setAffiliation(v);
                setRole(ROLES[v][1]);
              }}
              data={[
                { value: "internal", label: "Internal (staff)" },
                { value: "external", label: "External (guest)" },
              ]}
            />
            <Text size="xs" c="dimmed" mt={4}>
              Fixed once added. Roles never cross affiliation.
            </Text>
          </div>
          <Select label="Role" allowDeselect={false} value={role} onChange={(v) => v && setRole(v)} data={ROLES[affiliation]} />
          {users ? (
            <Select
              label="User"
              placeholder="Choose a user"
              searchable
              value={userId || null}
              onChange={(v) => setUserId(v ?? "")}
              data={candidates.map((u) => ({ value: u.id, label: `${u.name} (${u.email})` }))}
              nothingFoundMessage="Everyone is already a member"
            />
          ) : (
            <TextInput label="User id" description="Ask a system admin for the user's UUID" value={userId} onChange={(e) => setUserId(e.currentTarget.value)} />
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!userId}>
              Add member
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
