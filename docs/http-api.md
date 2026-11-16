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

Require `X-Tenant-ID`. Blueprints live directly in a project (no container) and have no public flag: external members reach one only through a share or a custom-role `grant_*`. External admins who are project members may create blueprints (they own those). Every route calls the Go API, which checks, stores, and versions; the API alone calls the stateless `blueprint-engine` (`BLUEPRINT_ENGINE_URL`). The brief is only the input of a create: the engine draws it into a DXF and the blueprint is stored like an import (no brief, the DXF as its original); the engine does all geometry, and later edits are explicit plan operations.

- `POST /api/v1/blueprint-drafts` body `{project_id, prompt}` returns `{brief, notes}`. Check `can_create_blueprint` on the project. Writes nothing: the UI shows the brief for review.
- `POST /api/v1/blueprint-drafts/dxf-import` multipart `project_id`, `file` (`.dxf`, `.pdf`, `.png`, `.jpg`/`.jpeg`, at most 20 MB; the extension decides the type), `unit?` (DXF only; `mm|cm|m|in|ft`; omitted = the file's `$INSUNITS`, else guessed from its extent), `longest_m?` and `scale?` (PDF or picture only: the real length of the drawing's longest side in metres (1 to 500), else the paper scale N of 1:N (1 to 5000); `longest_m` wins; neither: a PDF is 1:100; a picture's scale is read from it (the `xx m²` room-area labels, else the overall dimension written on it, else door widths of 0.9 m), and when none can be read the request is 400 asking for `longest_m`) returns `{conversion, plans: [{index, name, room_count, net_area, ok, checks, svg}], report: {units, unit_source, layers_used, layers_ignored, plans_found, doors, windows, unlabeled_rooms, unmapped_texts, fixtures, skipped, warnings}, source}`. Check `can_create_blueprint` on the project. No LLM and nothing is stored. A PDF or picture is first converted to a DXF by the engine (`POST /v1/convert/dxf`) and that DXF is then read like any other; `conversion` is `null` for a DXF, else `{method: "pdf-vector"|"raster-trace", scale_source, metres_across, walls, openings, texts, skipped, warnings}`. A vector PDF is read exactly (filled dark shapes are walls, curved strokes door swings, text is kept); a scan or picture is traced (thick dark strokes are walls; openings are found as door-sized gaps between wall ends and marked as doors; door or window is decided per gap (a straight leaf stroke from a gap end means a door; otherwise a gap in the outside wall is a window); common furniture and sanitary ware are recognised by size, proportion and the room's OCR name (bed, wardrobe, sofa, dining table, stove, counter, toilet, ... but not every piece, and not ones touching a wall) and written as blocks; outlines are squared up when within a few degrees of horizontal/vertical; coloured strokes count as ink for windows and furniture but never as walls; the text is read with OCR (RapidOCR, local, no LLM): names become room labels and area labels set the scale); a warning says when the result is not a plausible plan size (3 to 80 m across); furniture and sanitary blocks the model supports (KIND attribute, else block or layer name) are kept as fixtures, and `report.skipped` names what is not imported (dimensions, other blocks, tiny spaces) with counts; one plan per cluster of walls in the file (a sheet may hold several). A file with no floor plan (no wall linework, or walls that enclose no room) is imported as one read-only drawing plan (`room_count` 0, `net_area` 0, no checks, the model's `drawing` holds all its entities; the report's `warnings` say why); every other entity of a floor-plan file is kept the same way beside the rooms. An empty or unreadable file, or one with nothing drawable, is 400 with the reason.
- `POST /api/v1/blueprints/import` multipart `project_id`, `name`, `file`, `plan?` (1-based index from the preview, default 1), `unit?`, `longest_m?`, `scale?` (as in the preview) returns 201 with the blueprint detail. Same check; the engine reads the file again (deterministic) before any write. Version 1 has `kind: "import"`, `brief: null` and the plan as `model`; `source_kind` is `import`. The original is stored with the version (see `blueprint_originals`): the uploaded DXF, or for a PDF or picture the DXF the engine converted (named `<file>.dxf`; the picture itself is dropped). `source_name`, `source_mime` and `source_size` describe the upload, and the conversion report lines go to the version's `notes`.
- `GET /api/v1/blueprints/:id/source?version=` downloads the stored original (`attachment`, `nosniff`, no-store). The file belongs to the blueprint (stored with version 1 for an import or a create, or with the converting version, see `convert`), so an omitted `version` means the newest stored file, not the latest edit. Check `can_edit`, not `can_view`: an original can hold more than the extracted plan. 404 when the version has no original.
- `GET /api/v1/blueprints/:id/source/edited?version=` downloads the original file with the edits of that version (default current) written back into it (`attachment`, `nosniff`, no-store). Check `can_edit`, like the original. The engine compares the plan as it was when the file was stored (the version that holds it) with the chosen version; a `version` older than that is 400 and applies each difference to the file itself: it moves, turns, resizes or deletes the entities that draw a fixture, door or window, renames room labels, and closes the wall across a removed opening; layers, blocks, dimensions, hatches and every other entity stay as they were. An added fixture is copied from a block of the same kind in the file, else drawn as a rectangle on the layer `FGA-ADDED`. Changes a CAD file cannot hold (a room type, a flipped swing, moved walls) are not dropped silently: the response header `X-Export-Report` carries base64 JSON `{applied, added, skipped}` naming each (at most 20 lines per list, then "… and N more", to fit a header). When nothing was written the original is returned byte for byte. 404 when the blueprint has no original; 502/400 as for the engine. Nothing is stored.
- `POST /api/v1/blueprints/:id/plan-edits` body `{ops: {ops: [...]}, base_version}`. Check `can_edit`, then the `base_version` test (409), then the engine; one write after it. No LLM. Only an imported plan (`brief: null`): a plan with a brief is 400. Operations name rooms and openings by id: `rename_room {room, name}`, `set_room_type {room, type}`, `add_fixture {room, fixture}`, `delete_fixture {fixture}`, `move_fixture {fixture, dx, dy}` (metres, east and north positive), `rotate_fixture {fixture, facing}`, `resize_opening {opening, width}`, `move_opening {opening, offset}` (along the wall), `remove_opening {opening}`. The engine refuses what does not fit (a fixture leaving its room or overlapping another, an opening off its wall or onto another, removing the entrance) with 400 and the reason, and stores nothing. 201 with the blueprint and the new `version` (kind `edit`, brief `null`, checks re-run, `message` = what changed).
- `POST /api/v1/blueprints/:id/convert` body `{base_version}`. Check `can_edit`, then the `base_version` test (409), then the engine; one write after it. For a blueprint made before DXF plans (its current version still has a brief; a DXF plan is 400 "already a DXF plan"). The engine draws the stored plan into a DXF and reads it back (no LLM, `POST /v1/plan/draw`); the result must keep the plan's area (3%) and fixtures or it is 400 with the reason and nothing is stored. Rooms with no wall between them (an open plan) read back as one space, which is listed in the version's `notes`. Stores a new version (kind `edit`, brief `null`, message "converted to a DXF plan") and the drawn DXF as its original in one transaction; earlier versions keep their briefs. 201 with the blueprint and the new `version`. From then on the blueprint is edited, chatted about and exported like an import.
- `POST /api/v1/blueprints` body `{project_id, name, brief, source_kind: "prompt"|"form", prompt?}`. `prompt` (at most 2000 characters) is the description the brief was drafted from; it is stored as version 1's `message` so it starts the chat history. Check `can_create_blueprint` on the project. The engine generates (shrinking rooms toward their minimums when the brief is too big); a brief that still cannot be built is 400 `validation` with the engine's issues and nothing is stored. Otherwise the engine draws the plan into a DXF and reads it back, and the blueprint is stored like an import: version 1 (`kind: "create"`) has `brief: null` and that plan as `model`, `source_kind` stays `prompt` or `form` (`sketch` only on blueprints made before uploads were imported), and the drawn DXF is stored as the original (named `<name>.dxf`; see `blueprint_originals`) in the same transaction. So a created plan is edited, chatted about and exported like an imported one (`plan-edits`, `revisions`, `source`, `source/edited`); the brief is not kept. 201 with the blueprint and `version` (the get shape, without `permissions`). Blueprints created before this change keep their brief and edit through it.
- `GET /api/v1/blueprints?project_id=<optional>` lists visible blueprints (ListObjects `can_view`), without brief or model.
- `GET /api/v1/blueprints/:id` returns the blueprint, its current version (`brief`, `model`, `checks`, `notes`, `ok`), and `permissions`.
- `PATCH /api/v1/blueprints/:id` body `{name}`. Check `can_edit`.
- `DELETE /api/v1/blueprints/:id`. Check `can_delete`. Deletes every version.
- `POST /api/v1/blueprints/:id/revisions` body `{message, base_version}`. Check `can_edit`. The engine turns the message into edit operations on the brief and regenerates, keeping unchanged rooms where they were. It also receives the chat history: the messages of the last 6 `create` and `revise` versions, oldest first, so "a bit bigger please" refers to the previous request. A wrong LLM answer is sent back to the LLM with the error, up to 3 attempts; an unsupported or still unresolvable request is 400 with the engine's message. When the edit introduces a new kind of error, the engine runs one review round (below) before answering. 201 with the blueprint and its new `version`. On an imported plan (`brief: null`) the same route and the `/revisions/stream` route edit the model as drawn: the engine turns the message into plan operations (`rename_room`, `set_room_type`, `add_fixture`, `remove_fixture`, `resize_opening`, `move_opening`, `remove_opening`, `move_fixture`, `rotate_fixture`, `delete_fixture`); moving walls or rooms is 422 `invalid_edit` (not supported yet). The version keeps `brief: null`. The review and brief routes still return 400 for an imported plan.
- `POST /api/v1/blueprints/:id/revisions/stream` same body and Checks as `revisions`, answered as Server-Sent Events so the chat can show the work live. Every refusal that happens before the engine starts (400 body, 403, 404, 409 stale `base_version`, engine down) is the usual JSON error with its status, and the stream never opens. Once it opens (status 200, `text/event-stream`): `event: progress` with `{kind, stage, message}` (`kind: "step"` with stages `ask_model`, `retry`, `ops`, `layout`, `checks`, `review`; `kind: "thinking"` carries a piece of the model's reasoning in `message`), then exactly one `event: done` with the same body as the 201 of `revisions` (the new version is stored just before it), or one `event: error` with `{error, message?, status}` (the same codes as the JSON errors; nothing is stored). Progress and thinking are shown, never stored. Closing the connection cancels the engine call.
- `POST /api/v1/blueprints/:id/reviews` body `{base_version}`. Check `can_edit`. "Fix issues with AI": the engine's reviewer runs up to 3 rounds over the failed checks of the current version, each round proposing edit operations that are kept only when the issue score drops. It never removes or renames rooms or changes the lot or the entrance, and moves the total area by at most 15%. 201 with a new version of kind `review` whose `notes` list each round and the issues left for the person (`message` is empty). 400 when there is nothing to fix or no round reduced the issues (`"no fix found: <round notes>"`); nothing is stored.
- `PUT /api/v1/blueprints/:id/brief` body `{brief, base_version}`. Check `can_edit`. The form edit: regenerate without changing the user's numbers (a brief that cannot be built is 400). 201 with the blueprint and its new `version`.
- `GET /api/v1/blueprints/:id/versions` lists `{version, kind, message, ok, created_by, created_at}`. Check `can_view`.
- `POST /api/v1/blueprints/:id/versions/:version/restore` body `{base_version}`. Check `can_edit`. Copies an earlier version into a new one without calling the engine. 201 with the blueprint and its new `version`.
- `GET /api/v1/blueprints/:id/drawing?format=svg|pdf|dxf|ifc&version=<optional>`. Check `can_view`. Renders the stored model of that version (default: current) at 1:50 on the smallest sheet that fits (A3, growing to A2, A1 or A0 for bigger houses; a smaller scale only past A0); `pdf`, `dxf` and `ifc` are attachments named `<name>-v<version>.<ext>`. `ifc` is an IFC4 BIM model (site, building, one storey, walls, doors, windows, spaces, a floor slab and the fixtures as furniture and sanitary terminals) that opens in any IFC viewer.
- Grants, grant list, and share candidates: the same routes and body as documents, under `/api/v1/blueprints/:id/...`. Check `can_share`.

**Cost estimation** (a chat with an AI agent; `prds/plans/cost-estimation.md`). A session is pinned to the blueprint version it was created on and holds the agent's state (facts the person gave or agreed to assume, the condition of each room, researched unit rates, the latest estimate) as JSON that Go does not read, plus the messages.

- `POST /api/v1/blueprints/:id/estimate-session` (no body). Check `can_edit` (403): a session spends LLM and web-search calls. A blueprint has **one** session, keyed by the blueprint: this starts a fresh one and **replaces** the old (the row is reset and its messages deleted in one transaction; nothing changes if that fails). Finished estimates are kept. A turn that was running on the old session ends in 409 and saves nothing (an `epoch` counts restarts). 201 with the session (`blueprint_id, blueprint_version, status, turns, state, messages: [], created_by, created_at, updated_at`; `status` is read from the agent's state). `status` is `interviewing`, `researching`, `ready` or `failed`.
- `POST /api/v1/blueprints/:id/estimate-session/messages` body `{message?, rate_overrides?: {rate_code: number}}` (a message of at most 2000 characters, or at least one override; a rate is a positive VND amount per the line's unit). Check `can_edit` on the blueprint (403; another tenant's blueprint is 404), 404 when it has no session yet, a turn cap of 40 per session (400), then the engine, then one write. Answered as Server-Sent Events, like `revisions/stream`: every refusal before the engine starts is the usual JSON error and the stream never opens. Once it opens (200, `text/event-stream`): `event: progress` `{kind, stage, message}`; `token` `{text}` (a delta of the agent's reply); `fact` `{name, value, source}` (`source` is `user`, `plan` or `assumed`; room conditions are `scope.<room_id>`); `question` `{id, text, options[], allow_unknown}` (quick replies, sent once the reply text finished); `rate` `{code, low, base, high, unit, confidence, sources[]}` as each unit price is found; `estimate` (the priced result); then exactly one `done` with the saved session (the get shape below) or one `error` `{error, message?, status}`. The user message, the assistant message (the full streamed text), the new state and any estimate are written together after the engine finishes, guarded by the session's turn counter and epoch (409 when another turn finished first or the estimate was restarted meanwhile); closing the connection cancels the engine run and saves nothing, so the session never holds half an answer and the person resends. Events are shown, never stored; reloading shows the saved messages, not a live stream.
- `GET /api/v1/blueprints/:id/estimate-session` the blueprint's one session: `blueprint_id, blueprint_version, status, turns, state, messages: [{seq, role, content, created_at}]`, 404 when none was started. Needs `can_edit` and answers 404 when it fails (the chat is not visible to a viewer, who reads the finished estimates instead).
- `GET /api/v1/blueprints/:id/estimates` the finished estimates (`id, version, total_low, total_base, total_high, created_at`) and `GET /api/v1/blueprints/:id/estimates/:eid` one with its `result`: lines (quantity, unit, basis, confidence `measured|derived|assumed`, rate low/base/high), package and direct totals, the markup cascade (overhead, profit, design, contingency, VAT, each with its base), the low/base/high total, assumptions, warnings, and every rate with its sources (`url`, quote, date). Check `can_view` (404 when denied): an estimate derives from geometry a viewer can already see. Estimates are append-only; one made on an older plan version is shown as stale by the web (`version` below `current_version`).

A DXF blueprint (imported or created; current version `brief: null`) answers 400 to `reviews` and `PUT …/brief` (no program to regenerate); `revisions`, `revisions/stream` and `plan-edits` edit the plan as drawn (moving walls is a later phase), and `source/edited` writes those edits back into the original file. Restore, rename, share, delete and drawings work as usual.

`base_version` must equal the current version, otherwise 409 `conflict`; nothing is written. The engine is called only after the Check and the version check pass, and before anything is written. An engine or LLM that is down is 502 `engine_unavailable`. Versions are append-only Postgres rows; no version route touches OpenFGA.

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

