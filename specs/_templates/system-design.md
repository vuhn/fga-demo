# System design: <feature>

## Components changed
| Component | Change (create / update / delete) | Why |
| --- | --- | --- |
| api | | |
| web | | |
| blueprint-engine | | |
| OpenFGA model / tuples | | |
| infra / 3rd party | | |

## Sequence diagram
Communication between services and infrastructure only, not business logic. Participants are real names (handler, usecase service, Postgres, OpenFGA, engine). Include the authorization Check and the failure/alt paths that change behavior.

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Web
    participant API
    participant PG as Postgres
    participant FGA as OpenFGA
    User->>Web: action
    Web->>API: request (JWT, X-Tenant-ID)
    API->>FGA: Check <relation> on <object>
    FGA-->>API: allowed
    API->>PG: write
    API-->>Web: 201
```

## Data flow
```mermaid
flowchart LR
    A[source] --> B[transform] --> C[store]
```

## Authorization
New or changed actions (add to `docs/decision-catalog.md`), model/tuple changes (`docs/authorization-model.md`, `docs/tuple-catalog.md`), invariants touched.

## API contract
Routes, request/response, status codes (`docs/http-api.md`).

## Docs to update in the same change
-

## Risks and alternatives rejected
-
