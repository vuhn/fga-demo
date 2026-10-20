// UI permission layer.
//
// The server answers "what may I do" with OpenFGA Checks (GET /me/permissions
// for platform and tenant, and a `permissions` object on every content row).
// This file is the ONLY place that maps those permissions to menus, pages, and
// buttons. Components ask for a feature by name; they never read role or
// affiliation labels and never name OpenFGA relations themselves.
//
// Hiding is a convenience, not security: the API still checks every request.

import { Center, EmptyState, Loader } from "@mantine/core";
import { IconLock } from "@tabler/icons-react";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, type Perms } from "./api";
import { useAuth } from "./auth";

// ---- Server shape ----

export type PermissionSet = {
  platform: Perms;
  tenant_id: string | null;
  tenant: Perms | null;
};

const NONE: PermissionSet = { platform: {}, tenant_id: null, tenant: null };

// ---- Feature map ----

/** A permission reference: scope + relation, e.g. "tenant:can_manage_members". */
type PermRef = `platform:${string}` | `tenant:${string}`;

/**
 * A feature rule. `all` lists permissions that must ALL be true (for example
 * "member of the tenant" and "product enabled"); `any` lists alternatives where
 * ONE is enough (the decision catalog's "platform admin OR tenant relation").
 * When both are present, both must pass. A rule with neither is denied.
 */
type Rule = { readonly all?: readonly PermRef[]; readonly any?: readonly PermRef[] };

const FEATURES = {
  // Navigation and pages. Product pages need membership AND the product switched on.
  "nav.users": { all: ["platform:can_manage_platform"] },
  "nav.tenants": { all: ["platform:can_manage_platform"] },
  "nav.documents": { all: ["tenant:can_read_tenant", "tenant:documents_enabled"] },
  "nav.photos": { all: ["tenant:can_read_tenant", "tenant:photos_enabled"] },
  // Members is the tenant administration page (products, roster, roles): internal admins only.
  // It is a menu entry for the active tenant; platform admins reach tenants via the directory.
  "nav.members": { all: ["tenant:can_manage_members"] },

  // Tenant administration
  "tenant.member": { all: ["tenant:can_read_tenant"] },
  // Tenant administration page: internal admin of that tenant, or the platform.
  "tenant.admin": { any: ["tenant:can_manage_members", "platform:can_manage_platform"] },
  // Products are a platform entitlement: only system admins toggle them. Tenant admins see status.
  "tenant.products.manage": { all: ["platform:can_manage_platform"] },
  "tenant.members.manage": { any: ["tenant:can_manage_members", "platform:can_manage_platform"] },
  // Custom roles are a platform entitlement, like product toggles.
  "tenant.roles.manage": { all: ["platform:can_manage_platform"] },

  // Content creation at tenant level (the model already intersects the product).
  "folder.create": { all: ["tenant:can_create_folder"] },
  "album.create": { all: ["tenant:can_create_album"] },
  "folder.createPrivate": { all: ["tenant:can_create_private_folder"] },
  "album.createPrivate": { all: ["tenant:can_create_private_album"] },
} as const satisfies Record<string, Rule>;

export type Feature = keyof typeof FEATURES;

function has(set: PermissionSet, ref: PermRef): boolean {
  const [scope, relation] = ref.split(":") as ["platform" | "tenant", string];
  return (scope === "platform" ? set.platform : set.tenant)?.[relation] === true;
}

export function allows(set: PermissionSet, feature: Feature): boolean {
  const rule: Rule = FEATURES[feature];
  if (!rule.all && !rule.any) return false;
  const allOk = (rule.all ?? []).every((ref) => has(set, ref));
  const anyOk = !rule.any || rule.any.some((ref) => has(set, ref));
  return allOk && anyOk;
}

// ---- Object actions ----

export type ObjectType = "folder" | "document" | "album" | "photo";

/** Which relation on a content object enables each action button. */
const ACTIONS = {
  rename: { folder: "can_manage", album: "can_manage", document: "can_edit", photo: "can_edit" },
  delete: { folder: "can_delete", album: "can_delete", document: "can_delete", photo: "can_delete" },
  share: { folder: "can_share", album: "can_share", document: "can_share", photo: "can_share" },
  addItem: { folder: "can_create_document", album: "can_create_photo" },
  /** Create a subfolder/subalbum inside this container. */
  addChild: { folder: "can_edit", album: "can_edit" },
  /** Switch between private and public. */
  visibility: { folder: "can_change_visibility", album: "can_change_visibility" },
} as const satisfies Record<string, Partial<Record<ObjectType, string>>>;

export type Action = keyof typeof ACTIONS;

/** True when the object's server-provided permissions allow the action. */
export function canAct(obj: { permissions?: Perms }, type: ObjectType, action: Action): boolean {
  const relation = (ACTIONS[action] as Partial<Record<ObjectType, string>>)[type];
  return relation !== undefined && obj.permissions?.[relation] === true;
}

// ---- Loading ----

type Loaded = { set: PermissionSet; loading: boolean };

/**
 * Loads permissions for a tenant (or platform only when tenantId is null).
 * Reloads when the signed-in user's data refreshes, so a product toggle or a
 * role change shows up immediately. Denies everything while loading.
 */
export function usePermissionSet(tenantId: string | null): Loaded {
  const { me } = useAuth();
  const [state, setState] = useState<Loaded>({ set: NONE, loading: true });
  const [version, setVersion] = useState(0);

  useEffect(() => {
    if (!me) {
      setState({ set: NONE, loading: false });
      return;
    }
    let cancelled = false;
    // Same tenant: revalidate in the background and keep showing current permissions.
    // New tenant: deny everything until the answer arrives.
    setState((s) => (!s.loading && s.set.tenant_id === tenantId ? s : { set: NONE, loading: true }));
    const q = tenantId ? `?tenant_id=${tenantId}` : "";
    api<PermissionSet>("GET", `/me/permissions${q}`).then(
      (set) => !cancelled && setState({ set, loading: false }),
      () => !cancelled && setState({ set: NONE, loading: false }),
    );
    return () => {
      cancelled = true;
    };
  }, [me, tenantId, version]);

  // Permissions can change elsewhere (another admin toggles a product), so
  // re-check when the user returns to the tab.
  useEffect(() => {
    const onFocus = () => document.visibilityState === "visible" && setVersion((v) => v + 1);
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
    };
  }, []);

  return state;
}

const PermissionsContext = createContext<Loaded>({ set: NONE, loading: true });

/** Provides the permissions for the active tenant to the whole app. */
export function PermissionsProvider({ children }: { children: ReactNode }) {
  const { tenantId } = useAuth();
  const loaded = usePermissionSet(tenantId);
  return <PermissionsContext.Provider value={loaded}>{children}</PermissionsContext.Provider>;
}

/** Feature check against the active tenant. */
export function useCan(feature: Feature): boolean {
  return allows(useContext(PermissionsContext).set, feature);
}

/** Returns a checker for many features at once (for example, filtering a menu). */
export function useAllows(): (feature: Feature) => boolean {
  const { set } = useContext(PermissionsContext);
  return (feature) => allows(set, feature);
}

/** Renders children only when the feature is allowed for the active tenant. */
export function Can({ feature, children, fallback = null }: { feature: Feature; children: ReactNode; fallback?: ReactNode }) {
  return <>{useCan(feature) ? children : fallback}</>;
}

/**
 * Route guard. Pass `set` to evaluate against a specific tenant (for example
 * the tenant in the URL); otherwise the active tenant is used.
 */
export function RequireFeature({
  feature,
  set,
  denied,
  children,
}: {
  feature: Feature;
  set?: Loaded;
  /** Shown instead of the generic message when the feature is not allowed. */
  denied?: ReactNode;
  children: ReactNode;
}) {
  const active = useContext(PermissionsContext);
  const { set: perms, loading } = set ?? active;
  if (loading) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    );
  }
  if (!allows(perms, feature)) {
    if (denied) return <>{denied}</>;
    return (
      <EmptyState
        mt="xl"
        icon={<IconLock size={28} />}
        title="Not available"
        description="You do not have access to this page with your current relationships."
      />
    );
  }
  return <>{children}</>;
}
