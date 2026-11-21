---
name: python-agent
description: Implement or change Python code in agent/ (the gRPC Agent: all LLM work, Ollama, calls the Engine as tools) following the repo's Clean Architecture mixed with DDD. Use for any task touching agent/src/fga_agent, agent/tests, or the Agent's gRPC API.
---

# Implement Agent changes

Source of truth: `docs/harness/layer-structure.md` (the common layer structure), CLAUDE.md "Architecture rules" (Python services bullet), `agent/README.md` (layers, "Where things go"), `docs/service-contracts.md` (gRPC contracts), `docs/harness/evaluator-rubric.md` (Python layering checklist). This skill is the working checklist; read those when unsure.

## The Agent in one paragraph
Stateless gRPC service (`fga.demo.agent.v1.AgentService`, :8091). It owns every LLM call. It talks to the Engine only as gRPC tools over the generated client (never imports Engine code) and to Ollama through a gateway. The LLM only proposes JSON (brief, ops); the Engine validates, repairs and lays out. It never sees a tenant or user id.

## Layers (dependencies point inward)
`domain/{model,gateway}` + `errors.py` -> `usecase/<op>.py` (+ `usecase/helper/`) -> `service/<op>.py` (+ `service/helper/`, `service/prompts/`) -> `infrastructure/<technology>/` (+ `infrastructure/entity/`) -> `handler/grpc/`. `wire.py` is the only composition root; no layer imports it. The package root holds only `wire.py` and `serve.py`. Folder names are singular.

## Adding an operation: the usual steps
Read `docs/harness/layer-structure.md` first. These are the usual steps, in order, for a new operation; each points at the rule it applies. Skip a step that does not apply. For a case they do not cover, decide with "When the steps don't fit" in that doc and write the decision in the task contract.
1. Spec first: `docs/service-contracts.md` and `proto/fga/demo/agent/v1/agent.proto` (then `make proto-gen`; `make proto` lints and checks generated stubs and breaking changes).
2. `domain/model/<concept>.py` (data only, one file per concept; layer-structure.md "Layers"). Errors go in `domain/errors.py`.
3. `domain/gateway/<collaborator>_gateway.py` only for a NEW outside dependency (one Protocol per file, named for what the domain asks: `BlueprintEngine`, `LanguageModel`). A progress sink is a plain callback, not a gateway.
4. `usecase/<operation>.py`: signature rules in layer-structure.md "Operation signatures" (`CallContext` first; long runs take `on_progress`, which may raise `Cancelled`).
5. `service/<operation>.py`: `<Operation>Service`, the business steps (validate, ask the Engine, ask the model, retry with feedback). Same file name as the use case. Dependencies come in through the constructor as Protocols. Prompts are pure functions in `service/prompts/<operation>.py` (inputs in, text out, no classes).
6. Shared business rule: a Protocol in `usecase/helper/<name>.py` implemented in `service/helper/<name>.py` (paired). Never in `domain/`, never a `utils`/`helpers` module.
7. `infrastructure/<technology>/`: the adapter for a gateway (`ollama`, `engine_grpc`). Technology wire shapes go in `infrastructure/entity/<technology>_<shape>.py`, not dicts in the adapter. Model-specific tuning stays here, not in prompts. A new Engine operation = a proto method, a typed method on `BlueprintEngine`, one `@rpc` adapter method, and a mapper pair.
8. `handler/grpc/<domain>_handler.py`: own `<Domain>Usecases`, thin (rule 5; shape checked by `check-python-shape.py`). Errors via `common/error_helper.py`, streams via `common/streaming_helper.py`; `servicer.py` only delegates.
9. `handler/grpc/mapper/<model>_mapper.py` and `infrastructure/engine_grpc/mapper/<model>_mapper.py`: one per domain model, both ways; no `*_pb2` message module outside `mapper/`.
10. `wire.py`: build the service, the handler's `<Domain>Usecases` and the handler. Nothing else builds the graph.
11. Tests in `agent/tests/`: service tests with the scripted/fake Engine and fake language model (no real LLM), real-gRPC handler tests, mapper tests. Bug fixes: failing test first, then the fix without editing that test.

## Rules that bite
- Generated stubs are shared: import `from fga.demo.<svc>.v1 import <svc>_pb2` (package `fga`, from `proto/python` via the `fga-proto` path dependency), only in a `mapper/`. Never generate stubs into this service or hand edit them; change the `.proto` and run `make proto-gen`.
- Name things in the business domain (`BriefDrafter`, `PlanCheck`), not the mechanism (`Tool`, `Registry`, `Loader`).
- No `contextvars`; no ambient state. Dependencies are explicit arguments.
- `domain/model` is data only; `entity` is only a technology's own wire/row shape. A token, an exception, a factory or a callback type is not a model.
- Inner layers (`domain`, `usecase`, `service`) import no SDK (`grpc`, `httpx`, `ollama`, `ezdxf`, protobuf) and never `infrastructure`, `handler`, `gen`.
- Streaming: a refusal before the first event is a gRPC status with an `ErrorDetail`; after the first event it is an in-band `Error` event. Cancel by raising `Cancelled` from the progress callback.
- No function over 60 statements; business rules (scores, defaults, summaries) are computed in services, never in the delivery layer.
- Do not add an allow-list line in `scripts/python-arch-allow.txt`; fix the code (the list only shrinks).

## Done means
`make check` passes (it runs `scripts/check-architecture-python.sh`, `scripts/check-python-shape.py`, `agent` pytest, and the proto checks). Review your diff against the Python layering checklist in `docs/harness/evaluator-rubric.md`. Docs aligned: `agent/README.md`, `docs/service-contracts.md`, the task's `specs/*/test-map.md` and `plan.md`, `docs/harness/progress.md`. No Go `service/` or `fga/` change means `make verify-live` and `authz-reviewer` are not needed.
