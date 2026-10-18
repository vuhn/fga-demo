# OpenFGA multi-tenant authorization demo

This repository is a teaching demo of [OpenFGA](https://openfga.dev) in a multi-tenant application. A Go API owns users, tenants, documents, and photos. OpenFGA owns every allow or deny decision. A React admin panel built with Mantine is the clickable proof.

Postgres stores names, membership labels, and file metadata. It is the display record. OpenFGA stores relationship tuples and answers Check and ListObjects. The API allows a content action only after OpenFGA returns true.

The implementation contract is [ARCHITECTURE.md](ARCHITECTURE.md). Generate the database, the API, the model, the seed, and the UI from that document.

## What this demo teaches

- Relationship-based access: a permission is a path through objects, not a role string on a request.
- Tenant-wide defaults by affiliation: internal members see and edit all content; external members see public content. Nobody has to share the client folder with each guest.
- Private wins: a folder or album is visible to externals only when it and every ancestor are public. One private parent hides the whole subtree, without OpenFGA `but not`.
- Shares into private content: an internal admin can open a private folder to one external person or to all external users or admins.
- Entitlements: the documents product and the photos product are wildcard tuples. Content checks intersect that wildcard, so disabling a product fails every content check while the content tuples stay.
- Check versus ListObjects: a single object uses Check. A page of documents uses ListObjects, then a Postgres query for those ids only.
- Tenant isolation: the same person can be an internal admin of Acme and an external user of Globex. The JWT carries the user id. The active tenant is a header.

## Authorization story

Each tenant has two member lists. Internal members work for the tenant. External members do not; an internal tenant admin or a system admin invites them. Affiliation is chosen when the person is added and does not change afterward. Internal roles go only to internal members, external roles only to external members.

Every folder and album is **private** or **public** (`is_private`). Private content is for internal members only, unless it is shared. Public content is also visible to every external member of the tenant. A container is public to externals only if it and all its ancestors are public, so a private folder hides everything inside it.

| Role | Who | Private content | Public content | Tenant |
| --- | --- | --- | --- | --- |
| System admin | Platform operator | none | none | Enable or disable products. Create tenants and users. Assign members of either affiliation. |
| Internal tenant admin | Staff | View, add, edit, delete, share, change visibility | Same | The only tenant role with the Members page: the member roster and roles, plus read-only product status. |
| Internal tenant user | Staff | View, add, edit; delete and share only what they own | Same | Create folders and albums, private or public. |
| External tenant admin | Invited guest | Only what is shared with them | View, create folders, add and edit any item; delete and share only what they own | Create public folders and albums only. |
| External tenant user | Invited guest | Only what is shared with them (view) | View | Read-only. Can never be granted editor. |

`internal_admin` implies `internal_user`, and `external_admin` implies `external_user`, so each admin has every permission of the matching user role.

**Platform.** Ada is `system_admin` of `platform:main`. She manages tenants, users, products, and memberships, but she is not a tenant member and sees no files.

**Visibility.** Only internal members create private containers; an external admin's new folder is always public. Making a container public or private takes `can_change_visibility`: the internal admin, or an internal owner. Creating a subfolder needs edit rights on the parent, so an external admin can nest only inside public content.

**Sharing.** Sharing exists for external members, because internal members already see everything. An internal admin (or the owner) can share any folder, album, document, or photo, including private ones, with one external person (viewer, or editor for an external admin), with all external users (viewer), or with all external admins (viewer or editor). Shares inherit down the tree.

**Deletes do not inherit.** Contributors can add and change content, but deleting or sharing an item takes ownership or the internal admin role.

OpenFGA is allow-only. There is no deny tuple. Removing access means deleting the relationship that granted it.

## Seeded world

Password for every account: `password`

| Person | Email | Relationships |
| --- | --- | --- |
| Ada | ada@example.com | System admin of `platform:main`. No tenant membership. |
| Bob | bob@example.com | Internal admin of Acme. External user of Globex. |
| Cara | cara@example.com | Internal user of Acme. |
| Dan | dan@example.com | External user of Acme. Viewer of the private Partners folder. |
| Fay | fay@example.com | External admin of Acme. Owns the Visit Agenda document. |
| Eve | eve@example.com | Internal admin of Globex. No Acme membership. |

Both tenants start with documents and photos enabled.

Acme folders (owner Bob unless noted):

- **Company** (private): Handbook.
- **Board** (private): Q3 Plan, Secret.
  - **Legal** (flagged public, but hidden because Board is private): Legal Hold.
- **Client** (public): Client Brief, Visit Agenda (owned by Fay).
  - **Specs** (private): Pricing.
- **Partners** (private, shared with Dan as viewer): Partner Deck.

Acme albums: **All-hands** (private, Kickoff), **Offsite** (private, Offsite Shot), **Client visit** (public, Lobby).

Globex mirrors the shape with Eve as owner: Company, Board, and the All-hands and Offsite albums are private; Client and Client visit are public. Bob, a Globex external user, sees only Client, Client Brief, Client visit, and Lobby there.

Exact ids, tuples, and API fields are in [ARCHITECTURE.md](ARCHITECTURE.md).

## Five-minute script

1. Sign in as Ada. The tenant directory shows Acme and Globex with both products on. Her tenant switcher is empty and she sees no files. She can add a user to a tenant as internal or external, and the role list follows that choice.
2. Sign in as Bob and select Acme. He sees every folder with a Private or Public badge; Legal shows "Public · hidden by parent". Only internal admins have the Members menu: there he sees product status (read-only, with no switches) and manages members; the filter Internal shows Bob and Cara, External shows Dan and Fay.
3. Sign in as Cara. She sees every Acme folder, private or public, without any share, and can add documents anywhere. She cannot delete Secret (Bob owns it) and cannot change visibility. She has no Members menu, and opening the tenant page by URL says "Not available".
4. Sign in as Dan. He sees Client (Client Brief, Visit Agenda) and Partners (Partner Deck, shared with him), plus the Client visit album. He does not see Company, Board, Legal, or Specs. He has no upload or create buttons.
5. Sign in as Fay. She sees Client only. She can add a document there and edit Client Brief, but only delete her own Visit Agenda. Her New folder dialog has no Private switch: her folders are always public, and Dan sees them immediately.
6. As Bob, make Specs public. Dan now sees Specs and Pricing. Make it private again and they disappear.
7. As Bob, make Board public. Board and Legal (already flagged public) appear for Dan and Fay. Make it private again.
8. As Bob, share Board with all external admins as viewer. Fay sees Board, Q3 Plan, and Legal but cannot edit; Dan still cannot see Board. Revoke it.
9. As Ada, open Acme from the tenant directory and disable photos (only the platform admin can). Sign in as Bob: the Photos menu is gone for every Acme member, `/photos` says the product is turned off, and his Members page shows Photos as disabled. Kickoff's owner tuple is still visible in the inspector. Ada enables photos again and everything comes back.
10. As Bob, remove Dan from Acme. Dan loses Acme in the switcher and every Acme file, including the Partners share.
11. Sign in as Bob and switch to Globex. He sees only the public Client folder and Client visit album, with no create buttons: he is an external user there.

## Run

Requirements: Docker, Go 1.26 or newer, Node.js 22 or newer.

```bash
docker compose -f deploy/docker-compose.yml up -d
cp .env.example .env
make seed   # cd api && go run ./cmd/seed
make api    # cd api && go run ./cmd/api
```

In a second shell:

```bash
cd web && npm install && npm run dev
```

- Admin panel: http://localhost:5173
- API: http://localhost:8088
- OpenFGA HTTP: http://localhost:8080
- OpenFGA Playground: http://localhost:3000

`cmd/seed` migrates Postgres (including `00002_container_privacy.sql`), truncates the app tables, recreates the `fga-demo` OpenFGA store, writes the model, and writes the seeded rows and tuples. Run it again whenever you want to restart the script from step 1. Anything created in the UI is discarded. `cmd/api` migrates and opens the existing store with its newest model. It never seeds. Restart it after a reseed, because the store id changes.

The Playground is for inspecting tuples while you click through the script. The store name is `fga-demo`. The seed command prints the store id and model id.

## Repository map

The paths below are the ones the implementation creates. [ARCHITECTURE.md](ARCHITECTURE.md) defines each one.

```text
api/                              Go module for the API service
api/cmd/api/                      HTTP process; wire.go injects every dependency
api/cmd/seed/                     migrate, model bootstrap, demo data
api/internal/domain/entity/       entities, value types, sentinel errors
api/internal/domain/gateway/      data-access interfaces (Postgres, OpenFGA)
api/internal/usecase/             one interface per operation (FolderCreator, ...)
api/internal/usecase/helper/      shared rule interfaces (write ordering, share policy)
api/internal/service/             usecase implementations (folder_creator.go, ...)
api/internal/infrastructure/      postgres, fga, auth, idgen, config
api/internal/handler/http/        Gin handlers, one per domain, and middleware
api/migrations/                   goose SQL
api/fga/model.fga                 authorization model DSL
deploy/docker-compose.yml
deploy/postgres/init.sql
web/                     Vite + React admin panel
```

## Further reading

- [OpenFGA concepts](https://openfga.dev/docs/concepts)
- [Relationship queries: Check and ListObjects](https://openfga.dev/docs/interacting/relationship-queries)
- [Modeling guides](https://openfga.dev/docs/modeling)
