# HTTP API

Routes, error catalog, and payloads. Handlers live in `api/internal/handler/http`.


Base path `/api/v1`. JSON uses snake_case. Timestamps are RFC3339. Authenticated routes require `Authorization: Bearer <jwt>`. Creates return 201 with the created object. Deletes and revokes return 204 with no body.

#### Error catalog

| Status | Code | When |
| --- | --- | --- |
| 400 | `validation` | Malformed JSON, bad UUID, empty name, bad slug, bad role, bad product, owner grant, unknown affiliation |
| 400 | `affiliation_role_mismatch` | Internal role on an external member, external role on an internal member (tenant or project role), or a role change that would switch affiliation |
| 400 | `not_a_member` | Share target fails `can_read_tenant` |
| 400 | `cannot_upload` | Editor grant targets an external user, or subject `external_users` is combined with relation `editor` |
| 401 | `unauthenticated` | Missing or invalid JWT |
| 403 | `forbidden` | A required Check returned false on a write |
| 404 | `not_found` | Unknown id, or a GET-by-id whose Check returned false, or `X-Tenant-ID` does not own the object |
| 409 | `conflict` | Duplicate email or slug, duplicate membership, last internal admin, cycle, depth, folder or album still has children, blueprint `base_version` is stale |
| 500 | `tuple_write_failed` | OpenFGA Write failed and compensation ran |
| 500 | `internal` | Any other unexpected error |
| 502 | `engine_unavailable` | The blueprint engine or its LLM is down, or returned bad output |

Error body:

```json
{"error": "forbidden", "check": {"relation": "can_view", "object": "document:<uuid>"}}
```

Omit `check` when the failure is not a Check. GET-by-id uses 404 for a failed Check so existence in another tenant stays hidden. Collection routes return an empty list, not 403, when ListObjects is empty. Writes use 403 when Check fails.

#### Auth and profile

`POST /api/v1/auth/login`

```json
{"email": "ada@example.com", "password": "password"}
```

Response 200:

```json
{
  "access_token": "<jwt>",
  "expires_at": "2026-10-18T12:00:00Z",
  "user": {"id": "<uuid>", "email": "ada@example.com", "name": "Ada", "is_system_admin": true}
}
```

`is_system_admin` on login and on `GET /api/v1/me` is the Check `can_manage_platform` on `platform:main`.

`GET /api/v1/me` returns the same user object plus:

```json
{
  "user": {},
  "tenants": [
    {"id": "<uuid>", "slug": "acme", "name": "Acme", "affiliation": "internal", "role": "internal_admin"}
  ]
}
```

`tenants` comes from ListObjects `can_read_tenant`, then a Postgres load of those ids. `affiliation` and `role` are the display labels from `memberships`.

`GET /api/v1/me/permissions?tenant_id=<optional uuid>` returns the actor's own permissions, evaluated in one BatchCheck:

```json
{
  "platform": {"can_manage_platform": false},
  "tenant_id": "<uuid>",
  "tenant": {
    "can_read_tenant": true, "can_contribute": true, "can_manage_members": true, "can_view_products": true,
    "can_create_folder": true, "can_create_album": true, "documents_enabled": true, "photos_enabled": true
  }
}
```

`tenant_id` and `tenant` are null when the query parameter is omitted. A malformed `tenant_id` is 400. The endpoint only evaluates the actor, so it needs no Check of its own and reveals nothing beyond the actor's own access. The relation lists are the inspector whitelist for `platform` and `tenant`.

`GET /healthz` returns 200 `{"status":"ok"}` with no auth. It does not touch OpenFGA.

#### Users

System admin (`can_manage_platform`).

- `GET /api/v1/users` returns every user. `is_system_admin` is true when a `platform_admins` row exists.
- `POST /api/v1/users` body `{email, name, password, company_id}` creates an identity in one company. 201 and the user object (with `company`). No tuple. Email is unique case-insensitively (409).
- `PATCH /api/v1/users/:id` body `{name}` updates the display name.

Validation: email contains `@`, name length 1 to 200 after trim, password length 8 to 72, `company_id` an existing company.

Every place that shows a person (users, members, project members) includes `company: {id, name}`.

#### Tenants

- `GET /api/v1/tenants` lists every tenant. Check `can_manage_platform`. Include each tenant's products from `tenant_products`.
- `POST /api/v1/tenants` body `{name, slug}`. Slug matches `^[a-z0-9-]{2,40}$`. Check `can_manage_platform`. Insert products disabled. 201.
- `GET /api/v1/tenants/mine` is ListObjects `can_read_tenant` for the actor, plus Postgres rows and the actor's `affiliation` and `role` labels. Any authenticated user.
- `GET /api/v1/tenants/:id` is allowed by the "get one tenant" OR check.
- `PATCH /api/v1/tenants/:id` body `{name}`. Check `can_manage_platform`.

There is no delete tenant route.

#### Products

- `GET /api/v1/tenants/:id/products` returns `{product, enabled, updated_at}` for every product. OR check of `can_manage_platform` or `can_view_products` (internal admin); 403 otherwise. Tenant admins can see product state but not change it. `enabled` is the Postgres display flag. (Members learn whether a product is on from `GET /me/permissions`.)
- `PUT /api/v1/tenants/:id/products/:product` where `:product` is `documents`, `photos`, or `blueprints`. Body `{"enabled": true}`. Allow only when `can_manage_platform` is true: products are a platform entitlement. Then the set-product tuple command.

An internal tenant admin and a system admin can both call this PUT. An internal user, an external admin, and an external user receive 403. Disabling the product still deletes only the wildcard tuple.

#### Members

Routes do not require `X-Tenant-ID`. The tenant is the path id.

- `GET /api/v1/tenants/:id/members?affiliation=internal|external` returns `{user_id, email, name, affiliation, role, role_name?, company}`. `affiliation` is derived (internal iff the member's company is the tenant's own). The query parameter is optional. `internal` and `external` add `AND affiliation = $aff` after the authz check. Any other value is 400. OR check of `can_manage_platform` or `can_manage_members`; 403 otherwise. The roster is tenant administration: internal admins and the platform only. The share dialog does not use it; it uses the object's share candidates. The filter is display. It is not an authorization decision.
- `POST /api/v1/tenants/:id/members` body `{user_id | email, role, name?, password?, company_id?}`. The person is named by `user_id` or `email`. A known email reuses the one account; an unknown email creates it and needs `name`, `password` (8 to 72), and a `company_id` the tenant manages (its own company, or a subcontractor it created; the platform may use any). There is no `affiliation` field: it follows the company. `role` is a built-in role for that affiliation, or the id of a custom role with that affiliation. OR check of `can_manage_platform` or `can_manage_members`. 409 when the membership exists. 404 when the user id or company does not exist. 400 `affiliation_role_mismatch` for a crossed pair.
- `PATCH /api/v1/tenants/:id/members/:userId` body `{role}`. Same OR check. The role must belong to the current affiliation, built-in or custom. Role-change tuples. There is no field for affiliation. The response includes `role_name` when the role is custom.
- `DELETE /api/v1/tenants/:id/members/:userId`. Same OR check. Remove-member tuples, including that person's custom-role assignee tuples.

#### Companies

Routes do not require `X-Tenant-ID`.

- `GET /api/v1/companies` lists every company with `{id, name, tax_id?, created_by_tenant_id?, tenant_id?, claimed, claimed_at, member_count}`. `can_manage_platform`.
- `GET /api/v1/tenants/:id/companies` lists the tenant's own company and the subcontractor companies it created. OR check of `can_manage_platform` or `can_manage_members`.
- `POST /api/v1/tenants/:id/companies` body `{name, tax_id?}` creates an unclaimed subcontractor company on the tenant's behalf. 201. 409 on a duplicate name for that tenant. Same OR check.
- `POST /api/v1/tenants/:id/company-claims` body `{company_ids: string[]}` merges the unclaimed companies into the tenant's own company (the platform has verified ownership). `can_manage_platform`. 409 when a company is already claimed or one of its users is already a member of the tenant. Returns the tenant's company.

#### Projects

Require `X-Tenant-ID`. Rows carry the caller's `permissions` (`can_view`, `can_manage_members`, `can_contribute`, `can_create_folder`, `can_create_album`, `can_create_private_folder`, `can_create_private_album`).

- `GET /api/v1/projects` is ListObjects `can_view` on `project`, then a Postgres load. Internal members see every project of the tenant; an external member sees the projects they belong to.
- `POST /api/v1/projects` body `{name}`. OR check of `can_manage_platform` or `can_manage_projects`. 201. 409 on a duplicate name in the tenant.
- `GET /api/v1/projects/:id` needs `can_view` (404 when denied). `PATCH /api/v1/projects/:id` body `{name}` needs the create check.
- `GET /api/v1/projects/:id/members`, `GET /api/v1/projects/:id/member-candidates`, `GET /api/v1/projects/:id/roles`: OR check of `can_manage_platform` or `can_manage_members` on the project. Member rows are `{user_id, email, name, affiliation, role, role_name?, company}`; candidates are the tenant members not yet in the project.
- `POST /api/v1/projects/:id/members` body `{user_id, role}`. The person must be a tenant member (400 `not_a_member`). `role` is a built-in role of their affiliation or a project role of it (400 `affiliation_role_mismatch` otherwise). 409 when already a member. `PATCH /api/v1/projects/:id/members/:userId` body `{role}`. `DELETE` removes the project membership only.
- `POST /api/v1/projects/:id/roles` body `{name, affiliation}`, `PUT /api/v1/projects/:id/roles/:roleId/permissions` body `{permissions}` (same catalog as tenant roles), `DELETE /api/v1/projects/:id/roles/:roleId` (409 while held). `can_manage_platform` only.

A project role response is `{id, tenant_id, project_id, name, affiliation, permissions, assignees, created_at}`.

#### Custom roles

Routes do not require `X-Tenant-ID`. The tenant is the path id. Listing and getting a role is allowed for `can_manage_platform` or `can_manage_members`, so the member form can offer them. Creating, renaming, setting permissions, and deleting stay `can_manage_platform` only.

A role has `affiliation` `internal` or `external`, fixed at creation. A member holds one role. `POST` and `PATCH` on members accept a built-in role name or a custom role id of the member's affiliation. A crossed pair is 400 `affiliation_role_mismatch`. Assigning a custom role deletes the built-in role tuple and writes `tenant_role:<id>#assignee@user`. Assigning a built-in role deletes the assignee tuple.

Delete is 409 when any member still holds the role.

The permission catalog is fixed: `view`, `edit`, `manage`, `change_visibility`, `contribute`, `create_private`, `manage_members`, `view_products`. An unknown name is 400 `validation`. The response lists permissions in that order. Implications (`manage` includes edit and view, and so on) live in the model, so the stored list is exactly what was assigned.

A role response is `{id, tenant_id, name, affiliation, permissions, assignees, created_at}`. `assignees` is `[{user_id, email, name}]` for members whose current role is this one.

- `GET /api/v1/tenants/:id/roles` lists the tenant's custom roles. 404 when the tenant does not exist.
- `GET /api/v1/tenants/:id/roles/:roleId` returns one role.
- `POST /api/v1/tenants/:id/roles` body `{name, affiliation}`. 201. `affiliation` is `internal` or `external`. Name length is 1 to 200. The four built-in role ids are reserved (400). A duplicate name in the tenant is 409. The role starts with no permissions.
- `PATCH /api/v1/tenants/:id/roles/:roleId` body `{name}`. Display only; no tuple changes. Affiliation does not change.
- `PUT /api/v1/tenants/:id/roles/:roleId/permissions` body `{permissions: string[]}`. `permissions` is required; `[]` clears the role. Replaces the set. Removed grants are deleted in OpenFGA before Postgres; added grants are inserted in Postgres first.
- `DELETE /api/v1/tenants/:id/roles/:roleId`. 204. 409 when a member still holds it. Otherwise deletes the role's tuples, then the row. Permissions cascade.

#### Documents and folders

Require `X-Tenant-ID`.

- `GET /api/v1/folders?project_id=<optional>`
- `POST /api/v1/folders` body `{project_id, name, parent_id?, is_private}`. `project_id` is required for a top-level folder (400 otherwise) and must match the parent's when `parent_id` is set. `is_private` is required (400 when missing). `parent_id` null creates a top-level folder. Checks per the decision catalog. Project and parent must be in the header tenant.
- `PUT /api/v1/folders/:id/visibility` body `{is_private}`. Check `can_change_visibility`.
- `GET /api/v1/folders/:id`
- `PATCH /api/v1/folders/:id` body `{name}`. Check `can_manage` on the folder.
- `DELETE /api/v1/folders/:id`. Check `can_delete`.
- `GET /api/v1/documents?folder_id=<optional>&project_id=<optional>`
- `POST /api/v1/documents` body `{folder_id, name, mime_type, byte_size}`. Check `can_create_document` on the folder. This is the upload command. It stores metadata and does not accept a multipart body.
- `GET /api/v1/documents/:id`
- `PATCH /api/v1/documents/:id` body `{name}`
- `DELETE /api/v1/documents/:id`
- `POST /api/v1/folders/:id/grants` and `POST /api/v1/documents/:id/grants` body `{relation, subject, user_id?}`
  - Specific person: `{relation: "viewer"|"editor", subject: "user", user_id}`. Editor requires the target's `can_contribute`.
  - All external users: `{relation: "viewer", subject: "external_users"}`
  - All external admins: `{relation: "viewer"|"editor", subject: "external_admins"}`
  - Shares exist for external members; internal members already see everything.
- `DELETE /api/v1/folders/:id/grants` and `DELETE /api/v1/documents/:id/grants` with the same body shape, identifying the grant to remove.
- `GET /api/v1/folders/:id/share-candidates` and `GET /api/v1/documents/:id/share-candidates` return the external members of the object's tenant as `{user_id, email, name, affiliation, role}`. Check `can_share` on the object. This lets an owner share without reading the full roster.
- `GET /api/v1/folders/:id/grants` and `GET /api/v1/documents/:id/grants` list the `resource_grants` rows as `{relation, subject, user_id, name}`. Check `can_share`. The share dialog uses this to show and revoke existing grants.

`GET /documents` returns every visible document in the tenant (or project) when `folder_id` is omitted, including documents whose parent folder the caller cannot view. The UI uses that response for a "Shared with me" group.

Name length is 1 to 200. `mime_type` length is 1 to 200. `byte_size` is an integer greater than or equal to 0.

Folder and document JSON includes `id`, `tenant_id`, `project_id`, `name`, `created_by`, `created_at`, plus `parent_id` and `is_private` for folders or `folder_id` for documents, and for documents `mime_type` and `byte_size`.

List and get-by-id responses also include `permissions`: the actor's result for every content relation in the inspector whitelist for that type (for a folder: `can_view`, `can_edit`, `can_manage`, `can_delete`, `can_share`, `can_change_visibility`, `can_create_document`). The service computes them with one BatchCheck per page (`Authorizer.Permissions`). Create and rename responses omit it.

#### Photos and albums

Same routes and checks, with these replacements:

| Documents | Photos |
| --- | --- |
| `/folders` | `/albums` |
| `/documents` | `/photos` |
| `parent_id` on folder | `parent_id` on album |
| `folder_id` | `album_id` |
| `can_create_folder` (on the project) | `can_create_album` (on the project) |
| `can_create_document` | `can_create_photo` |
| `documents_enabled` | `photos_enabled` |

Grant routes: `POST` and `DELETE /api/v1/albums/:id/grants` and `/api/v1/photos/:id/grants`.

#### Blueprints

Require `X-Tenant-ID`. Blueprints live directly in a project (no container) and have no public flag: external members reach one only through a share or a custom-role `grant_*`. External admins who are project members may create blueprints (they own those). Every route calls the Go API, which checks, stores, and versions; the API alone calls the stateless `blueprint-engine` (`BLUEPRINT_ENGINE_URL`). The LLM edits only the brief; the engine does all geometry.

- `POST /api/v1/blueprint-drafts` body `{project_id, prompt}` returns `{brief, notes}`. Check `can_create_blueprint` on the project. Writes nothing: the UI shows the brief for review.
- `POST /api/v1/blueprint-drafts/sketch` multipart `project_id`, `file` (PNG, JPG, or PDF; first page only; at most 10 MB), `note?` returns `{brief, notes, source: {name, mime_type, byte_size}}`. Same check. The file is sent to the engine and discarded; only its name, type, and size are kept when the blueprint is created.
- `POST /api/v1/blueprints` body `{project_id, name, brief, source_kind: "prompt"|"sketch"|"form", source?: {name, mime_type, byte_size}, prompt?}`. `prompt` (at most 2000 characters) is the description or sketch note the brief was drafted from; it is stored as version 1's `message` so it starts the chat history. Check `can_create_blueprint` on the project. The engine generates (shrinking rooms toward their minimums when the brief is too big); a brief that still cannot be built is 400 `validation` with the engine's issues and nothing is stored. Otherwise creates the blueprint and version 1. 201 with the blueprint and `version` (the get shape, without `permissions`).
- `GET /api/v1/blueprints?project_id=<optional>` lists visible blueprints (ListObjects `can_view`), without brief or model.
- `GET /api/v1/blueprints/:id` returns the blueprint, its current version (`brief`, `model`, `checks`, `notes`, `ok`), and `permissions`.
- `PATCH /api/v1/blueprints/:id` body `{name}`. Check `can_edit`.
- `DELETE /api/v1/blueprints/:id`. Check `can_delete`. Deletes every version.
- `POST /api/v1/blueprints/:id/revisions` body `{message, base_version}`. Check `can_edit`. The engine turns the message into edit operations on the brief and regenerates, keeping unchanged rooms where they were. It also receives the chat history: the messages of the last 6 `create` and `revise` versions, oldest first, so "a bit bigger please" refers to the previous request. A wrong LLM answer is sent back to the LLM with the error, up to 3 attempts; an unsupported or still unresolvable request is 400 with the engine's message. When the edit introduces a new kind of error, the engine runs one review round (below) before answering. 201 with the blueprint and its new `version`.
- `POST /api/v1/blueprints/:id/reviews` body `{base_version}`. Check `can_edit`. "Fix issues with AI": the engine's reviewer runs up to 3 rounds over the failed checks of the current version, each round proposing edit operations that are kept only when the issue score drops. It never removes or renames rooms or changes the lot or the entrance, and moves the total area by at most 15%. 201 with a new version of kind `review` whose `notes` list each round and the issues left for the person (`message` is empty). 400 when there is nothing to fix or no round reduced the issues (`"no fix found: <round notes>"`); nothing is stored.
- `PUT /api/v1/blueprints/:id/brief` body `{brief, base_version}`. Check `can_edit`. The form edit: regenerate without changing the user's numbers (a brief that cannot be built is 400). 201 with the blueprint and its new `version`.
- `GET /api/v1/blueprints/:id/versions` lists `{version, kind, message, ok, created_by, created_at}`. Check `can_view`.
- `POST /api/v1/blueprints/:id/versions/:version/restore` body `{base_version}`. Check `can_edit`. Copies an earlier version into a new one without calling the engine. 201 with the blueprint and its new `version`.
- `GET /api/v1/blueprints/:id/drawing?format=svg|pdf|dxf|ifc&version=<optional>`. Check `can_view`. Renders the stored model of that version (default: current) at 1:50 on the smallest sheet that fits (A3, growing to A2, A1 or A0 for bigger houses; a smaller scale only past A0); `pdf`, `dxf` and `ifc` are attachments named `<name>-v<version>.<ext>`. `ifc` is an IFC4 BIM model (site, building, one storey, walls, doors, windows, spaces, a floor slab and the fixtures as furniture and sanitary terminals) that opens in any IFC viewer.
- Grants, grant list, and share candidates: the same routes and body as documents, under `/api/v1/blueprints/:id/...`. Check `can_share`.

`base_version` must equal the current version, otherwise 409 `conflict`; nothing is written. The engine is called only after the Check and the version check pass, and before anything is written. An engine or LLM that is down is 502 `engine_unavailable` (for sketches, also when the vision model is not pulled). Versions are append-only Postgres rows; no version route touches OpenFGA.

Blueprint JSON: `id`, `tenant_id`, `project_id`, `name`, `current_version`, `source_kind`, `source` (or null), `created_by`, `created_at`, `updated_at`; list and get add `permissions` (`can_view`, `can_edit`, `can_manage`, `can_delete`, `can_share`). A version adds `version`, `kind` (`create`, `revise`, `review`, `edit`, `restore`), `message`, `brief`, `model`, `checks`, `notes`, `ok`.

#### Access inspector

`POST /api/v1/authz/check`

```json
{
  "user_id": "<uuid>",
  "relation": "can_view",
  "object_type": "folder",
  "object_id": "<uuid>"
}
```

Response 200: `{"allowed": false}`.

The actor may check their own `user_id` always. The actor may check another user when `can_manage_platform` is true, or when the actor has `can_manage_members` on the object's tenant. The API derives that tenant from the object, never from the request: a `tenant` object is its own tenant, a content object's tenant is loaded from Postgres, and `platform:main` has no tenant. A missing object is 404. Otherwise 403. This keeps an admin of one tenant from probing access to another tenant's objects.

Whitelisted relations: `can_view`, `can_edit`, `can_delete`, `can_share`, `can_create_document`, `can_create_photo`, `can_create_folder`, `can_create_album`, `can_manage`, `can_manage_members`, `can_view_products`, `can_contribute`, `can_read_tenant`, `can_manage_platform`, `documents_enabled`, `photos_enabled`, `blueprints_enabled`, `can_create_blueprint`.

`can_manage_platform` requires `object_type=platform` and `object_id=main`. Tenant relations require `object_type=tenant`. Content relations require the matching object type. A mismatch is 400.

`POST /api/v1/authz/batch-check` body `{"checks":[...]}` with the same item shape, maximum 50. Same per-item authorization rule. Response `{"results":[{"allowed":true}]}` in request order. The UI uses this to hide buttons.

`GET /api/v1/access?object_type=document&object_id=<uuid>`

Caller must pass at least one of: `can_manage_platform`, `can_share` on the object, or `can_manage_members` on the object's tenant. Load the object's tenant from Postgres. A missing object is 404.

Response:

```json
{
  "object": "document:<uuid>",
  "direct_tuples": [
    {"user": "user:<uuid>", "relation": "owner", "object": "document:<uuid>"}
  ],
  "members": [
    {"user_id": "<uuid>", "name": "Bob", "can_view": true, "can_edit": true, "can_delete": true}
  ]
}
```

`direct_tuples` is OpenFGA Read for that object. `members` is the Postgres member list of the object's tenant, plus every platform admin, each evaluated with BatchCheck for `can_view`, `can_edit`, and `can_delete`. This stays accurate when the product wildcard removes access and the owner tuple remains.

Do not add a ListUsers requirement. Read plus BatchCheck is the inspector.

