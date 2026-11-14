import { Alert, Badge, Button, Card, Code, Grid, Group, Select, SimpleGrid, Stack, Table, Text, TextInput, Title } from "@mantine/core";
import { IconCheck, IconSearch, IconShieldCheck, IconX } from "@tabler/icons-react";
import { useEffect, useState, type FormEvent } from "react";
import { api, type Member, type Tuple } from "../api";
import { useAuth } from "../auth";
import { useCan } from "../permissions";
import { notifyError, PageHeader } from "../ui";

const RELATIONS: Record<string, string[]> = {
  platform: ["can_manage_platform"],
  tenant: [
    "can_read_tenant",
    "can_contribute",
    "can_manage_members",
    "can_view_products",
    "can_create_folder",
    "can_create_album",
    "can_create_private_folder",
    "can_create_private_album",
    "documents_enabled",
    "photos_enabled",
  ],
  folder: ["can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_document"],
  document: ["can_view", "can_edit", "can_manage", "can_delete", "can_share"],
  album: ["can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_photo"],
  photo: ["can_view", "can_edit", "can_manage", "can_delete", "can_share"],
};

type Access = {
  object: string;
  direct_tuples: Tuple[];
  members: { user_id: string; name: string; can_view: boolean; can_edit: boolean; can_delete: boolean }[];
};

function Allowed({ value }: { value: boolean }) {
  return value ? (
    <Badge color="teal" variant="light" leftSection={<IconCheck size={12} />}>
      allowed
    </Badge>
  ) : (
    <Badge color="red" variant="light" leftSection={<IconX size={12} />}>
      denied
    </Badge>
  );
}

export default function Inspector() {
  const { me, tenantId } = useAuth();
  // Checking other people needs member management; everyone else checks only themselves.
  const canPickMembers = useCan("tenant.members.manage");
  const [members, setMembers] = useState<Member[]>([]);
  const [form, setForm] = useState({ user_id: me?.user.id ?? "", relation: "can_view", object_type: "folder", object_id: "" });
  const [result, setResult] = useState<boolean | null>(null);
  const [lookup, setLookup] = useState({ object_type: "document", object_id: "" });
  const [access, setAccess] = useState<Access | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (tenantId && canPickMembers) api<Member[]>("GET", `/tenants/${tenantId}/members`).then(setMembers, () => setMembers([]));
    else setMembers([]);
  }, [tenantId, canPickMembers]);

  const people = [
    ...(me ? [{ value: me.user.id, label: `${me.user.name} (me)` }] : []),
    ...members.filter((m) => m.user_id !== me?.user.id).map((m) => ({ value: m.user_id, label: m.name })),
  ];

  const check = async (e: FormEvent) => {
    e.preventDefault();
    setResult(null);
    try {
      const r = await api<{ allowed: boolean }>("POST", "/authz/check", form);
      setResult(r.allowed);
    } catch (err) {
      notifyError(err);
    }
  };

  const inspect = async (e: FormEvent) => {
    e.preventDefault();
    setAccess(null);
    setBusy(true);
    try {
      setAccess(await api<Access>("GET", `/access?object_type=${lookup.object_type}&object_id=${lookup.object_id}`));
    } catch (err) {
      notifyError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <PageHeader title="Access inspector" description="Ask OpenFGA directly. You can always check yourself; admins can check members of their tenant." />

      <SimpleGrid cols={{ base: 1, lg: 2 }} mb="lg">
        <Card withBorder padding="lg">
          <Title order={4} mb="md">
            Check
          </Title>
          <form onSubmit={check}>
            <Stack>
              <Select
                label="User"
                searchable
                allowDeselect={false}
                value={form.user_id}
                onChange={(v) => v && setForm({ ...form, user_id: v })}
                data={people}
              />
              <Grid>
                <Grid.Col span={{ base: 12, sm: 5 }}>
                  <Select
                    label="Object type"
                    allowDeselect={false}
                    value={form.object_type}
                    data={Object.keys(RELATIONS)}
                    onChange={(t) =>
                      t &&
                      setForm({
                        ...form,
                        object_type: t,
                        relation: RELATIONS[t][0],
                        object_id: t === "platform" ? "main" : t === "tenant" ? tenantId ?? "" : "",
                      })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 7 }}>
                  <Select
                    label="Relation"
                    allowDeselect={false}
                    value={form.relation}
                    data={RELATIONS[form.object_type]}
                    onChange={(r) => r && setForm({ ...form, relation: r })}
                  />
                </Grid.Col>
              </Grid>
              <TextInput label="Object id" placeholder="UUID" value={form.object_id} onChange={(e) => setForm({ ...form, object_id: e.currentTarget.value })} />
              <Group justify="space-between">
                <Button type="submit" leftSection={<IconShieldCheck size={16} />}>
                  Check
                </Button>
                {result !== null && <Allowed value={result} />}
              </Group>
            </Stack>
          </form>
        </Card>

        <Card withBorder padding="lg">
          <Title order={4} mb="md">
            Object access
          </Title>
          <form onSubmit={inspect}>
            <Stack>
              <Select
                label="Object type"
                allowDeselect={false}
                value={lookup.object_type}
                data={["folder", "document", "album", "photo"]}
                onChange={(t) => t && setLookup({ ...lookup, object_type: t })}
              />
              <TextInput label="Object id" placeholder="UUID" value={lookup.object_id} onChange={(e) => setLookup({ ...lookup, object_id: e.currentTarget.value })} />
              <Alert variant="light" color="gray" p="xs">
                <Text size="xs">Requires can_share on the object, can_manage_members on its tenant, or platform admin.</Text>
              </Alert>
              <Button type="submit" variant="light" leftSection={<IconSearch size={16} />} loading={busy}>
                Inspect
              </Button>
            </Stack>
          </form>
        </Card>
      </SimpleGrid>

      {access && (
        <SimpleGrid cols={{ base: 1, lg: 2 }}>
          <Card withBorder padding={0}>
            <Text fw={600} p="md">
              Direct tuples on <Code>{access.object}</Code>
            </Text>
            <Table.ScrollContainer minWidth={420}>
              <Table verticalSpacing="xs">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>User</Table.Th>
                    <Table.Th>Relation</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {access.direct_tuples.map((t) => (
                    <Table.Tr key={`${t.user}|${t.relation}`}>
                      <Table.Td>
                        <Code fz="xs">{t.user}</Code>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="outline">{t.relation}</Badge>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          </Card>
          <Card withBorder padding={0}>
            <Text fw={600} p="md">
              Member checks
            </Text>
            <Table.ScrollContainer minWidth={420}>
              <Table verticalSpacing="xs">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Name</Table.Th>
                    <Table.Th>can_view</Table.Th>
                    <Table.Th>can_edit</Table.Th>
                    <Table.Th>can_delete</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {access.members.map((m) => (
                    <Table.Tr key={m.user_id}>
                      <Table.Td fw={500}>{m.name}</Table.Td>
                      {[m.can_view, m.can_edit, m.can_delete].map((v, i) => (
                        <Table.Td key={i}>{v ? <IconCheck size={16} color="var(--mantine-color-teal-6)" /> : <IconX size={16} color="var(--mantine-color-red-6)" />}</Table.Td>
                      ))}
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          </Card>
        </SimpleGrid>
      )}
    </>
  );
}
