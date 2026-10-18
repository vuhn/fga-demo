# Authorization model

The OpenFGA model, identifier format, and store bootstrap. `api/fga/model.fga` must equal the DSL block below exactly. Overview: [api/ARCHITECTURE.md](../api/ARCHITECTURE.md).

## Identifiers

Postgres primary keys and OpenFGA object ids are the same UUID strings, except the platform object id, which is the literal `main`.

OpenFGA object format is `<type>:<id>`. User format is `user:<uuid>`. The wildcard is `user:*`. Group-share usersets are `tenant:<uuid>#external_user` and `tenant:<uuid>#external_admin`. A container's `tenant`, `root`, and `public` relations point at the plain `tenant:<uuid>` object.

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

type tenant
  relations
    define internal_admin: [user]
    define internal_user: [user] or internal_admin
    define external_admin: [user]
    define external_user: [user] or external_admin
    define documents_enabled: [user:*]
    define photos_enabled: [user:*]
    define can_manage_members: internal_admin
    define can_view_products: internal_admin
    define can_read_tenant: internal_user or external_user
    define can_contribute: internal_user or external_admin
    define can_create_folder: can_contribute and documents_enabled
    define can_create_album: can_contribute and photos_enabled
    define can_create_private_folder: internal_user and documents_enabled
    define can_create_private_album: internal_user and photos_enabled

type folder
  relations
    define tenant: [tenant]
    define parent: [folder]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible) and can_read_tenant from tenant and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and external_admin from tenant)) and can_contribute from tenant and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: can_manage and internal_user from tenant
    define can_create_document: can_edit

type document
  relations
    define tenant: [tenant]
    define folder: [folder]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from folder
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from folder
    define can_view: (internal_user from tenant or viewer or external_visible from folder) and can_read_tenant from tenant and documents_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from folder and external_admin from tenant)) and can_contribute from tenant and documents_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and documents_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage

type album
  relations
    define tenant: [tenant]
    define parent: [album]
    define root: [tenant]
    define public: [tenant]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from parent
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from parent
    define external_visible: external_user from public and (external_user from root or external_visible from parent)
    define can_view: (internal_user from tenant or viewer or external_visible) and can_read_tenant from tenant and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible and external_admin from tenant)) and can_contribute from tenant and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
    define can_change_visibility: can_manage and internal_user from tenant
    define can_create_photo: can_edit

type photo
  relations
    define tenant: [tenant]
    define album: [album]
    define owner: [user]
    define editor: [user, tenant#external_admin] or owner or editor from album
    define viewer: [user, tenant#external_user, tenant#external_admin] or editor or viewer from album
    define can_view: (internal_user from tenant or viewer or external_visible from album) and can_read_tenant from tenant and photos_enabled from tenant
    define can_edit: (internal_user from tenant or editor or (external_visible from album and external_admin from tenant)) and can_contribute from tenant and photos_enabled from tenant
    define can_manage: (owner or internal_admin from tenant) and can_contribute from tenant and photos_enabled from tenant
    define can_delete: can_manage
    define can_share: can_manage
```

How to read it:

- **Roles.** `internal_admin` implies `internal_user`; `external_admin` implies `external_user`. An admin satisfies the lower relation without a second tuple.
- **Internal members see and edit everything.** Every content `can_view` and `can_edit` includes `internal_user from tenant`, so internal users and admins reach every folder, album, document, and photo, private or public, without any share.
- **External members see public content.** A container is externally visible (`external_visible`) when it is public **and** every ancestor is public: `external_user from public and (external_user from root or external_visible from parent)`. Top-level containers carry a `root` tuple that anchors the chain; children inherit through `parent`. One private ancestor hides the whole subtree, even subfolders flagged public. Documents and photos use their container's `external_visible`.
- **External admins contribute in public content.** `can_edit` includes `external_visible and external_admin from tenant`: they create subfolders, upload, and edit any item in public content. `can_manage` (delete, share) stays with the owner or the internal admin, so an external admin can delete only what they own.
- **External users are read-only.** `can_contribute` excludes them, so every `can_edit`, `can_manage`, and create relation is false for them even if a tuple names them as editor.
- **Shares reach into private content.** `viewer`/`editor` accept a specific `user` and the groups `tenant#external_user` (viewer) / `tenant#external_admin` (viewer or editor), and inherit down `parent` and to items. This is how an internal admin opens a private folder to an external person or group.
- **Creating containers.** `can_create_folder`/`can_create_album` is any contributor with the product on (internal members and external admins). `can_create_private_folder`/`can_create_private_album` is internal members only, so external admins can only create public containers. A subfolder also needs `can_edit` on its parent (checked in the service).
- **Changing visibility** (`can_change_visibility`) is `can_manage` intersected with `internal_user from tenant`: the internal admin, or an internal owner.
- **Owners** keep `can_view`/`can_manage` on what they own (owner implies editor implies viewer), even when the surrounding folder is private to them.
- **Membership is a prerequisite.** `can_view` intersects `can_read_tenant from tenant`, so a stale share on a removed member grants nothing.
- **Products.** `documents_enabled`/`photos_enabled` accept only `user:*`; when absent every content check and create relation is false. Only the platform toggles them (the service checks `can_manage_platform`); `can_view_products` lets the internal admin read their status. Member administration and product viewing are not intersected with a product.
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

