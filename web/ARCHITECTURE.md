# Web architecture

The React admin panel (Vite, React 19, TypeScript, Mantine 9, react-router-dom 6). It talks only to the HTTP API; see the [system overview](../ARCHITECTURE.md).

## Structure

```text
src/
  main.tsx, App.tsx     providers, routes, app shell, menu config
  api.ts                fetch client: JWT, X-Tenant-ID, typed endpoints
  auth.tsx              login state, /me
  permissions.tsx       the only mapping from permissions to UI
  ui.tsx                shared helpers: run, notifyError, confirmDanger, PageHeader, badges
  pages/                one lazy-loaded route component per screen
  components/           TenantSwitcher, ShareDialog, ContentPage
  styles.css            small tweaks on Mantine CSS variables
```

Pages: Login, Home, Users, Tenants, TenantDetail, Documents, Photos, Inspector. Documents and Photos are thin wrappers over one `ContentPage` that renders folders/albums as a tree. Page-by-page behavior: [web UI reference](../docs/web-ui.md).

## Data flow

- `api.ts` attaches the JWT and the active tenant (`sessionStorage.tenant_id`) as `X-Tenant-ID`. The Vite dev server proxies `/api` to `:8088`.
- `TenantSwitcher` lists the actor's tenants (`GET /tenants/mine`) and sets the active tenant; changing it reloads page data and permissions.
- Mutations go through `run(fn, successMsg)` in `ui.tsx`, which toasts success or the mapped API error. There is no per-page error state.

## Permissions

The API computes permissions with OpenFGA; the UI renders them. It never decides.

- `PermissionsProvider` loads `GET /me/permissions` for the active tenant. Everything is denied while loading and revalidated when the tab regains focus.
- Content rows carry a `permissions` object from the API (one server-side BatchCheck per page). Buttons read it through `ACTIONS` and `canAct`.
- `FEATURES` maps feature names (menu entries, route guards, controls) to rules over `platform:` or `tenant:` permissions, as `{ all?, any? }`. Product menus need tenant membership and the product enabled.
- Components use `useCan`, `useAllows`, `<Can>`, and `RequireFeature`. They never name an OpenFGA relation and never read `role` or `affiliation` to show a control; those are display labels (`RoleBadge`, `AffiliationBadge`).
- `/tenants/:id` is guarded against the tenant in the URL (`usePermissionSet(id)`), not the active one, because system admins open tenants they are not members of.
- The top menu is a config array of `{label, to, feature}` filtered by feature.

Hiding a control is UX only; the API checks again on every request.

## UI conventions

- Mantine components and props first; plain CSS only for small tweaks with `--mantine-*` variables. No second UI library, no CSS-in-JS runtime.
- Forms in `Modal`s, destructive actions through `confirmDanger`, results as notifications.
- Pages are `React.lazy` routes; `vite.config.ts` splits `react`, `mantine`, and `icons` vendor chunks. Icons are imported by name so they tree-shake.
- Empty lists show one line of copy; the UI never falls back to a role name to show rows the API omitted.

## Build

`npm run build` runs `tsc --noEmit` then `vite build`. Versions are pinned exactly.
