import { AppShell, Avatar, Burger, Center, EmptyState, Group, Loader, Menu, NavLink as MantineNavLink, Stack, Text, ThemeIcon, UnstyledButton, useMantineColorScheme } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconBuilding,
  IconChevronDown,
  IconFileText,
  IconHome,
  IconLogout,
  IconMoon,
  IconPhoto,
  IconPower,
  IconShieldCheck,
  IconShieldLock,
  IconSun,
  IconUsers,
  IconUsersGroup,
  type Icon,
} from "@tabler/icons-react";
import { lazy, Suspense } from "react";
import { Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "./auth";
import { RequireFeature, useAllows, useCan, type Feature } from "./permissions";
import TenantSwitcher from "./components/TenantSwitcher";
import Login from "./pages/Login";

// Pages load on demand so the first paint ships only the shell.
const Home = lazy(() => import("./pages/Home"));
const Users = lazy(() => import("./pages/Users"));
const Tenants = lazy(() => import("./pages/Tenants"));
const TenantDetail = lazy(() => import("./pages/TenantDetail"));
const Documents = lazy(() => import("./pages/Documents"));
const Photos = lazy(() => import("./pages/Photos"));
const Inspector = lazy(() => import("./pages/Inspector"));

/** Menu entries. `feature` is resolved against the active tenant; omit it for always-visible entries. */
type NavItem = { label: string; icon: Icon; to: (tenantId: string | null) => string; feature?: Feature; section: "workspace" | "platform" };

const NAV: NavItem[] = [
  { section: "workspace", label: "Home", icon: IconHome, to: () => "/" },
  { section: "workspace", label: "Documents", icon: IconFileText, to: () => "/documents", feature: "nav.documents" },
  { section: "workspace", label: "Photos", icon: IconPhoto, to: () => "/photos", feature: "nav.photos" },
  { section: "workspace", label: "Members", icon: IconUsersGroup, to: (t) => `/tenants/${t}`, feature: "nav.members" },
  { section: "workspace", label: "Access inspector", icon: IconShieldCheck, to: () => "/inspector" },
  { section: "platform", label: "Users", icon: IconUsers, to: () => "/users", feature: "nav.users" },
  { section: "platform", label: "Tenants", icon: IconBuilding, to: () => "/tenants", feature: "nav.tenants" },
];

function Navigation({ onNavigate }: { onNavigate: () => void }) {
  const { tenantId } = useAuth();
  const allows = useAllows();
  const location = useLocation();
  const visible = NAV.filter((item) => !item.feature || allows(item.feature));
  const section = (name: NavItem["section"], label: string) => {
    const items = visible.filter((i) => i.section === name);
    if (items.length === 0) return null;
    return (
      <Stack gap={2}>
        <Text size="xs" fw={600} c="dimmed" tt="uppercase" px="sm" mb={4}>
          {label}
        </Text>
        {items.map((item) => {
          const to = item.to(tenantId);
          return (
            <MantineNavLink
              key={item.label}
              component={NavLink}
              to={to}
              label={item.label}
              leftSection={<item.icon size={18} stroke={1.6} />}
              active={location.pathname === to}
              onClick={onNavigate}
              style={{ borderRadius: "var(--mantine-radius-md)" }}
            />
          );
        })}
      </Stack>
    );
  };
  return (
    <Stack gap="lg">
      {section("workspace", "Workspace")}
      {section("platform", "Platform")}
    </Stack>
  );
}

/** Denied state for a product page: explains a switched-off product to members. */
function ProductUnavailable({ product }: { product: string }) {
  const member = useCan("tenant.member");
  return (
    <EmptyState
      mt="xl"
      icon={<IconPower size={28} />}
      title={member ? `${product} is turned off for this tenant` : "Not available"}
      description={
        member
          ? "The content is kept, but every content Check fails until a tenant admin or system admin enables the product again."
          : "You do not have access to this page with your current relationships."
      }
    />
  );
}

function UserMenu() {
  const { me, logout } = useAuth();
  const { colorScheme, setColorScheme } = useMantineColorScheme();
  if (!me) return null;
  const dark = colorScheme === "dark";
  return (
    <Menu position="bottom-end" width={220} shadow="md">
      <Menu.Target>
        <UnstyledButton>
          <Group gap="xs">
            <Avatar color="indigo" radius="xl" size="sm">
              {me.user.name[0]}
            </Avatar>
            <Text size="sm" fw={500} visibleFrom="sm">
              {me.user.name}
            </Text>
            <IconChevronDown size={14} />
          </Group>
        </UnstyledButton>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>{me.user.email}</Menu.Label>
        <Menu.Item leftSection={dark ? <IconSun size={16} /> : <IconMoon size={16} />} onClick={() => setColorScheme(dark ? "light" : "dark")}>
          {dark ? "Light mode" : "Dark mode"}
        </Menu.Item>
        <Menu.Divider />
        <Menu.Item color="red" leftSection={<IconLogout size={16} />} onClick={logout}>
          Sign out
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}

export default function App() {
  const { me, loading } = useAuth();
  const [opened, { toggle, close }] = useDisclosure();

  if (loading) {
    return (
      <Center h="100vh">
        <Loader />
      </Center>
    );
  }
  if (!me) {
    return (
      <Routes>
        <Route path="*" element={<Login />} />
      </Routes>
    );
  }

  return (
    <AppShell header={{ height: 60 }} navbar={{ width: 250, breakpoint: "sm", collapsed: { mobile: !opened } }} padding="lg">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <ThemeIcon variant="gradient" gradient={{ from: "indigo", to: "cyan" }} size="lg" radius="md">
              <IconShieldLock size={20} />
            </ThemeIcon>
            <Text fw={700} visibleFrom="xs">
              FGA Demo
            </Text>
          </Group>
          <Group gap="md" wrap="nowrap">
            <TenantSwitcher />
            <UserMenu />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="sm">
        <Navigation onNavigate={close} />
      </AppShell.Navbar>
      <AppShell.Main>
        <Suspense
          fallback={
            <Center py="xl">
              <Loader />
            </Center>
          }
        >
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/users" element={<RequireFeature feature="nav.users"><Users /></RequireFeature>} />
          <Route path="/tenants" element={<RequireFeature feature="nav.tenants"><Tenants /></RequireFeature>} />
          {/* Guarded inside, against the tenant in the URL rather than the active one. */}
          <Route path="/tenants/:id" element={<TenantDetail />} />
          <Route path="/documents" element={<RequireFeature feature="nav.documents" denied={<ProductUnavailable product="Documents" />}><Documents /></RequireFeature>} />
          <Route path="/photos" element={<RequireFeature feature="nav.photos" denied={<ProductUnavailable product="Photos" />}><Photos /></RequireFeature>} />
          <Route path="/inspector" element={<Inspector />} />
          <Route path="/login" element={<Navigate to="/" replace />} />
          <Route path="*" element={<Text>Not found.</Text>} />
        </Routes>
        </Suspense>
      </AppShell.Main>
    </AppShell>
  );
}
