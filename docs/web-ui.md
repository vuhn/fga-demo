# Web UI behavior

Routes, permission mapping, and visual design of the React admin panel. Overview: [web/ARCHITECTURE.md](../web/ARCHITECTURE.md).

## React admin panel

Vite dev server proxies `/api` to `http://localhost:8088`. The client sends the JWT from `sessionStorage` and the selected tenant id from `sessionStorage` key `tenant_id` as `X-Tenant-ID`.

Routes:

| Path | Page | Behavior |
| --- | --- | --- |
| `/login` | Login | `POST /auth/login`. Store the token. |
| `/` | Home | `GET /me`. Show the person, whether they are a system admin, and each membership's affiliation and role. Explain internal versus external in a short static paragraph. |
| `/users` | Users | Feature `nav.users` (`can_manage_platform`) for the menu entry and the route guard. List and create users. |
| `/tenants` | Tenants | Feature `nav.tenants`, same rule. List tenants, create a tenant, link to detail. |
| `/tenants/:id` | Tenant detail | Tenant administration: products and members. Guarded by feature `tenant.admin` (`can_manage_members` or `can_manage_platform`) for the tenant in the URL; other members get "Not available". The sidebar entry "Members" uses `nav.members` (`tenant:can_manage_members` on the active tenant), so only internal admins see it; a system admin opens the page from the Tenants directory. Product switches render only for `tenant.products.manage` (`can_manage_platform`); internal admins see status badges and the note "Products are enabled or disabled by the platform admin." Member controls render for `tenant.members.manage` (`can_manage_platform` or `can_manage_members`). A filter control calls `GET /members?affiliation=internal` or `external`. The invite form picks affiliation first and then offers only `internal_admin` and `internal_user`, or only `external_admin` and `external_user`. The role editor for an existing member offers only the two roles of that member's affiliation. |
| `/documents` | Documents | Require a selected tenant. Tree of folders the user can view, each with a Private, Public, or "Public · hidden by parent" badge (computed from loaded ancestors). The row menu offers Make public/private when `can_change_visibility` is true; making something public asks for confirmation. The New folder dialog shows a Private switch (default on) only with feature `folder.createPrivate`; otherwise it states the folder will be public. The parent list contains only folders where the actor has `can_edit`. Documents whose folder is hidden and that ListObjects returned go under "Shared with me". Create folder, add document metadata, rename, delete, and share call the matching routes. Hide each button unless the row's `permissions` allow the action (see `ACTIONS`). Route guard `nav.documents`; when the actor is a member but the product is off, the guard explains that the product is turned off. |
| `/photos` | Photos | Same interaction as documents, for albums and photos. |
| `/inspector` | Inspector | Form for `POST /authz/check`. Object lookup via `GET /access`, showing direct tuples and the member Check grid. |

`TenantSwitcher` calls `GET /tenants/mine` (ListObjects `can_read_tenant`) and shows each tenant with the actor's affiliation and role labels. Changing tenant writes `tenant_id`, which reloads page data and the active-tenant permissions. On `/tenants/:id` it also navigates to the new tenant's page. A stored tenant the user can no longer read is replaced after `/me` refreshes. Ada's switcher is empty. She still opens `/tenants` from the nav.

`ShareDialog` loads the object's `GET …/share-candidates` and `GET …/grants`, lists existing grants with a revoke button for each non-owner grant, and offers viewer or editor for an external person, viewer for all external users, and viewer or editor for all external admins. Only external members are listed: internal members already see everything. Editor is disabled in the dialog when the selected person is an external user. The API still returns 400 `cannot_upload` if that grant is posted. It posts the grant route.

Permissions in the UI live in `web/src/permissions.tsx`, the only file that maps permissions to UI:

- `FEATURES` maps a feature name (`nav.users`, `tenant.members.manage`, `folder.create`, …) to a rule over permissions written `platform:<relation>` or `tenant:<relation>`. A rule has `all` (every permission must be true) and/or `any` (one is enough, mirroring the OR rows of the decision catalog); when both are present both must pass, and an empty rule is denied.
- Product menus and pages need membership **and** the product: `nav.documents` is `all: [tenant:can_read_tenant, tenant:documents_enabled]`, `nav.photos` is `all: [tenant:can_read_tenant, tenant:photos_enabled]`. Disabling a product removes its menu entry for every member, and a typed URL shows "<Product> is turned off for this tenant".
- `ACTIONS` maps an object action (`rename`, `delete`, `share`, `addItem`) and object type to the relation in that row's `permissions` object. Rename is `can_manage` on folders and albums and `can_edit` on documents and photos.
- `PermissionsProvider` loads `GET /me/permissions` for the active tenant and reloads when the tenant changes or `/me` is refreshed after a mutation. `useCan(feature)`, `useAllows()`, and `<Can feature>` read it. Everything is denied while loading.
- `RequireFeature` guards routes, so a typed URL shows "Not available" instead of a failed request. `/tenants/:id` is guarded against the tenant in the URL through `usePermissionSet(id)`, because a system admin opens tenants that are not active.
- The top bar menu is a config array of `{label, to, feature}` filtered with `useAllows()`. Home and Inspector have no feature: everyone may inspect their own access. The Members entry uses `tenant:can_manage_members` only: internal admins of the active tenant. The Inspector loads the roster for its user picker only with `tenant.members.manage`; everyone else checks themselves.
- Permissions are revalidated in the background when the tab regains focus, so a product another admin switched off disappears without a reload. A revalidation keeps the current permissions on screen; only a tenant switch denies everything until the new answer arrives.
- Content pages render buttons from each row's `permissions` through `canAct`. They do not call batch Check. `POST /authz/batch-check` stays available as an API; the inspector page uses `POST /authz/check`.
- Components never name an OpenFGA relation and never read `role` or `affiliation` to show a control. The one display-only exception is the share dialog greying out editor for a person labelled `external_user`, which the API enforces with `cannot_upload` anyway.

Empty lists stay empty, with one line of copy: "Nothing here is visible with the current relationships." Do not fall back to a role name to show rows the API omitted.

Visual design uses Mantine and looks like a real admin app:

- `AppShell` with a header (logo, tenant switcher `Select`, user menu with a dark-mode toggle and sign out) and a sidebar split into Workspace and Platform sections. The sidebar collapses behind a burger below the `sm` breakpoint.
- Pages start with `PageHeader` (title, description, primary action). Lists are `Table`s inside `Card`s; folders and albums are a collapsible tree with a `Menu` of row actions built from `ACTIONS`.
- Forms open in `Modal`s. Destructive actions confirm through `@mantine/modals`. Results and API errors appear as `@mantine/notifications` toasts via `run`/`notifyError` in `web/src/ui.tsx`, which also maps error codes to readable titles.
- Role and affiliation labels show as badges for display only.
- Theme: indigo primary, `defaultColorScheme="auto"`. `web/src/styles.css` holds only app tweaks that use Mantine CSS variables.
- Pages are lazy-loaded with `React.lazy`. `vite.config.ts` splits `react`, `mantine`, and `icons` into long-lived vendor chunks.

