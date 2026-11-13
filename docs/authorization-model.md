# Authorization model

The OpenFGA model, identifier format, and store bootstrap. `api/fga/model.fga` must equal the DSL block below exactly. Overview: [api/ARCHITECTURE.md](../api/ARCHITECTURE.md).

## Identifiers

Postgres primary keys and OpenFGA object ids are the same UUID strings, except the platform object id, which is the literal `main`.

OpenFGA object format is `<type>:<id>`. User format is `user:<uuid>`. The wildcard is `user:*`. Group-share usersets are `tenant:<uuid>#external_user` and `tenant:<uuid>#external_admin`. A tenant custom-role grant userset is `tenant_role:<uuid>#assignee`; a project role's is `project_role:<uuid>#assignee`. A container's `tenant`, `root`, and `public` relations point at the plain `tenant:<uuid>` object; its `project` relation points at `project:<uuid>`.

The fga infrastructure adds those prefixes. Services pass bare UUIDs, the platform id `main`, the relation name, and the object type.

Generate ids with UUIDv4, except the seed section, which uses fixed ids.

## OpenFGA model

Write this text to `api/fga/model.fga` exactly.

```fga
model
  schema 1.1

type user

type platform
  relations
    define system_admin: [user]
    define can_manage_platform: system_admin

type tenant_role
  relations
    define assignee: [user]
    define tenant: [tenant]

type tenant
  relations
    define internal_admin: [user]
    define internal_user: [user] or internal_admin
    define external_admin: [user]
    define documents_enabled: [user:*]
    define photos_enabled: [user:*]
    define blueprints_enabled: [user:*]
    define internal_role: [tenant_role]
    define external_role: [tenant_role]
    define custom_internal: assignee from internal_role
    define custom_external: assignee from external_role
    define external_user: [user] or external_admin or custom_external
    define grant_manage_members: [tenant_role#assignee]
    define grant_view_products: [tenant_role#assignee]
    define grant_view: [tenant_role#assignee]
    define grant_edit: [tenant_role#assignee]
    define grant_manage: [tenant_role#assignee]
    define grant_change_visibility: [tenant_role#assignee]
    define grant_contribute: [tenant_role#assignee]
    define grant_create_private: [tenant_role#assignee]
    define can_read_tenant: internal_user or external_user or custom_internal
    define can_manage_members: internal_admin or (grant_manage_members and can_read_tenant)
    define can_manage_projects: can_manage_members
    define can_view_products: internal_admin or (grant_view_products and can_read_tenant)
    define can_contribute: internal_user or external_admin or ((grant_contribute or grant_edit or grant_manage or grant_create_private) and can_read_tenant)

type project_role
  relations
    define assignee: [user]
    define project: [project]

type project
  relations
    define tenant: [tenant]
    define internal_admin: [user]
    define internal_user: [user] or internal_admin
    define external_admin: [user]
    define internal_role: [project_role]
    define external_role: [project_role]
    define custom_internal: assignee from internal_role
    define custom_external: assignee from external_role
    define external_user: [user] or external_admin or custom_external
    define grant_manage_members: [project_role#assignee]
    define grant_view_products: [project_role#assignee]
    define grant_view: [project_role#assignee]
    define grant_edit: [project_role#assignee]
    define grant_manage: [project_role#assignee]
    define grant_change_visibility: [project_role#assignee]
    define grant_contribute: [project_role#assignee]
    define grant_create_private: [project_role#assignee]
    define member: internal_user or external_user or custom_internal
    define can_access: can_read_tenant from tenant and (internal_user from tenant or custom_internal from tenant or member)
    define can_view: can_access
    define can_manage_members: can_manage_members from tenant or (can_access and (internal_admin or grant_manage_members))
    define can_contribute: can_access and (can_contribute from tenant or external_admin or grant_contribute or grant_edit or grant_manage or grant_create_private)
    define can_create_folder: can_contribute and documents_enabled from tenant
    define can_create_album: can_contribute and photos_enabled from tenant
    define can_create_private_folder: can_access and (internal_user from tenant or grant_create_private from tenant or grant_create_private) and documents_enabled from tenant
    define can_create_private_album: can_access and (internal_user from tenant or grant_create_private from tenant or grant_create_private) and photos_enabled from tenant
    define can_create_blueprint: can_contribute and blueprints_enabled from tenant

type folder
  relations
    define tenant: [tenant]
    define project: [project]
    define parent: [folder]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible or grant_view from tenant or grant_edit from tenant or grant_manage from tenant or grant_view from project or grant_edit from project or grant_manage from project) and can_access from project and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and (external_admin from tenant or external_admin from project)) or grant_edit from tenant or grant_manage from tenant or grant_edit from project or grant_manage from project) and can_contribute from project and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant or internal_admin from project or grant_manage from tenant or grant_manage from project) and can_contribute from project and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: (can_manage and internal_user from tenant) or ((grant_change_visibility from tenant or grant_change_visibility from project) and can_access from project and documents_enabled from tenant)
    define can_create_document: can_edit

type document
  relations
    define tenant: [tenant]
    define project: [project]
    define folder: [folder]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from folder
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from folder
    define can_view: (internal_user from tenant or viewer or external_visible from folder or grant_view from tenant or grant_edit from tenant or grant_manage from tenant or grant_view from project or grant_edit from project or grant_manage from project) and can_access from project and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from folder and (external_admin from tenant or external_admin from project)) or grant_edit from tenant or grant_manage from tenant or grant_edit from project or grant_manage from project) and can_contribute from project and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant or internal_admin from project or grant_manage from tenant or grant_manage from project) and can_contribute from project and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage

type album
  relations
    define tenant: [tenant]
    define project: [project]
    define parent: [album]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible or grant_view from tenant or grant_edit from tenant or grant_manage from tenant or grant_view from project or grant_edit from project or grant_manage from project) and can_access from project and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and (external_admin from tenant or external_admin from project)) or grant_edit from tenant or grant_manage from tenant or grant_edit from project or grant_manage from project) and can_contribute from project and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant or internal_admin from project or grant_manage from tenant or grant_manage from project) and can_contribute from project and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: (can_manage and internal_user from tenant) or ((grant_change_visibility from tenant or grant_change_visibility from project) and can_access from project and photos_enabled from tenant)
    define can_create_photo: can_edit

type photo
  relations
    define tenant: [tenant]
    define project: [project]
    define album: [album]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from album
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from album
    define can_view: (internal_user from tenant or viewer or external_visible from album or grant_view from tenant or grant_edit from tenant or grant_manage from tenant or grant_view from project or grant_edit from project or grant_manage from project) and can_access from project and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from album and (external_admin from tenant or external_admin from project)) or grant_edit from tenant or grant_manage from tenant or grant_edit from project or grant_manage from project) and can_contribute from project and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant or internal_admin from project or grant_manage from tenant or grant_manage from project) and can_contribute from project and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage

type blueprint
  relations
    define tenant: [tenant]
    define project: [project]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor
    define can_view: (internal_user from tenant or viewer or grant_view from tenant or grant_edit from tenant or grant_manage from tenant or grant_view from project or grant_edit from project or grant_manage from project) and can_access from project and blueprints_enabled from tenant
    define can_edit: (internal_user from tenant or editor or grant_edit from tenant or grant_manage from tenant or grant_edit from project or grant_manage from project) and can_contribute from project and blueprints_enabled from tenant
    define can_manage: (owner or internal_admin from tenant or internal_admin from project or grant_manage from tenant or grant_manage from project) and can_contribute from project and blueprints_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
```

How to read it:

- **Roles.** `internal_admin` implies `internal_user`; `external_admin` implies `external_user`. An admin satisfies the lower relation without a second tuple.
- **Custom roles.** A `tenant_role` (formerly `role`) is internal or external. Creating it writes `tenant_role:R#tenant@tenant:T` and `tenant:T#internal_role@tenant_role:R` or `tenant:T#external_role@tenant_role:R`. A member holds one role, built-in or custom, assigned the same way. A custom role replaces the built-in role tuple with `tenant_role:R#assignee@user:U`. An internal custom role satisfies `can_read_tenant` through `custom_internal` and does not become `internal_user`; its content access is the permissions on the role. An external custom role is included in `external_user`, so public content stays visible, and the role's permissions add to that. Permissions are `grant_*` usersets. `grant_manage` satisfies edit, view, and contribute; `grant_edit` satisfies view and contribute; `grant_create_private` satisfies contribute and private creation. A custom role never crosses affiliation.
- **Projects.** `project` sits between the tenant and content: `tenant -> project -> items`. Every container, document, and photo carries a `project` tuple. A project has the same shape as a tenant for roles: the four built-in relations (`internal_admin`, `internal_user`, `external_admin`, plus `external_user`), `project_role` objects (`project_role:R#project@project:P`, `project:P#internal_role|external_role@project_role:R`, `project_role:R#assignee@user`), and the same eight `grant_*` usersets. A project role is scoped to one project, so its grants apply there only.
- **Union of tenant and project roles.** Every content `can_*` ORs the tenant grant with the project grant (`grant_view from tenant or grant_view from project`, likewise edit and manage; `internal_admin from tenant or internal_admin from project` for manage; `external_admin from tenant or external_admin from project` for editing public content) and is gated by `can_access from project` (view) or `can_contribute from project` (edit, manage). Tenant role grants a and b, project role grants c: the member has a, b, and c in that project. Project grants never leave their project.
- **Access to a project (`can_access`).** `can_read_tenant from tenant` and one of: `internal_user from tenant` (internal members reach every project of their tenant), `custom_internal from tenant`, or a project membership (`member`: any project built-in role or project role). External members therefore see nothing in a project they were not added to. This is a default for a requirement the PRD left open; it is one expression to change.
- **Project permissions.** `can_view` is `can_access`. `can_manage_members` is the tenant's `can_manage_members` or, inside the project, its internal admin or a `manage_members` grant. `can_contribute`, `can_create_folder`, `can_create_album`, `can_create_private_folder`, and `can_create_private_album` moved here from the tenant: creating a top-level container is a project Check. The tenant keeps `can_manage_projects` (same people as `can_manage_members`) for creating projects.
- **Internal members see and edit everything.** Every content `can_view` and `can_edit` includes `internal_user from tenant`, so internal users and admins reach every folder, album, document, and photo, private or public, without any share.
- **External members see public content.** A container is externally visible (`external_visible`) when it is public **and** every ancestor is public: `external_user from public and (external_user from root or external_visible from parent)`. Top-level containers carry a `root` tuple that anchors the chain; children inherit through `parent`. One private ancestor hides the whole subtree, even subfolders flagged public. Documents and photos use their container's `external_visible`.
- **External admins contribute in public content.** `can_edit` includes `external_visible and external_admin from tenant`: they create subfolders, upload, and edit any item in public content. `can_manage` (delete, share) stays with the owner, the internal admin, or a custom role that has `manage`.
- **External users are read-only by their built-in role.** `can_contribute` excludes `external_user`, so every `can_edit`, `can_manage`, and create relation is false for them even if a share names them as editor. A custom role can add contribute, edit, or manage on top.
- **Shares reach into private content.** `viewer`/`editor` accept a specific `user` and the groups `tenant#external_user` (viewer) / `tenant#external_admin` (viewer or editor), and inherit down `parent` and to items. This is how an internal admin opens a private folder to an external person or group.
- **Creating containers.** On the project, `can_create_folder`/`can_create_album` is any contributor with the product on (internal members, external admins, and custom roles that grant contribute, edit, manage, or create-private). `can_create_private_folder`/`can_create_private_album` is internal members, or a custom role with `create_private`. A subfolder also needs `can_edit` on its parent (checked in the service).
- **Changing visibility** (`can_change_visibility`) is `can_manage` intersected with `internal_user from tenant` (the internal admin, or an internal owner), or a custom role with `change_visibility`.
- **Owners** keep `can_view`/`can_manage` on what they own (owner implies editor implies viewer), even when the surrounding folder is private to them.
- **Membership is a prerequisite.** `can_view` intersects `can_access from project`, which requires `can_read_tenant`, so a stale share on a removed member grants nothing, and a stale project membership grants nothing without the tenant membership.
- **Products.** `documents_enabled`/`photos_enabled`/`blueprints_enabled` accept only `user:*`; when absent every content check and create relation is false. Only the platform toggles them (the service checks `can_manage_platform`); `can_view_products` lets the internal admin read their status. Member administration and product viewing are not intersected with a product.
- **Blueprints.** A `blueprint` sits directly in a project (`tenant`, `project`, `owner`; no container, no `public`/`root`), gated by `blueprints_enabled from tenant`. Internal members and the same tenant/project `grant_*` reach it like a document; there is no `external_visible`, so an external member sees a blueprint only through a share (`viewer`/`editor` on the blueprint itself) or a custom-role `grant_*`. `can_create_blueprint` on the project is `can_contribute and blueprints_enabled from tenant`; this deliberately lets an external admin who is a project member create (and own) blueprints, an exception to "external admins contribute only in externally visible content" because blueprints have no externally visible regime. Versions are Postgres rows and never tuples.
- **No `but not`.** "Private wins" is expressed with intersection over the ancestor chain, so the model stays allow-only.
- A system admin is not an `internal_admin` of each tenant. Nothing pulls `platform#system_admin` into a tenant or resource relation.

## Model bootstrap

`api/internal/infrastructure/fga/bootstrap.go` has two entry points.

`Reset` (used by `cmd/seed`):

1. List stores. Delete every store named `OPENFGA_STORE_NAME` (`fga-demo`).
2. Create a fresh store with that name.
3. Read `OPENFGA_MODEL_PATH`, transform the DSL, and call WriteAuthorizationModel.
4. Print `store_id` and `model_id` to stdout.

`Open` (used by `cmd/api`):

1. List stores and find the one named `OPENFGA_STORE_NAME`. A missing store is a startup error that tells the operator to run `cmd/seed`.
2. Call ReadAuthorizationModels with page size 1. The first result is the newest model. Use its id.

Every Check, BatchCheck, ListObjects, Read, and Write sends that `authorization_model_id`. Check, BatchCheck, and ListObjects use consistency `HIGHER_CONSISTENCY`.

Resetting the store is what makes the seed repeatable: the demo always starts from the README's step 1. The store id changes after each seed run, so restart `cmd/api` after seeding and reselect the store in the Playground.

