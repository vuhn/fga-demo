import { Button, Card, Group, Modal, Stack, Table, Text, TextInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconChevronRight, IconPlus } from "@tabler/icons-react";
import { useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Project } from "../api";
import { useAuth } from "../auth";
import { Can } from "../permissions";
import { notifyError, PageHeader, run } from "../ui";

/** Projects of the active tenant. The server lists only the ones the caller can view. */
export default function Projects() {
  const navigate = useNavigate();
  const { me, tenantId } = useAuth();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [opened, { open, close }] = useDisclosure();
  const [name, setName] = useState("");

  const load = () => api<Project[]>("GET", "/projects").then(setProjects, notifyError);
  useEffect(() => {
    if (me && tenantId) void load();
  }, [me, tenantId]);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("POST", "/projects", { name: name.trim() }), `Created ${name}`)) {
      setName("");
      close();
      await load();
    }
  };

  return (
    <>
      <PageHeader
        title="Projects"
        description="A project owns documents, photos, and other items. Members and roles are set per project and add to the tenant role."
        actions={
          <Can feature="tenant.projects.manage">
            <Button leftSection={<IconPlus size={16} />} onClick={open}>
              New project
            </Button>
          </Can>
        }
      />
      <Card withBorder padding={0}>
        <Table verticalSpacing="sm" highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Name</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {(projects ?? []).map((p) => (
              <Table.Tr key={p.id} style={{ cursor: "pointer" }} onClick={() => navigate(`/projects/${p.id}`)}>
                <Table.Td fw={500}>{p.name}</Table.Td>
                <Table.Td ta="right">
                  <IconChevronRight size={16} />
                </Table.Td>
              </Table.Tr>
            ))}
            {projects?.length === 0 && (
              <Table.Tr>
                <Table.Td colSpan={2}>
                  <Text size="sm" c="dimmed">
                    You are not in any project yet.
                  </Text>
                </Table.Td>
              </Table.Tr>
            )}
          </Table.Tbody>
        </Table>
      </Card>

      <Modal opened={opened} onClose={close} title="New project">
        <form onSubmit={create}>
          <Stack>
            <TextInput label="Name" data-autofocus required value={name} onChange={(e) => setName(e.currentTarget.value)} />
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
