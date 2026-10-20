# Seed data

The fixed demo world written by `cmd/seed`. Source of truth in code: `api/internal/seed/world.go`.


`cmd/seed` resets the demo. It runs migrations, truncates every app table, recreates the OpenFGA store (see Model bootstrap), inserts the rows in this section, and writes the tuples in this section. Running it again always returns the world to this state and discards anything created in the UI. That is the intended behavior for a click-through demo.

Password for every user is `password`, hashed at seed time.

#### People

| Name | Email | Id | Platform |
| --- | --- | --- | --- |
| Ada | ada@example.com | `11111111-1111-1111-1111-111111111111` | system admin |
| Bob | bob@example.com | `22222222-2222-2222-2222-222222222222` | |
| Cara | cara@example.com | `33333333-3333-3333-3333-333333333333` | |
| Dan | dan@example.com | `44444444-4444-4444-4444-444444444444` | |
| Eve | eve@example.com | `55555555-5555-5555-5555-555555555555` | |
| Fay | fay@example.com | `66666666-6666-6666-6666-666666666666` | |

#### Tenants and memberships

| Tenant | Slug | Id | Members |
| --- | --- | --- | --- |
| Acme | acme | `aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | Bob internal_admin, Cara internal_user, Dan external_user, Fay external_admin |
| Globex | globex | `bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | Eve internal_admin, Bob external_user |

Role tuples, and no implied duplicate:

- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#internal_admin@user:22222222-2222-2222-2222-222222222222`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#internal_user@user:33333333-3333-3333-3333-333333333333`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#external_user@user:44444444-4444-4444-4444-444444444444`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#external_admin@user:66666666-6666-6666-6666-666666666666`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#internal_admin@user:55555555-5555-5555-5555-555555555555`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#external_user@user:22222222-2222-2222-2222-222222222222`

Both products enabled on both tenants, which means all four wildcard tuples exist.

#### Custom roles

No assignees, so nobody's checks change until a platform admin assigns someone.

| Tenant | Role | Type | Id | Permissions |
| --- | --- | --- | --- |
| Acme | Auditor | internal | `c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | `view`, `view_products` |
| Globex | Vendor | external | `c0bbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | `contribute` |

Tuples, and no `assignee` tuples:

- `role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#tenant@tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#internal_role@role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#grant_view@role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#assignee`
- `tenant:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#grant_view_products@role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#assignee`
- `role:c0bbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#tenant@tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#external_role@role:c0bbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb`
- `tenant:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#grant_contribute@role:c0bbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb#assignee`

#### Acme content

| Kind | Name | Id | Parent or container | Visibility | Owner | Shares |
| --- | --- | --- | --- | --- | --- | --- |
| folder | Company | `f1111111-1111-1111-1111-111111111111` | top level | private | Bob | |
| folder | Board | `f2222222-2222-2222-2222-222222222222` | top level | private | Bob | |
| folder | Legal | `f3333333-3333-3333-3333-333333333333` | Board | public flag, hidden by Board | Bob | |
| folder | Client | `f7777777-7777-7777-7777-777777777777` | top level | public | Bob | |
| folder | Specs | `f9999999-9999-9999-9999-999999999999` | Client | private | Bob | |
| folder | Partners | `faaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | top level | private | Bob | viewer Dan |
| document | Handbook | `d1111111-1111-1111-1111-111111111111` | Company | | Bob | |
| document | Secret | `d2222222-2222-2222-2222-222222222222` | Board | | Bob | |
| document | Q3 Plan | `d3333333-3333-3333-3333-333333333333` | Board | | Bob | |
| document | Legal Hold | `d4444444-4444-4444-4444-444444444444` | Legal | | Bob | |
| document | Client Brief | `d7777777-7777-7777-7777-777777777777` | Client | | Bob | |
| document | Visit Agenda | `d9999999-9999-9999-9999-999999999999` | Client | | Fay | |
| document | Pricing | `daaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | Specs | | Bob | |
| document | Partner Deck | `dbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | Partners | | Bob | |
| album | All-hands | `c1111111-1111-1111-1111-111111111111` | top level | private | Bob | |
| album | Offsite | `c2222222-2222-2222-2222-222222222222` | top level | private | Bob | |
| album | Client visit | `c7777777-7777-7777-7777-777777777777` | top level | public | Bob | |
| photo | Kickoff | `e1111111-1111-1111-1111-111111111111` | All-hands | | Bob | |
| photo | Offsite Shot | `e2222222-2222-2222-2222-222222222222` | Offsite | | Bob | |
| photo | Lobby | `e7777777-7777-7777-7777-777777777777` | Client visit | | Bob | |

Mime type `text/plain` and byte size `128` on every seeded document. Mime type `image/jpeg` and byte size `2048` on every seeded photo. `created_by` is the owner.

#### Globex content

| Kind | Name | Id | Parent or container | Visibility | Owner |
| --- | --- | --- | --- | --- | --- |
| folder | Company | `f4444444-4444-4444-4444-444444444444` | top level | private | Eve |
| folder | Board | `f5555555-5555-5555-5555-555555555555` | top level | private | Eve |
| folder | Client | `f8888888-8888-8888-8888-888888888888` | top level | public | Eve |
| document | Handbook | `d5555555-5555-5555-5555-555555555555` | Company | | Eve |
| document | Secret | `d6666666-6666-6666-6666-666666666666` | Board | | Eve |
| document | Client Brief | `d8888888-8888-8888-8888-888888888888` | Client | | Eve |
| album | All-hands | `c3333333-3333-3333-3333-333333333333` | top level | private | Eve |
| album | Offsite | `c4444444-4444-4444-4444-444444444444` | top level | private | Eve |
| album | Client visit | `c8888888-8888-8888-8888-888888888888` | top level | public | Eve |
| photo | Kickoff | `e3333333-3333-3333-3333-333333333333` | All-hands | | Eve |
| photo | Offsite Shot | `e4444444-4444-4444-4444-444444444444` | Offsite | | Eve |
| photo | Lobby | `e8888888-8888-8888-8888-888888888888` | Client visit | | Eve |

Every container gets the tuples from "Create folder": `tenant`, `owner`, `parent` or `root`, and `public` when public. Every item gets `tenant`, its container, and `owner`. The one share is `folder:faaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#viewer@user:44444444-4444-4444-4444-444444444444` (Partners to Dan). All four product wildcards exist. The ids, rows, and tuples live in `api/internal/seed/world.go` (one slice per domain: `Folders`, `Albums`, `Documents`, `Photos`); this table describes them.

