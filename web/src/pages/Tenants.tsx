import { Badge, Button, Card, Group, Modal, Stack, Table, Text, TextInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconChevronRight, IconPlus } from "@tabler/icons-react";
import { useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Tenant } from "../api";
import { notifyError, PageHeader, run } from "../ui";

export default function Tenants() {
  const navigate = useNavigate();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [opened, { open, close }] = useDisclosure();
  const [form, setForm] = useState({ name: "", slug: "" });

  const load = () => api<Tenant[]>("GET", "/tenants").then(setTenants, notifyError);
  useEffect(() => {
    void load();
  }, []);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("POST", "/tenants", form), `Created ${form.name}`)) {
      setForm({ name: "", slug: "" });
      close();
      await load();
    }
  };

  return (
    <>
      <PageHeader
        title="Tenant directory"
        description="Every tenant on the platform. A system admin manages products and members here but is not a member."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={open}>
            New tenant
          </Button>
        }
      />
      <Card withBorder padding={0}>
        <Table.ScrollContainer minWidth={520}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Slug</Table.Th>
                <Table.Th>Products</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {tenants.map((t) => (
                <Table.Tr key={t.id} style={{ cursor: "pointer" }} onClick={() => navigate(`/tenants/${t.id}`)}>
                  <Table.Td fw={500}>{t.name}</Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {t.slug}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Group gap={6}>
                      {t.products.map((p) => (
                        <Badge key={p.product} variant="light" color={p.enabled ? "teal" : "gray"}>
                          {p.product} {p.enabled ? "on" : "off"}
                        </Badge>
                      ))}
                    </Group>
                  </Table.Td>
                  <Table.Td ta="right">
                    <IconChevronRight size={16} />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Card>

      <Modal opened={opened} onClose={close} title="New tenant">
        <form onSubmit={create}>
          <Stack>
            <TextInput label="Name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.currentTarget.value })} />
            <TextInput
              label="Slug"
              description="Lowercase letters, digits, and dashes"
              required
              value={form.slug}
              onChange={(e) => setForm({ ...form, slug: e.currentTarget.value })}
            />
            <Text size="xs" c="dimmed">
              New tenants start with both products disabled, so no wildcard tuple exists yet.
            </Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>
                Cancel
              </Button>
              <Button type="submit">Create tenant</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
