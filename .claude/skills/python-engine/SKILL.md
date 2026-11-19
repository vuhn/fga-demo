---
name: python-engine
description: Implement or change Python code in blueprint-engine/ (the deterministic Engine: geometry, DXF/IFC, checks, plan ops, estimates; gRPC and legacy FastAPI) following the repo's Clean Architecture mixed with DDD. Use for any task touching blueprint-engine/src/fga_engine, blueprint-engine/tests, or the Engine's gRPC API.
---

# Implement Engine changes

Source of truth: CLAUDE.md "Architecture rules" (Python services bullet), `blueprint-engine/README.md`, `docs/service-contracts.md`, `docs/harness/evaluator-rubric.md` (Python layering checklist), and `specs/re-architecture-system/plan.md` for what is being moved. This skill is the working checklist.

## The Engine in one paragraph
Stateless and deterministic: **the LLM only produces or edits the brief; code does every piece of geometry.** No auth, no storage, no tenant or user ids. The Go API calls it (legacy FastAPI `app.py`), and the Agent calls its tools over gRPC (`NormalizeDraft`, `ApplyBriefOps`, `ListTools`, `GetCatalog`, plan ops, checks). New work belongs in the gRPC path; the Engine must not gain LLM or network code (`engine-no-llm`, `engine-no-network` checks).

## State of the refactor (read before choosing where code goes)
The delivery side already follows the target layout: `handler/grpc/<domain>_handler.py` (own `<Domain>Usecases`), `mapper/`, `common/*_helper.py`, one gateway per file in `domain/gateway/`, `usecase/<op>.py` paired with `service/<op>.py`, `wire.py` as the only composition root. Its `domain/` is still flat modules and some root modules (`llm_steps.py`, `prompts.py`, `pipeline.py`, `app.py`, ...) are legacy, listed in `scripts/python-arch-allow.txt`. Rules:
- New code follows the target layout (CLAUDE.md), never the legacy shape. Do not add a new root module or a new allow-list line.
- Touching a legacy module for a task: only move what the task needs. A later plan task refactors the rest (`specs/re-architecture-system/plan.md`); do not widen the PR.
- The layout, model, usecase and service checks run on the Agent only for now; follow them in the Engine anyway.

## Adding an operation: touch exactly these
1. Spec first: `docs/service-contracts.md` and `proto/fga/demo/engine/v1/engine.proto` (then `make proto-gen`; `make proto` lints, checks generated stubs and breaking changes). Keep the Agent/API callers in mind: a new Engine operation needs a proto method, a gateway method on the caller's side, one `@rpc` adapter method and a mapper pair.
2. Model/rules: business data in a model module (data only, stdlib only); pure geometry/rules stay deterministic and unit-tested (`domain/layout.py`, `building.py`, `checks.py`, `fixtures.py`).
3. `domain/gateway/<kind>_gateway.py` for each outside dependency (DXF read/draw/patch, rendering, conversion): exactly one Protocol per file. Implemented in `infrastructure/` (ezdxf, ifcopenshell, OpenCV, pdfium live only there).
4. `usecase/<operation>.py` (Protocol, verb methods) + `service/<operation>.py` (the implementation, same file name). Deps through the constructor.
5. `handler/grpc/<domain>_handler.py`: decode, call ONE use case, encode through `handler/grpc/mapper/<model>_mapper.py` (both ways, no decisions). At most 15 statements, no loops. Errors via `common/error_helper.py`. `servicer.py` only delegates. No `*_pb2` message module outside `mapper/`.
6. `wire.py`: build the graph there and nowhere else; no layer imports it.
7. Tests in `blueprint-engine/tests/` (no LLM): pure geometry and checks, services with fake gateways, real-gRPC handler tests (`test_grpc.py`). Bug fixes: failing test first, then the fix without editing that test. `make engine-eval` (live LLM golden prompts) is only for prompt behavior and is not part of `make check`.

## Rules that bite
- Determinism: same input, same output; layouts stay stable against the previous version. No randomness, no clock, no network.
- Inner layers (`domain`, `usecase`, `service`) import no SDK (`grpc`, `ezdxf`, `ifcopenshell`, `cv2`, `pypdfium2`, `rapidocr`, `fastapi`, protobuf) and never `infrastructure`, `handler`, `gen`.
- The Engine never imports Agent code and never sees tenant or user ids; the Agent never imports Engine code.
- Name things in the business domain (`PlanCheck`, `BriefOpsApplier`), not the mechanism. One gateway per file; no `utils`/`helpers`/`results` catch-alls.
- Pinned versions: exact pins in `blueprint-engine/pyproject.toml`, locked by `uv.lock`; changing a pin updates both and `CLAUDE.md` Conventions.
- Uploads (PDF, picture) are converted and discarded; the Engine stores nothing.

## Done means
`make check` passes (architecture-python and shape checks, proto checks, `blueprint-engine` pytest). Review your diff against the Python layering checklist in `docs/harness/evaluator-rubric.md`. Docs aligned: `blueprint-engine/README.md`, `docs/service-contracts.md`, `plan.md`, `docs/harness/progress.md`. When the Go API behavior changes with it, also follow `go-api`; run `make verify-live` and `authz-reviewer` only if Go `service/`, `fga/` or `model.fga` changed.
