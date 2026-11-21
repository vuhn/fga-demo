---
name: system-design
description: Stage 2 of the feature SDLC. Produce specs/<feature>/system-design.md with the components to change and Mermaid sequence and data-flow diagrams. Use after requirements are approved.
---

# System design

Input: approved `requirements.md`. Output: `specs/<feature>/system-design.md` from `specs/_templates/system-design.md`.

1. Read `ARCHITECTURE.md`, `api/ARCHITECTURE.md` (Go), `web/ARCHITECTURE.md` (panel), and the relevant `docs/*`. Name every component to create/update/delete: api layers, web, blueprint-engine, OpenFGA model/tuples, infra/3rd party.
2. Sequence diagram: communication between services and infrastructure (Web -> API handler -> service -> Postgres/OpenFGA/engine/3rd party), including the authorization Check, streaming, and the alt/failure paths that change behavior. Do NOT explain detailed logic inside a service. Style reference: `prds/example-system-design-squence-diagram.md`.
3. Data-flow diagram where data moves or transforms across stores.
4. Authorization: new/changed actions and Checks (`docs/decision-catalog.md`), model/tuple changes, which CLAUDE.md invariants are touched; respect write ordering (grant PG then FGA, revoke FGA then PG). Model stays allow-only.
5. API contract: routes, bodies, statuses per `docs/http-api.md` conventions.
6. List the docs that must change in the same PR set (spec first).
7. Skip the diagram only when the feature adds no interaction (state why in the status table).
8. Run `system-design-reviewer`, then gate: user approves.
