import { Alert, Avatar, Box, Button, Center, Divider, Group, Paper, PasswordInput, SimpleGrid, Stack, Text, TextInput, ThemeIcon, Title, UnstyledButton } from "@mantine/core";
import { IconAlertCircle, IconShieldLock } from "@tabler/icons-react";
import { useState, type FormEvent } from "react";
import { useAuth } from "../auth";

// Seeded demo personas (display copy only; access comes from OpenFGA).
const DEMO = [
  { id: "ada", name: "Ada", hint: "System admin" },
  { id: "bob", name: "Bob", hint: "Acme admin · Globex guest" },
  { id: "cara", name: "Cara", hint: "Acme internal user" },
  { id: "dan", name: "Dan", hint: "Acme external user" },
  { id: "fay", name: "Fay", hint: "Acme external admin" },
  { id: "eve", name: "Eve", hint: "Globex admin" },
];

export default function Login() {
  const { login } = useAuth();
  const [email, setEmail] = useState("ada@example.com");
  const [password, setPassword] = useState("password");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const signIn = async (address: string) => {
    setError("");
    setBusy(true);
    try {
      await login(address, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    void signIn(email);
  };

  return (
    <Center mih="100vh" p="md" bg="var(--mantine-color-body)">
      <Box w="100%" maw={440}>
        <Stack align="center" gap={6} mb="lg">
          <ThemeIcon variant="gradient" gradient={{ from: "indigo", to: "cyan" }} size={48} radius="md">
            <IconShieldLock size={28} />
          </ThemeIcon>
          <Title order={2}>FGA Demo</Title>
          <Text c="dimmed" size="sm" ta="center">
            Multi-tenant fine-grained authorization with OpenFGA
          </Text>
        </Stack>
        <Paper withBorder shadow="sm" p="xl" radius="lg">
          <form onSubmit={submit}>
            <Stack>
              {error && (
                <Alert color="red" icon={<IconAlertCircle size={16} />}>
                  Invalid email or password.
                </Alert>
              )}
              <TextInput label="Email" value={email} onChange={(e) => setEmail(e.currentTarget.value)} autoComplete="username" required />
              <PasswordInput label="Password" value={password} onChange={(e) => setPassword(e.currentTarget.value)} autoComplete="current-password" required />
              <Button type="submit" fullWidth loading={busy}>
                Sign in
              </Button>
            </Stack>
          </form>
          <Divider my="lg" label="or continue as a demo user" labelPosition="center" />
          <SimpleGrid cols={2} spacing="xs">
            {DEMO.map((d) => (
              <UnstyledButton key={d.id} onClick={() => signIn(`${d.id}@example.com`)} disabled={busy}>
                <Paper withBorder p="xs" radius="md" className="row-hover">
                  <Group gap="xs" wrap="nowrap">
                    <Avatar size="sm" radius="xl" color="indigo">
                      {d.name[0]}
                    </Avatar>
                    <Box style={{ minWidth: 0 }}>
                      <Text size="sm" fw={500}>
                        {d.name}
                      </Text>
                      <Text size="xs" c="dimmed" truncate>
                        {d.hint}
                      </Text>
                    </Box>
                  </Group>
                </Paper>
              </UnstyledButton>
            ))}
          </SimpleGrid>
          <Text size="xs" c="dimmed" ta="center" mt="sm">
            Every demo account uses the password “password”.
          </Text>
        </Paper>
      </Box>
    </Center>
  );
}
