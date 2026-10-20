import { Alert, Avatar, Badge, Card, Code, EmptyState, Group, SimpleGrid, Stack, Table, Text, ThemeIcon, Title } from "@mantine/core";
import { IconBuilding, IconInfoCircle, IconShieldCheck, IconUserShield } from "@tabler/icons-react";
import { useAuth } from "../auth";
import { AffiliationBadge, PageHeader, RoleBadge } from "../ui";

export default function Home() {
  const { me, tenantId } = useAuth();
  if (!me) return null;
  return (
    <>
      <PageHeader title={`Welcome, ${me.user.name}`} description="Your identity, memberships, and how access works in this demo." />

      <SimpleGrid cols={{ base: 1, md: 3 }} mb="lg">
        <Card withBorder padding="lg">
          <Group>
            <Avatar size="lg" radius="xl" color="indigo">
              {me.user.name[0]}
            </Avatar>
            <Stack gap={2} style={{ minWidth: 0 }}>
              <Text fw={600}>{me.user.name}</Text>
              <Text size="sm" c="dimmed" truncate>
                {me.user.email}
              </Text>
            </Stack>
          </Group>
          <Code mt="md" block fz="xs" style={{ wordBreak: "break-all", whiteSpace: "pre-wrap" }}>
            user:{me.user.id}
          </Code>
        </Card>
        <Card withBorder padding="lg">
          <Group justify="space-between">
            <Text size="sm" c="dimmed">
              Platform role
            </Text>
            <ThemeIcon variant="light" color={me.user.is_system_admin ? "indigo" : "gray"}>
              <IconUserShield size={18} />
            </ThemeIcon>
          </Group>
          <Title order={3} mt="xs">
            {me.user.is_system_admin ? "System admin" : "Tenant user"}
          </Title>
          <Text size="xs" c="dimmed" mt={4}>
            can_manage_platform on platform:main is {String(me.user.is_system_admin)}
          </Text>
        </Card>
        <Card withBorder padding="lg">
          <Group justify="space-between">
            <Text size="sm" c="dimmed">
              Tenants
            </Text>
            <ThemeIcon variant="light">
              <IconBuilding size={18} />
            </ThemeIcon>
          </Group>
          <Title order={3} mt="xs">
            {me.tenants.length}
          </Title>
          <Text size="xs" c="dimmed" mt={4}>
            From ListObjects can_read_tenant
          </Text>
        </Card>
      </SimpleGrid>

      <Card withBorder padding="lg" mb="lg">
        <Title order={4} mb="sm">
          Memberships
        </Title>
        {me.tenants.length === 0 ? (
          <EmptyState icon={<IconBuilding size={24} />} title="No memberships" description="Nothing here is visible with the current relationships." />
        ) : (
          <Table.ScrollContainer minWidth={420}>
            <Table verticalSpacing="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Tenant</Table.Th>
                  <Table.Th>Affiliation</Table.Th>
                  <Table.Th>Role</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {me.tenants.map((t) => (
                  <Table.Tr key={t.id}>
                    <Table.Td fw={500}>{t.name}</Table.Td>
                    <Table.Td>
                      <AffiliationBadge affiliation={t.affiliation} />
                    </Table.Td>
                    <Table.Td>
                      <RoleBadge role={t.role} name={t.role_name} />
                    </Table.Td>
                    <Table.Td>{t.id === tenantId && <Badge variant="outline">active</Badge>}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </Card>

      <SimpleGrid cols={{ base: 1, md: 2 }}>
        <Alert variant="light" color="indigo" title="Internal versus external" icon={<IconInfoCircle />}>
          Internal members work for the tenant and see every folder and album, private or public; they can add and edit content,
          and the internal admin also manages members, products, and every file. External members are invited guests: they see
          public folders (and anything shared with them). An external admin can also create public folders and add or edit
          content in them; an external user can only view. Affiliation is fixed when the person is added.
        </Alert>
        <Alert variant="light" color="teal" title="How this UI decides what to show" icon={<IconShieldCheck />}>
          Every menu and button is shown from OpenFGA Check results returned by the API, never from the role labels above. The API
          checks again on every request.
        </Alert>
      </SimpleGrid>
    </>
  );
}
