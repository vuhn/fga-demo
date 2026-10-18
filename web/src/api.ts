// Thin client for the Go API. The JWT and selected tenant live in sessionStorage.

export type User = { id: string; email: string; name: string; is_system_admin: boolean };
export type Product = { product: "documents" | "photos"; enabled: boolean; updated_at: string };
export type Tenant = {
  id: string;
  slug: string;
  name: string;
  created_at: string;
  products: Product[];
  affiliation?: string;
  role?: string;
};
export type Member = { user_id: string; email: string; name: string; affiliation: string; role: string };
export type Container = {
  id: string;
  tenant_id: string;
  parent_id: string | null;
  name: string;
  /** Private containers are visible to internal members only, unless shared. */
  is_private: boolean;
  created_by: string;
  created_at: string;
  permissions?: Perms;
};
export type Item = {
  id: string;
  tenant_id: string;
  name: string;
  mime_type: string;
  byte_size: number;
  created_by: string;
  created_at: string;
  folder_id?: string;
  album_id?: string;
  permissions?: Perms;
};
export type Grant = { id: string; relation: string; subject: string; user_id: string | null; name?: string };
export type Tuple = { user: string; relation: string; object: string };
/** The caller's own permission relations on one object, as returned by the API. */
export type Perms = Record<string, boolean>;

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public check?: { relation: string; object: string },
    detail?: string,
  ) {
    super(detail ? `${code}: ${detail}` : check ? `${code} (${check.relation} on ${check.object})` : code);
  }
}

export const session = {
  get token() {
    return sessionStorage.getItem("token");
  },
  set token(v: string | null) {
    v ? sessionStorage.setItem("token", v) : sessionStorage.removeItem("token");
  },
  get tenantId() {
    return sessionStorage.getItem("tenant_id");
  },
  set tenantId(v: string | null) {
    v ? sessionStorage.setItem("tenant_id", v) : sessionStorage.removeItem("tenant_id");
  },
};

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (session.token) headers.Authorization = `Bearer ${session.token}`;
  if (session.tenantId) headers["X-Tenant-ID"] = session.tenantId;
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.error ?? "error", data.check, data.message);
  return data as T;
}

export const ROLES: Record<string, string[]> = {
  internal: ["internal_admin", "internal_user"],
  external: ["external_admin", "external_user"],
};
