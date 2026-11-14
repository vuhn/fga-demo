// Thin client for the Go API. The JWT and selected tenant live in sessionStorage.

/** The employer shown next to a person everywhere user info is displayed. */
export type CompanyRef = { id: string; name: string };
export type User = { id: string; email: string; name: string; is_system_admin: boolean; company: CompanyRef };
export type Product = { product: "documents" | "photos" | "blueprints"; enabled: boolean; updated_at: string };
export type Tenant = {
  id: string;
  slug: string;
  name: string;
  created_at: string;
  products: Product[];
  affiliation?: string;
  role?: string;
  role_name?: string;
};
export type Member = {
  user_id: string;
  email: string;
  name: string;
  /** Derived from the member's company: internal when it is the tenant's own company. */
  affiliation: string;
  role: string;
  role_name?: string;
  company: CompanyRef;
};
export type Company = {
  id: string;
  name: string;
  tax_id?: string;
  created_by_tenant_id?: string;
  /** Set on a tenant's own company. */
  tenant_id?: string;
  claimed: boolean;
  claimed_at: string | null;
  member_count: number;
  created_at: string;
};
export type Project = { id: string; tenant_id: string; name: string; created_by?: string; created_at: string; permissions?: Perms };
export type ProjectMember = Member;
export type ProjectRole = {
  id: string;
  tenant_id: string;
  project_id: string;
  name: string;
  affiliation: "internal" | "external";
  permissions: string[];
  assignees: RoleAssignee[];
  created_at: string;
};
export type RoleAssignee = { user_id: string; email: string; name: string };
export type CustomRole = {
  id: string;
  tenant_id: string;
  name: string;
  affiliation: "internal" | "external";
  permissions: string[];
  assignees: RoleAssignee[];
  created_at: string;
};

/** Built-in roles plus the custom roles of the same affiliation (tenant roles or project roles). */
export function memberRoleOptions(affiliation: string, roles: { id: string; name: string; affiliation: string }[]) {
  const builtin = (ROLES[affiliation] ?? []).map((role) => ({ value: role, label: role.replaceAll("_", " ") }));
  const custom = roles.filter((role) => role.affiliation === affiliation).map((role) => ({ value: role.id, label: role.name }));
  return [...builtin, ...custom];
}
/** Fixed catalog a platform admin can attach to a tenant role. */
export const CUSTOM_PERMISSIONS: { value: string; label: string; hint: string }[] = [
  { value: "view", label: "View all content", hint: "Including private folders and albums, while the product is on." },
  { value: "edit", label: "Edit all content", hint: "Also grants view and contribute." },
  { value: "manage", label: "Manage all content", hint: "Delete, share, and rename. Also grants edit, view, and contribute." },
  { value: "change_visibility", label: "Change visibility", hint: "Make any folder or album public or private." },
  { value: "contribute", label: "Contribute", hint: "Create public folders and albums, and upload." },
  { value: "create_private", label: "Create private containers", hint: "Also grants contribute." },
  { value: "manage_members", label: "Manage members", hint: "Invite people and change their built-in role." },
  { value: "view_products", label: "View products", hint: "See whether documents and photos are enabled." },
];
export type Container = {
  id: string;
  tenant_id: string;
  project_id: string;
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
  project_id: string;
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

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  if (session.token) headers.Authorization = `Bearer ${session.token}`;
  if (session.tenantId) headers["X-Tenant-ID"] = session.tenantId;
  return headers;
}

async function toError(res: Response): Promise<ApiError> {
  const data = await res.json().catch(() => ({}));
  // a non-JSON answer (a proxy page, a route the server does not have) still says what happened
  return new ApiError(res.status, data.error ?? "error", data.check, data.message ?? (data.error ? undefined : `HTTP ${res.status} ${res.statusText}`.trim()));
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  if (!res.ok) throw await toError(res);
  return (await res.json().catch(() => ({}))) as T;
}

export type Progress = { kind: "step" | "thinking"; stage: string; message: string };

/**
 * POSTs and reads a Server-Sent Events answer (fetch, not EventSource: it needs the auth headers
 * and a POST body). `progress` events go to onProgress; the `done` payload is returned. A refusal
 * before the stream opens is the usual JSON error; an `error` event throws the same ApiError.
 */
export async function apiStream<T>(path: string, body: unknown, onProgress: (p: Progress) => void, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream", ...authHeaders() },
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok) throw await toError(res);
  if (!res.body) throw new ApiError(502, "no_stream");
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += value;
    for (let i = buf.indexOf("\n\n"); i >= 0; i = buf.indexOf("\n\n")) {
      const block = buf.slice(0, i);
      buf = buf.slice(i + 2);
      let event = "message";
      let data = "";
      for (const line of block.split("\n")) {
        if (line.startsWith("event: ")) event = line.slice(7);
        else if (line.startsWith("data: ")) data += line.slice(6);
      }
      if (!data) continue;
      const payload = JSON.parse(data);
      if (event === "progress") onProgress(payload as Progress);
      else if (event === "done") return payload as T;
      else if (event === "error") throw new ApiError(payload.status ?? 500, payload.error ?? "error", payload.check, payload.message);
    }
  }
  throw new ApiError(502, "stream_ended", undefined, "the server closed the stream before finishing");
}

/** POSTs multipart form data (the browser sets the boundary). */
export async function apiUpload<T>(path: string, form: FormData): Promise<T> {
  const res = await fetch(`/api/v1${path}`, { method: "POST", headers: authHeaders(), body: form });
  if (!res.ok) throw await toError(res);
  return (await res.json()) as T;
}

/** GETs a binary response (drawings need the auth headers, so no plain <img src>). */
export async function apiBlob(path: string): Promise<Blob> {
  const res = await fetch(`/api/v1${path}`, { headers: authHeaders() });
  if (!res.ok) throw await toError(res);
  return res.blob();
}

// ---- Blueprints (the engine owns the brief schema; these mirror it) ----

export type Side = "north" | "east" | "south" | "west";
export const SIDES: Side[] = ["north", "east", "south", "west"];
export const ROOM_TYPES = ["entry", "living", "kitchen", "dining", "study", "laundry", "storage", "bedroom", "bathroom"] as const;
export type RoomType = (typeof ROOM_TYPES)[number];
export const FIXTURE_KINDS = [
  "bed", "wardrobe", "desk", "shelves", "sofa", "tv_unit", "dining_table", "kitchen_counter",
  "sink", "stove", "fridge", "toilet", "basin", "shower", "bathtub", "washer",
] as const;
export type FixtureKind = (typeof FIXTURE_KINDS)[number];
/** Types that may sit inside another room (ensuite, walk-in closet, pantry). */
export const NESTABLE_TYPES: readonly RoomType[] = ["bathroom", "storage", "laundry"];
/** The engine's per-type defaults, used when a room omits `fixtures`. */
export const DEFAULT_FIXTURES: Record<RoomType, FixtureKind[]> = {
  entry: [], living: ["sofa", "tv_unit"], kitchen: ["kitchen_counter", "sink", "stove", "fridge"],
  dining: ["dining_table"], study: ["desk", "shelves"], laundry: ["washer", "sink"], storage: ["shelves"],
  bedroom: ["bed", "wardrobe"], bathroom: ["toilet", "basin", "shower"],
};
export type BriefRoom = {
  id: string;
  name: string;
  type: RoomType;
  target_area: number;
  min_width?: number | null;
  side?: Side | null;
  adjacent_to: string[];
  needs_window?: boolean | null;
  /** id of the room this one is inside (ensuite in a bedroom); its only door opens into that room. */
  parent?: string | null;
  fixtures?: FixtureKind[];
};
export type Brief = {
  title: string;
  lot: { width: number; depth: number; setbacks: { front: number; back: number; sides: number } };
  entrance_side: Side;
  rooms: BriefRoom[];
  /** No walls between the hall, entry, living, kitchen and dining (the hall itself stays). */
  open_plan?: boolean;
};
export type BlueprintCheck = {
  rule: string;
  severity: "error" | "warning";
  ok: boolean;
  message: string;
  room_id?: string | null;
  // issue record (older versions lack these)
  category?: "geometry" | "collision" | "information" | "program" | "design";
  issue_id?: string;
  elements?: string[];
  hint?: string;
};
export type BlueprintSource = { name: string; mime_type: string; byte_size: number };
export type BlueprintVersionSummary = { version: number; kind: "create" | "revise" | "review" | "edit" | "restore"; message: string; ok: boolean; created_by: string; created_at: string };
export type BlueprintVersion = BlueprintVersionSummary & { brief: Brief; model: unknown; checks: BlueprintCheck[]; notes: string[] };
export type Blueprint = {
  id: string;
  tenant_id: string;
  project_id: string;
  name: string;
  current_version: number;
  source_kind: "prompt" | "sketch" | "form";
  source: BlueprintSource | null;
  created_by: string;
  created_at: string;
  updated_at: string;
  permissions?: Perms;
};
export type BlueprintDetail = Blueprint & { version: BlueprintVersion };
export type BlueprintDraft = { brief: Brief; notes: string[]; source?: BlueprintSource };

export const ROLES: Record<string, string[]> = {
  internal: ["internal_admin", "internal_user"],
  external: ["external_admin", "external_user"],
};
