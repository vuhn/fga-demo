---
name: plan-tasks
description: Stage 4 of the feature SDLC. Break an approved design into small per-service PR tasks with estimates, dependencies, infra changes and release order; writes specs/<feature>/plan.md. Use after system and DB design are approved.
---

# Plan

Input: approved `requirements.md`, `system-design.md`, `db-design.md`. Output: `specs/<feature>/plan.md` from the template.

Use plan mode thinking: read the code that will change so tasks name real files and layers.

1. Infrastructure / 3rd party: list every create/update/delete (compose services, env vars in `docs/operations.md`, OpenFGA model, search/LLM providers, secrets).
2. Break down by service: each task is one PR for ONE service (api, web, blueprint-engine, infra). Size so a reviewer finishes in 15-30 minutes: roughly one usecase+service+repo+handler slice, or one migration, or one page. If bigger, split by vertical slice or by layer with a working state at each merge.
3. Per task: id, service, scope (files/layers), estimate in hours, dependencies, tests (unit; integration if the project requires it for authz changes).
4. Dependencies and order: tasks that must merge first (migration before code that reads it; API before web). Mark what can run in parallel.
5. Release order: what ships first and why (expand migration -> API -> web; contract later). This feeds `release-docs`.
6. Test plan section: manual/e2e steps mapped to the acceptance criteria.
7. Docs to update per task (spec first). Run `design-reviewer`, then gate: user approves. Approved plan is what `build-task` follows; deviations are recorded in the file.
