import { Avatar, Badge, Button, Card, Code, Group, Modal, PasswordInput, Stack, Table, Text, TextInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconPlus } from "@tabler/icons-react";
import { useEffect, useState, type FormEvent } from "react";
import { api, type User } from "../api";
import { notifyError, PageHeader, run } from "../ui";

export default function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [opened, { open, close }] = useDisclosure();
  const [form, setForm] = useState({ email: "", name: "", password: "" });

  const load = () => api<User[]>("GET", "/users").then(setUsers, notifyError);
  useEffect(() => {
    void load();
  }, []);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("POST", "/users", form), `Created ${form.name}`)) {
      setForm({ email: "", name: "", password: "" });
      close();
      await load();
    }
  };

  return (
    <>
      <PageHeader
        title="Users"
        description="Identities on the platform. Creating a user writes no tuple until someone adds them to a tenant."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={open}>
            New user
          </Button>
        }
      />
      <Card withBorder padding={0}>
        <Table.ScrollContainer minWidth={600}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Email</Table.Th>
                <Table.Th>Platform</Table.Th>
                <Table.Th>User id</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {users.map((u) => (
                <Table.Tr key={u.id}>
                  <Table.Td>
                    <Group gap="sm">
                      <Avatar size="sm" radius="xl" color="indigo">
                        {u.name[0]}
                      </Avatar>
                      <Text size="sm" fw={500}>
                        {u.name}
                      </Text>
                    </Group>
                  </Table.Td>
                  <Table.Td>{u.email}</Table.Td>
                  <Table.Td>{u.is_system_admin && <Badge variant="light">system admin</Badge>}</Table.Td>
                  <Table.Td>
                    <Code fz="xs">{u.id}</Code>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Card>

      <Modal opened={opened} onClose={close} title="New user">
        <form onSubmit={create}>
          <Stack>
            <TextInput label="Email" required value={form.email} onChange={(e) => setForm({ ...form, email: e.currentTarget.value })} />
            <TextInput label="Name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.currentTarget.value })} />
            <PasswordInput
              label="Password"
              description="8 to 72 characters"
              required
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.currentTarget.value })}
            />
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>
                Cancel
              </Button>
              <Button type="submit">Create user</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
