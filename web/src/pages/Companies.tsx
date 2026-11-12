import { Badge, Button, Card, Group, Modal, MultiSelect, Select, Stack, Table, Text } from "@mantine/core";
import { useEffect, useState, type FormEvent } from "react";
import { api, type Company, type Tenant } from "../api";
import { notifyError, PageHeader, run } from "../ui";

/**
 * Platform view of every company. A subcontractor that signs its own
 * contract is claimed: its unclaimed records (and confirmed duplicates from
 * other tenants) merge into the new tenant's own company.
 */
export default function Companies() {
  const [companies, setCompanies] = useState<Company[]>([]);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [claiming, setClaiming] = useState<Company | null>(null);

  const load = () => api<Company[]>("GET", "/companies").then(setCompanies, notifyError);
  useEffect(() => {
    void load();
    api<Tenant[]>("GET", "/tenants").then(setTenants, notifyError);
  }, []);

  const tenantName = (id?: string) => tenants.find((t) => t.id === id)?.name ?? id;
  const duplicates = (c: Company) => companies.filter((o) => o.id !== c.id && !o.claimed && o.name.toLowerCase() === c.name.toLowerCase());

  return (
    <>
      <PageHeader title="Companies" description="Employers of platform users. Tenants create subcontractor companies on their own behalf until the company claims itself." />
      <Card withBorder padding={0}>
        <Table.ScrollContainer minWidth={640}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Company</Table.Th>
                <Table.Th>Status</Table.Th>
                <Table.Th>Created by</Table.Th>
                <Table.Th>People</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {companies.map((c) => (
                <Table.Tr key={c.id}>
                  <Table.Td fw={500}>{c.name}</Table.Td>
                  <Table.Td>
                    {c.claimed ? (
                      <Badge variant="light" color="teal">
                        tenant: {tenantName(c.tenant_id)}
                      </Badge>
                    ) : (
                      <Badge variant="light" color="orange">
                        unclaimed
                      </Badge>
                    )}
                    {!c.claimed && duplicates(c).length > 0 && (
                      <Badge ml="xs" variant="outline" color="yellow">
                        possible duplicate
                      </Badge>
                    )}
                  </Table.Td>
                  <Table.Td>{c.created_by_tenant_id ? tenantName(c.created_by_tenant_id) : <Text c="dimmed">platform</Text>}</Table.Td>
                  <Table.Td>{c.member_count}</Table.Td>
                  <Table.Td ta="right">
                    {!c.claimed && (
                      <Button size="compact-xs" variant="light" onClick={() => setClaiming(c)}>
                        Claim…
                      </Button>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Card>
      {claiming && (
        <ClaimModal
          company={claiming}
          tenants={tenants}
          others={duplicates(claiming)}
          onClose={() => setClaiming(null)}
          onDone={async () => {
            setClaiming(null);
            await load();
          }}
        />
      )}
    </>
  );
}

function ClaimModal({ company, tenants, others, onClose, onDone }: { company: Company; tenants: Tenant[]; others: Company[]; onClose: () => void; onDone: () => Promise<void> }) {
  const [tenantId, setTenantId] = useState<string | null>(null);
  const [merge, setMerge] = useState<string[]>(others.map((o) => o.id));
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (tenantId && (await run(() => api("POST", `/tenants/${tenantId}/company-claims`, { company_ids: [company.id, ...merge] }), `Claimed ${company.name}`))) {
      await onDone();
    }
  };
  return (
    <Modal opened onClose={onClose} title={`Claim ${company.name}`}>
      <form onSubmit={submit}>
        <Stack>
          <Text size="sm" c="dimmed">
            Confirm ownership first (for example a verified email at the company). The people move into the tenant&rsquo;s own company; their memberships and access in
            other tenants do not change.
          </Text>
          <Select label="Claiming tenant" searchable value={tenantId} onChange={setTenantId} data={tenants.map((t) => ({ value: t.id, label: t.name }))} />
          {others.length > 0 && (
            <MultiSelect
              label="Merge duplicates"
              description="Same name, created by other tenants."
              value={merge}
              onChange={setMerge}
              data={others.map((o) => ({ value: o.id, label: `${o.name} (${o.member_count})` }))}
            />
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!tenantId}>
              Claim
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
