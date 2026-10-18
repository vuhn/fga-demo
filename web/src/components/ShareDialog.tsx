import { ActionIcon, Avatar, Badge, Box, Button, Divider, Group, Modal, SegmentedControl, Select, Stack, Text, ThemeIcon, Tooltip } from "@mantine/core";
import { IconTrash, IconUsersGroup } from "@tabler/icons-react";
import { useEffect, useState, type FormEvent } from "react";
import { api, type Grant, type Member } from "../api";
import { notifyError, run } from "../ui";

const SUBJECTS = [
  { value: "user", label: "An external person" },
  { value: "external_users", label: "All external users" },
  { value: "external_admins", label: "All external admins" },
];

const subjectLabel = (g: Grant) => (g.subject === "user" ? g.name || g.user_id : SUBJECTS.find((s) => s.value === g.subject)?.label);

const RELATION_COLOR: Record<string, string> = { owner: "indigo", editor: "teal", viewer: "gray" };

export default function ShareDialog({
  objectType,
  objectId,
  objectName,
  onClose,
}: {
  objectType: string;
  objectId: string;
  objectName: string;
  onClose: () => void;
}) {
  const [members, setMembers] = useState<Member[]>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [subject, setSubject] = useState("user");
  const [userId, setUserId] = useState<string | null>(null);
  const [relation, setRelation] = useState("viewer");
  const path = `/${objectType}s/${objectId}/grants`;

  const load = async () => {
    try {
      // Share targets come from the object (needs can_share), not the tenant roster.
      const [ms, gs] = await Promise.all([api<Member[]>("GET", `/${objectType}s/${objectId}/share-candidates`), api<Grant[]>("GET", path)]);
      setMembers(ms);
      setGrants(gs);
    } catch (e) {
      notifyError(e);
    }
  };
  useEffect(() => {
    void load();
  }, [objectId]);

  const person = members.find((m) => m.user_id === userId);
  // Display hint only; the API rejects these with cannot_upload regardless.
  const editorDisabled = subject === "external_users" || (subject === "user" && person?.role === "external_user");
  useEffect(() => {
    if (editorDisabled && relation === "editor") setRelation("viewer");
  }, [editorDisabled, relation]);

  const body = (g: { relation: string; subject: string; user_id?: string | null }) =>
    g.subject === "user" ? { relation: g.relation, subject: g.subject, user_id: g.user_id } : { relation: g.relation, subject: g.subject };

  const share = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("POST", path, body({ relation, subject, user_id: userId })), "Access granted")) await load();
  };

  const revoke = async (g: Grant) => {
    if (await run(() => api("DELETE", path, body(g)), "Access revoked")) await load();
  };

  return (
    <Modal opened onClose={onClose} title={`Share “${objectName}”`} size="lg">
      <Stack>
        <Text size="sm" fw={600}>
          People and groups with access
        </Text>
        <Stack gap="xs">
          {grants.map((g) => (
            <Group key={g.id} justify="space-between" wrap="nowrap">
              <Group gap="sm" wrap="nowrap">
                {g.subject === "user" ? (
                  <Avatar size="sm" radius="xl" color="indigo">
                    {(g.name || "?")[0]}
                  </Avatar>
                ) : (
                  <ThemeIcon size="md" radius="xl" variant="light" color="orange">
                    <IconUsersGroup size={14} />
                  </ThemeIcon>
                )}
                <Text size="sm">{subjectLabel(g)}</Text>
              </Group>
              <Group gap="xs" wrap="nowrap">
                <Badge variant="light" color={RELATION_COLOR[g.relation]}>
                  {g.relation}
                </Badge>
                {g.relation !== "owner" ? (
                  <Tooltip label="Revoke">
                    <ActionIcon variant="subtle" color="red" onClick={() => revoke(g)} aria-label="Revoke">
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Tooltip>
                ) : (
                  <Box w={28} />
                )}
              </Group>
            </Group>
          ))}
        </Stack>

        <Divider label="Add access" labelPosition="left" />

        <form onSubmit={share}>
          <Stack>
            <Group grow align="flex-start">
              <Select label="Share with" allowDeselect={false} value={subject} onChange={(v) => v && setSubject(v)} data={SUBJECTS} />
              {subject === "user" && (
                <Select
                  label="Person"
                  placeholder="Choose a member"
                  searchable
                  value={userId}
                  onChange={setUserId}
                  // Candidates are external members only: internal members already see everything.
                  data={members.map((m) => ({ value: m.user_id, label: `${m.name} · ${m.role.replace("_", " ")}` }))}
                  nothingFoundMessage="No external members"
                />
              )}
            </Group>
            <div>
              <Text size="sm" fw={500} mb={4}>
                Access
              </Text>
              <SegmentedControl
                value={relation}
                onChange={setRelation}
                data={[
                  { value: "viewer", label: "Viewer" },
                  { value: "editor", label: "Editor", disabled: editorDisabled },
                ]}
              />
              {editorDisabled && (
                <Text size="xs" c="dimmed" mt={4}>
                  External users cannot upload, so editor is unavailable.
                </Text>
              )}
            </div>
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose}>
                Done
              </Button>
              <Button type="submit" disabled={subject === "user" && !userId}>
                Share
              </Button>
            </Group>
          </Stack>
        </form>
      </Stack>
    </Modal>
  );
}
