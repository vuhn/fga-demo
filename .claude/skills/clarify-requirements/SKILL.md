---
name: clarify-requirements
description: Stage 1 of the feature SDLC. Turn a basic requirement into clear, agreed requirements by asking targeted questions; writes specs/<feature>/requirements.md. Use at the start of a feature or when a requirement is ambiguous.
---

# Clarify requirements

Goal: no misunderstanding, and a balance between the business ask and the current system so effort stays small.

1. Read the raw ask (prompt, `prds/*`), `ARCHITECTURE.md`, and the docs that the feature touches (`docs/decision-catalog.md`, `docs/http-api.md`, `docs/database-schema.md`). Explore the code before asking anything the code answers.
2. Find gaps: actors and tenants (internal vs external, roles, platform admin), authorization (new action? which Check?), data (existing rows, migration), edge cases and failure behavior, UI vs API only, scale, 3rd parties/infra, conflicts with CLAUDE.md invariants and non-goals.
3. Ask the user in one batch (AskUserQuestion, 1-4 questions, recommended option first). Where a PO/biz answer is needed, list those separately as "to confirm with PO" and propose a default that minimizes effort against the current system (e.g. reuse an existing relation instead of a new model type).
4. Write `specs/<feature>/requirements.md` from `specs/_templates/requirements.md`: problem, outcome, scope/out, verifiable acceptance criteria, impact on current system, decisions, open questions. Open questions must be empty or explicitly assigned before approval.
5. Gate: user approves. Later changes to requirements after planning started are rework; note them in the file.
