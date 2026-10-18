# HTTP API

Routes, error catalog, and payloads. Handlers live in `api/internal/handler/http`.


Base path `/api/v1`. JSON uses snake_case. Timestamps are RFC3339. Authenticated routes require `Authorization: Bearer <jwt>`. Creates return 201 with the created object. Deletes and revokes return 204 with no body.

#### Error catalog

| Status | Code | When |
| --- | --- | --- |
| 400 | `validation` | Malformed JSON, bad UUID, empty name, bad slug, bad role, bad product, owner grant, unknown affiliation |
| 400 | `affiliation_role_mismatch` | Internal role on an external member, external role on an internal member, or a role change that would switch affiliation |
| 400 | `not_a_member` | Share target fails `can_read_tenant` |
| 400 | `cannot_upload` | Editor grant targets an external user, or subject `external_users` is combined with relation `editor` |
| 401 | `unauthenticated` | Missing or invalid JWT |
| 403 | `forbidden` | A required Check returned false on a write |
| 404 | `not_found` | Unknown id, or a GET-by-id whose Check returned false, or `X-Tenant-ID` does not own the object |
| 409 | `conflict` | Duplicate email or slug, duplicate membership, last internal admin, cycle, depth, folder or album still has children |
| 500 | `tuple_write_failed` | OpenFGA Write failed and compensation ran |
| 500 | `internal` | Any other unexpected error |

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
- `POST /api/v1/users` body `{email, name, password}` creates an identity. 201 and the user object. No tuple.
- `PATCH /api/v1/users/:id` body `{name}` updates the display name.

Validation: email contains `@`, name length 1 to 200 after trim, password length 8 to 72.

#### Tenants

- `GET /api/v1/tenants` lists every tenant. Check `can_manage_platform`. Include each tenant's products from `tenant_products`.
- `POST /api/v1/tenants` body `{name, slug}`. Slug matches `^[a-z0-9-]{2,40}$`. Check `can_manage_platform`. Insert products disabled. 201.
- `GET /api/v1/tenants/mine` is ListObjects `can_read_tenant` for the actor, plus Postgres rows and the actor's `affiliation` and `role` labels. Any authenticated user.
- `GET /api/v1/tenants/:id` is allowed by the "get one tenant" OR check.
- `PATCH /api/v1/tenants/:id` body `{name}`. Check `can_manage_platform`.

There is no delete tenant route.

#### Products

- `GET /api/v1/tenants/:id/products` returns `{product, enabled, updated_at}` for both products. OR check of `can_manage_platform` or `can_view_products` (internal admin); 403 otherwise. Tenant admins can see product state but not change it. `enabled` is the Postgres display flag. (Members learn whether a product is on from `GET /me/permissions`.)
- `PUT /api/v1/tenants/:id/products/:product` where `:product` is `documents` or `photos`. Body `{"enabled": true}`. Allow only when `can_manage_platform` is true: products are a platform entitlement. Then the set-product tuple command.

An internal tenant admin and a system admin can both call this PUT. An internal user, an external admin, and an external user receive 403. Disabling the product still deletes only the wildcard tuple.

#### Members

Routes do not require `X-Tenant-ID`. The tenant is the path id.

- `GET /api/v1/tenants/:id/members?affiliation=internal|external` returns `{user_id, email, name, affiliation, role}`. The query parameter is optional. `internal` and `external` add `AND affiliation = $aff` after the authz check. Any other value is 400. OR check of `can_manage_platform` or `can_manage_members`; 403 otherwise. The roster is tenant administration: internal admins and the platform only. The share dialog does not use it; it uses the object's share candidates. The filter is display. It is not an authorization decision.
- `POST /api/v1/tenants/:id/members` body `{user_id, affiliation, role}`. `affiliation` is `internal` or `external`. `role` must be one of the two roles for that affiliation. OR check of `can_manage_platform` or `can_manage_members`. 409 when the membership exists. 404 when the user id does not exist. 400 `affiliation_role_mismatch` for a crossed pair.
- `PATCH /api/v1/tenants/:id/members/:userId` body `{role}`. Same OR check. The role must belong to the current affiliation. Role-change tuples. There is no field for affiliation.
- `DELETE /api/v1/tenants/:id/members/:userId`. Same OR check. Remove-member tuples.

#### Documents and folders

Require `X-Tenant-ID`.

- `GET /api/v1/folders`
- `POST /api/v1/folders` body `{name, parent_id?, is_private}`. `is_private` is required (400 when missing). `parent_id` null creates a top-level folder. Checks per the decision catalog. Parent must be in the header tenant.
- `PUT /api/v1/folders/:id/visibility` body `{is_private}`. Check `can_change_visibility`.
- `GET /api/v1/folders/:id`
- `PATCH /api/v1/folders/:id` body `{name}`. Check `can_manage` on the folder.
- `DELETE /api/v1/folders/:id`. Check `can_delete`.
- `GET /api/v1/documents?folder_id=<optional>`
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

`GET /documents` returns every visible document in the tenant when `folder_id` is omitted, including documents whose parent folder the caller cannot view. The UI uses that response for a "Shared with me" group.

Name length is 1 to 200. `mime_type` length is 1 to 200. `byte_size` is an integer greater than or equal to 0.

Folder and document JSON includes `id`, `tenant_id`, `name`, `created_by`, `created_at`, plus `parent_id` and `is_private` for folders or `folder_id` for documents, and for documents `mime_type` and `byte_size`.

List and get-by-id responses also include `permissions`: the actor's result for every content relation in the inspector whitelist for that type (for a folder: `can_view`, `can_edit`, `can_manage`, `can_delete`, `can_share`, `can_change_visibility`, `can_create_document`). The service computes them with one BatchCheck per page (`Authorizer.Permissions`). Create and rename responses omit it.

#### Photos and albums

Same routes and checks, with these replacements:

| Documents | Photos |
| --- | --- |
| `/folders` | `/albums` |
| `/documents` | `/photos` |
| `parent_id` on folder | `parent_id` on album |
| `folder_id` | `album_id` |
| `can_create_folder` | `can_create_album` |
| `can_create_document` | `can_create_photo` |
| `documents_enabled` | `photos_enabled` |

Grant routes: `POST` and `DELETE /api/v1/albums/:id/grants` and `/api/v1/photos/:id/grants`.

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

Whitelisted relations: `can_view`, `can_edit`, `can_delete`, `can_share`, `can_create_document`, `can_create_photo`, `can_create_folder`, `can_create_album`, `can_manage`, `can_manage_members`, `can_view_products`, `can_contribute`, `can_read_tenant`, `can_manage_platform`, `documents_enabled`, `photos_enabled`.

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

