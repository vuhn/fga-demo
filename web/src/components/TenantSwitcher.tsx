import { Badge, Group, Select, Text } from "@mantine/core";
import { IconBuilding } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { matchPath, useLocation, useNavigate } from "react-router-dom";
import { api, type Tenant } from "../api";
import { useAuth } from "../auth";

/**
 * Lists the tenants the signed-in user belongs to (GET /tenants/mine, which is
 * ListObjects can_read_tenant) and sets the active tenant sent as X-Tenant-ID.
 */
export default function TenantSwitcher() {
  const { me, tenantId, selectTenant } = useAuth();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const location = useLocation();
  const navigate = useNavigate();

  // Reload when the user's data refreshes, e.g. after being added or removed.
  useEffect(() => {
    api<Tenant[]>("GET", "/tenants/mine").then(setTenants, () => setTenants([]));
  }, [me]);

  if (tenants.length === 0) {
    return (
      <Badge variant="light" color="gray" leftSection={<IconBuilding size={12} />}>
        No tenants
      </Badge>
    );
  }

  const change = (id: string | null) => {
    if (!id) return;
    selectTenant(id);
    // A tenant-scoped page follows the switch; other pages reload from context.
    if (matchPath("/tenants/:id", location.pathname)) navigate(`/tenants/${id}`);
  };

  const byId = new Map(tenants.map((t) => [t.id, t]));
  return (
    <Select
      aria-label="Active tenant"
      w={{ base: 160, sm: 240 }}
      value={tenantId}
      onChange={change}
      allowDeselect={false}
      leftSection={<IconBuilding size={16} />}
      data={tenants.map((t) => ({ value: t.id, label: t.name }))}
      renderOption={({ option }) => {
        const t = byId.get(option.value);
        return (
          <Group justify="space-between" w="100%" wrap="nowrap">
            <Text size="sm">{option.label}</Text>
            <Text size="xs" c="dimmed">
              {t?.affiliation} · {t?.role?.replace("_", " ")}
            </Text>
          </Group>
        );
      }}
    />
  );
}
