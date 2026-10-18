// Shared UI helpers on top of Mantine.
import { Badge, Group, Stack, Text, Title } from "@mantine/core";
import { modals } from "@mantine/modals";
import { notifications } from "@mantine/notifications";
import { IconAlertCircle, IconCheck } from "@tabler/icons-react";
import type { ReactNode } from "react";
import { ApiError } from "./api";

export function notifyError(err: unknown) {
  const title = err instanceof ApiError ? errorTitle(err) : "Something went wrong";
  const message =
    err instanceof ApiError && err.check ? `${err.check.relation} on ${err.check.object}` : err instanceof Error ? err.message : String(err);
  notifications.show({ color: "red", title, message, icon: <IconAlertCircle size={18} /> });
}

export function notifySuccess(message: string) {
  notifications.show({ color: "teal", message, icon: <IconCheck size={18} />, autoClose: 2500 });
}

function errorTitle(err: ApiError): string {
  switch (err.code) {
    case "forbidden":
      return "Not allowed";
    case "not_found":
      return "Not found";
    case "conflict":
      return "Conflict";
    case "cannot_upload":
      return "This person cannot upload";
    case "not_a_member":
      return "Not a member of this tenant";
    case "affiliation_role_mismatch":
      return "Role does not match affiliation";
    case "validation":
      return "Invalid input";
    default:
      return `Error: ${err.code}`;
  }
}

/** Runs an async action, reporting failure as a notification. Returns true on success. */
export async function run(fn: () => Promise<unknown>, success?: string): Promise<boolean> {
  try {
    await fn();
    if (success) notifySuccess(success);
    return true;
  } catch (e) {
    notifyError(e);
    return false;
  }
}

export function confirmDanger(title: string, body: ReactNode, confirmLabel: string, onConfirm: () => void) {
  modals.openConfirmModal({
    title,
    children: <Text size="sm">{body}</Text>,
    labels: { confirm: confirmLabel, cancel: "Cancel" },
    confirmProps: { color: "red" },
    onConfirm,
  });
}

export function PageHeader({ title, description, actions }: { title: ReactNode; description?: ReactNode; actions?: ReactNode }) {
  return (
    <Group justify="space-between" align="flex-start" mb="lg" wrap="wrap">
      <Stack gap={4}>
        <Title order={2}>{title}</Title>
        {description && (
          <Text c="dimmed" size="sm">
            {description}
          </Text>
        )}
      </Stack>
      {actions && <Group gap="xs">{actions}</Group>}
    </Group>
  );
}

const ROLE_COLOR: Record<string, string> = {
  internal_admin: "indigo",
  internal_user: "blue",
  external_admin: "orange",
  external_user: "yellow",
};

/** Display badge for a membership label. Never used to decide access. */
export function RoleBadge({ role }: { role?: string }) {
  if (!role) return null;
  return (
    <Badge variant="light" color={ROLE_COLOR[role] ?? "gray"}>
      {role.replace("_", " ")}
    </Badge>
  );
}

export function AffiliationBadge({ affiliation }: { affiliation?: string }) {
  if (!affiliation) return null;
  return (
    <Badge variant="dot" color={affiliation === "internal" ? "indigo" : "orange"}>
      {affiliation}
    </Badge>
  );
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
