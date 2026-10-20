import { Button, Card, Checkbox, Group, Stack, Text, TextInput, Title } from "@mantine/core";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, CUSTOM_PERMISSIONS, type CustomRole } from "../api";
import { RequireFeature, usePermissionSet } from "../permissions";
import { AffiliationBadge, confirmDanger, notifyError, PageHeader, run } from "../ui";

function samePerms(a: string[], b: string[]) {
  if (a.length !== b.length) return false;
  const sorted = [...b].sort();
  return [...a].sort().every((value, i) => value === sorted[i]);
}

export default function RoleDetail() {
  const { id = "", roleId = "" } = useParams();
  const loaded = usePermissionSet(id);
  return (
    <RequireFeature feature="tenant.roles.manage" set={loaded}>
      <RolePage tenantId={id} roleId={roleId} />
    </RequireFeature>
  );
}

function RolePage({ tenantId, roleId }: { tenantId: string; roleId: string }) {
  const navigate = useNavigate();
  const [role, setRole] = useState<CustomRole | null>(null);
  const [name, setName] = useState("");
  const [selected, setSelected] = useState<string[]>([]);

  const load = useCallback(() => {
    api<CustomRole>("GET", `/tenants/${tenantId}/roles/${roleId}`).then(
      (next) => {
        setRole(next);
        setName(next.name);
        setSelected(next.permissions);
      },
      notifyError,
    );
  }, [tenantId, roleId]);

  useEffect(() => {
    void load();
  }, [load]);

  const rename = async (e: FormEvent) => {
    e.preventDefault();
    if (await run(() => api("PATCH", `/tenants/${tenantId}/roles/${roleId}`, { name }), "Role renamed")) load();
  };

  const savePerms = () =>
    run(() => api("PUT", `/tenants/${tenantId}/roles/${roleId}/permissions`, { permissions: selected }), "Permissions saved").then((ok) => ok && load());

  if (!role) return null;

  return (
    <>
      <PageHeader
        title={role.name}
        description={
          <Group gap="xs">
            <Text span c="dimmed" size="sm">
              Custom role
            </Text>
            <AffiliationBadge affiliation={role.affiliation} />
          </Group>
        }
        actions={
          <Group>
            <Button component={Link} to={`/tenants/${tenantId}/roles`} variant="default">
              All roles
            </Button>
            <Button
              color="red"
              variant="light"
              onClick={() =>
                confirmDanger(
                  `Delete ${role.name}?`,
                  role.assignees.length
                    ? "This role is still assigned. Change those members to another role first."
                    : "The role and its permission grants are removed. Built-in roles are unchanged.",
                  "Delete",
                  () =>
                    void run(() => api("DELETE", `/tenants/${tenantId}/roles/${roleId}`), `Deleted ${role.name}`).then(
                      (ok) => ok && navigate(`/tenants/${tenantId}/roles`),
                    ),
                )
              }
            >
              Delete
            </Button>
          </Group>
        }
      />

      <Card withBorder mb="lg">
        <form onSubmit={rename}>
          <Group align="flex-end">
            <TextInput label="Name" value={name} onChange={(e) => setName(e.currentTarget.value)} style={{ flex: 1 }} />
            <Button type="submit" variant="default" disabled={!name.trim() || name.trim() === role.name}>
              Rename
            </Button>
          </Group>
        </form>
        <Text size="xs" c="dimmed" mt="sm">
          {role.affiliation === "internal" ? "Only internal members can hold this role." : "Only external members can hold this role."} Assign it from the
          tenant member list.
        </Text>
      </Card>

      <Card withBorder mb="lg">
        <Title order={4} mb="sm">
          Permissions
        </Title>
        <Checkbox.Group value={selected} onChange={setSelected}>
          <Stack gap="xs">
            {CUSTOM_PERMISSIONS.map((p) => (
              <Checkbox key={p.value} value={p.value} label={p.label} description={p.hint} />
            ))}
          </Stack>
        </Checkbox.Group>
        <Group justify="flex-end" mt="md">
          <Button disabled={samePerms(selected, role.permissions)} onClick={() => void savePerms()}>
            Save permissions
          </Button>
        </Group>
      </Card>

      <Card withBorder>
        <Title order={4} mb="sm">
          Members
        </Title>
        {role.assignees.length === 0 ? (
          <Text size="sm" c="dimmed">
            Nobody holds this role. Assign it from the member list, the same way as a built-in role.
          </Text>
        ) : (
          <Stack gap={4}>
            {role.assignees.map((a) => (
              <Text key={a.user_id} size="sm">
                {a.name} <Text span c="dimmed">{a.email}</Text>
              </Text>
            ))}
          </Stack>
        )}
      </Card>
    </>
  );
}
