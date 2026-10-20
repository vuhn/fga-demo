import { Badge, Button, Card, Group, Modal, SegmentedControl, Stack, Table, Text, TextInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconPlus, IconShield } from "@tabler/icons-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, CUSTOM_PERMISSIONS, type CustomRole } from "../api";
import { RequireFeature, usePermissionSet } from "../permissions";
import { AffiliationBadge, notifyError, PageHeader } from "../ui";

const permLabel = Object.fromEntries(CUSTOM_PERMISSIONS.map((p) => [p.value, p.label]));

export default function Roles() {
  const { id = "" } = useParams();
  const loaded = usePermissionSet(id);
  return (
    <RequireFeature feature="tenant.roles.manage" set={loaded}>
      <RolesPage id={id} />
    </RequireFeature>
  );
}

function RolesPage({ id }: { id: string }) {
  const navigate = useNavigate();
  const [roles, setRoles] = useState<CustomRole[] | null>(null);
  const [opened, { open, close }] = useDisclosure();
  const [name, setName] = useState("");
  const [affiliation, setAffiliation] = useState("internal");

  const load = useCallback(() => {
    api<CustomRole[]>("GET", `/tenants/${id}/roles`).then(setRoles, notifyError);
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    try {
      const created = await api<CustomRole>("POST", `/tenants/${id}/roles`, { name, affiliation });
      close();
      setName("");
      navigate(`/tenants/${id}/roles/${created.id}`);
    } catch (err) {
      notifyError(err);
    }
  };

  return (
    <>
      <PageHeader
        title="Roles"
        description="Custom roles for this tenant. Each one is internal or external, and is assigned from the member list like a built-in role."
        actions={
          <Group>
            <Button component={Link} to={`/tenants/${id}`} variant="default">
              Back to tenant
            </Button>
            <Button leftSection={<IconPlus size={16} />} onClick={open}>
              New role
            </Button>
          </Group>
        }
      />
      <Card withBorder padding={0}>
        <Table.ScrollContainer minWidth={640}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Role</Table.Th>
                <Table.Th>Type</Table.Th>
                <Table.Th>Permissions</Table.Th>
                <Table.Th>Members</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {(roles ?? []).map((role) => (
                <Table.Tr key={role.id} style={{ cursor: "pointer" }} onClick={() => navigate(`/tenants/${id}/roles/${role.id}`)}>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <IconShield size={16} />
                      <Text fw={500}>{role.name}</Text>
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    <AffiliationBadge affiliation={role.affiliation} />
                  </Table.Td>
                  <Table.Td>
                    <Group gap={4}>
                      {role.permissions.length === 0 && (
                        <Text size="sm" c="dimmed">
                          None
                        </Text>
                      )}
                      {role.permissions.map((p) => (
                        <Badge key={p} variant="light" size="sm">
                          {permLabel[p] ?? p}
                        </Badge>
                      ))}
                    </Group>
                  </Table.Td>
                  <Table.Td>{role.assignees.length}</Table.Td>
                </Table.Tr>
              ))}
              {roles?.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={4}>
                    <Text size="sm" c="dimmed" py="sm">
                      No custom roles yet.
                    </Text>
                  </Table.Td>
                </Table.Tr>
              )}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Card>
      <Modal opened={opened} onClose={close} title="New custom role">
        <form onSubmit={create}>
          <Stack>
            <TextInput label="Name" value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Auditor" data-autofocus />
            <div>
              <Text size="sm" fw={500} mb={4}>
                Type
              </Text>
              <SegmentedControl
                fullWidth
                value={affiliation}
                onChange={setAffiliation}
                data={[
                  { value: "internal", label: "Internal" },
                  { value: "external", label: "External" },
                ]}
              />
              <Text size="xs" c="dimmed" mt={4}>
                Fixed once created. An internal role can only be given to internal members.
              </Text>
            </div>
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>
                Cancel
              </Button>
              <Button type="submit" disabled={!name.trim()}>
                Create
              </Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
