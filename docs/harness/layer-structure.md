# Source code layer structure

The target structure of every backend service: `api/` (Go), `agent/` and `blueprint-engine/` (Python). See "Conformance" for how far each service is today. Each service's `ARCHITECTURE.md` shows its own layers and links here; its skill (`go-api`, `python-agent`, `python-engine`) is the "usual steps" checklist for it. `web/` is a React app with its own `web/ARCHITECTURE.md` and is not covered. When this file and a service document disagree, fix the service document or change this standard first (spec first).

## Conformance (today)

| Service | State | What to do when adding code |
| --- | --- | --- |
| `api/` | Follows the standard; every rule is checked by script. | Follow it. |
| `agent/` | Follows the standard; layout, model, usecase, service, mapper and shape checks run on it. | Follow it. |
| `blueprint-engine/` | Partly. The delivery side follows it and is checked: `handler/grpc/<domain>_handler.py` with its own use cases, `mapper/`, `common/`, one gateway per file in `domain/gateway/`, `usecase/<op>.py` paired with `service/<op>.py`, `wire.py` as the only composition root. Not yet: `domain/` is flat modules (`building.py`, `layout.py`, `checks.py`, ...) instead of `domain/model/<concept>.py`, and a few modules sit at the package root (`estimate.py`, `estimate_models.py`, `pipeline.py`, `takeoff.py`). The layout, model, usecase and service checks do not run on these parts, so a script will not catch a misplaced file. | New code follows this standard (a new concept goes in `domain/model/<concept>.py`, no new root module). Touching a legacy module: move only what the task needs; do not widen the PR. The refactor of the rest is its own task (`specs/re-architecture-system/plan.md`); once done, the Python checks extend to the Engine and this row becomes "Follows". |

Where this standard and the Engine's existing code disagree, the standard wins for new code and the existing code is the known gap, not a precedent.

## Dependency direction

Dependencies point inward. Outer layers know inner ones, never the reverse.

```text
handler  ->  usecase (interfaces)  ->  domain/model
service  ->  usecase, usecase/helper, domain/gateway, domain/model
infrastructure -> domain/gateway, domain/model
wire (composition root) builds infrastructure, services and handlers and injects them
```

A request runs `handler -> usecase (interface) -> service -> gateway (interface) -> infrastructure -> dependency` (database, OpenFGA, Ollama, another service). The handler never sees a service or an adapter; it sees only the usecase interface that the composition root injected.

## Layers

| Layer | Responsibility | Go (`api/internal/`) | Python (`<package>/`) |
| --- | --- | --- | --- |
| `domain/model` | Business data: entities, value types, sentinel errors. Data only, stdlib only. One file per concept. | `domain/model/*.go` | `domain/model/<concept>.py` (dataclasses and enums), `errors.py` |
| `domain/gateway` | What the business asks of the outside, as interfaces named for the collaborator, with business-named methods (not RPC or SQL names). | `domain/gateway/<collaborator>_gateway.go` | `domain/gateway/<collaborator>_gateway.py`, one Protocol per file, `__init__.py` only re-exports |
| `usecase` | One interface per operation, signatures only. Shared rule interfaces (not operations) live in `usecase/helper`. | `usecase/<domain>.go`, `usecase/input/<domain>.go`, `usecase/output/<domain>.go`, `usecase/helper/` | `usecase/<operation>.py` (a Protocol named after the file), `usecase/helper/` |
| `service` | Implements the usecases; all business logic (validate, authorize, orchestrate, order writes). One file per operation. | `service/<operation>.go`, `service/helper.go` | `service/<operation>.py` (`<Operation>Service`), `service/helper/`, `service/prompts/` |
| `infrastructure` | Adapts one technology and implements gateways. The only place an SDK, a driver or a wire format appears. | `infrastructure/{postgres,fga,agent,engine,auth,idgen,config,grpcutil}` | `infrastructure/<technology>/`, `infrastructure/entity/<technology>_<shape>.py` |
| `handler` | Delivery: decode the request, call ONE usecase, encode the response, map errors. No business rules. | `handler/http/<domain>_handler.go`, `handler/http/mapper/<domain>_mapper.go` | `handler/grpc/<domain>_handler.py`, `handler/grpc/mapper/`, `handler/grpc/common/<kind>_helper.py` |
| composition root | The only code that names concrete types and builds the graph. No layer imports it. | `cmd/api/wire.go` | `wire.py` (with `serve.py` as the entry point) |

Fixtures and tooling that are not a layer (the Go `seed` package, `cmd/*`) are not imported by the business layer.

## Rules (every service)

1. **Business layer independence.** `domain`, `usecase` and `service` import only the standard library and each other. No transport or storage SDK, no web framework, no third-party package, and never `infrastructure/`, `handler/` or the composition root. Needing one means a gateway interface in `domain/gateway` implemented in `infrastructure/`.
2. **Dependency injection.** Constructors take interfaces. Services hold no concrete types. Only the composition root builds the graph.
3. **One operation, one usecase, one service file.** A new operation is a usecase interface, a service file with the same name, and wiring. Operations are named for the business (`FolderCreator`, `BriefReviser`), not the mechanism.
4. **Each domain is separate in every layer** (its own model, gateways, repos, usecases, services, handler and routes). Do not merge domains behind a shared `Kind`-style abstraction or a generic service or handler; shared code is only cross-domain rules in `service/helper` and plumbing inside `infrastructure/`. Behavior stays identical across domains unless the spec says otherwise.
5. **Handlers are thin.** Decode, call one usecase method, encode, map errors. DTOs (JSON views, protobuf messages) are converted only in `mapper/`, one `<domain>_mapper` per domain, both directions. A handler holds no `...View` function and no DTO type.
6. **Wire shapes stay out of the business layer.** A technology's own row or wire shape is an `entity` inside `infrastructure/`; `model` is the business data in every service (the word `entity` is reserved for the technology's shape).
7. **Prompts are business behavior** (Python Agent): pure functions in `service/prompts/`; `infrastructure/` only adapts the model and its tuning.
8. **Folder names are singular** (`model`, `entity`, `mapper`, `gateway`, `helper`). No catch-all modules (`utils`, `helpers`, `results`, a growing `__init__`).
9. **Common code stays in its own layer:** handler helpers in `handler/.../common/`, shared business rules as an interface in `usecase/helper` implemented in `service/helper`, infrastructure helpers inside `infrastructure/`, none in `domain/`.

## Operation signatures

- **Go.** Every usecase method is `Method(ctx context.Context, in input.X) (output.X, error)` (or only `error`), where `X` is the interface name. `ctx` is the only parameter outside the struct, so the actor, the tenant and the ids live in `input.X`; a streaming callback such as `onProgress` stays a trailing parameter. `output.X` has its own copied fields and never a `domain/model` type, so a handler never sees a model through a usecase. Inputs and outputs live in `usecase/input/<domain>.go` and `usecase/output/<domain>.go`, one file per domain; services map model -> output, handler mappers map output -> JSON view and request body -> `input.X`. A `domain/gateway` method with more than 3 parameters after `ctx` takes one `model.<Action><Model>Params` struct.
- **Python.** A usecase method is a verb on the Protocol (`BriefDrafter.draft`) taking a `CallContext` (request id) first and, for long runs, a progress callback `Callable[[Progress], None]` (it may raise `Cancelled`), passed explicitly; no ambient `ContextVar`s. A gateway method has business names, not RPC names. (Input and output structs are a Go rule today; adopting them in Python is a separate decision.)

## Where things go

| Need | Put it in |
| --- | --- |
| A new outside dependency (a database, a model, another service) | An interface in `domain/gateway`, named for what the domain asks of it; the adapter in `infrastructure/<technology>/` |
| A concept the services pass around | `domain/model` |
| A business operation | A usecase interface, a service file of the same name, wiring in the composition root |
| A rule shared by several operations | An interface in `usecase/helper` implemented in `service/helper` |
| A new endpoint or RPC | A method of the domain's handler (decode, one usecase, encode through `mapper/`), plus its route or RPC registration |
| Converting a DTO | `mapper/<domain>_mapper` in the handler, or in the infrastructure client for a response it reads |
| A technology's wire shape | `infrastructure/.../entity` (Python) or inside the adapter (Go) |

## When the steps don't fit (edge cases)

A service skill's "usual steps" cover the common new operation. For anything else (changing existing code, a streaming RPC, a helper shared by operations, a gateway that spans domains, a cross-service change) decide in this order, and write the decision in the task contract:

1. Name the business concept first. Which `domain/model` type or `domain/gateway` owns it? If none does, that is the first thing to add.
2. Pick the layer by responsibility (the "Layers" table), not by the nearest similar file.
3. Check the dependency direction (rule 1). If the code needs an SDK or another layer's concrete type, add a gateway or helper interface instead.
4. Keep domains separate (rule 4). Shared logic goes only in `service/helper` or `usecase/helper`, never in a generic service or a shared abstraction.
5. Still unsure: choose the smaller change, say what you assumed, and ask at the contract gate.

Touching existing code: change only what the task needs; do not move or rename files outside the task's PR.

## Enforcement

| Rule | Checked by |
| --- | --- |
| Business layer imports, tuple literals, no role-string branching, no handler views (Go) | `scripts/check-architecture.sh` |
| Go usecase shape (`ctx` + `in input.X`, `input.X`/`output.X` declared), no model field in an output, at most 3 gateway params after `ctx`, business-layer imports | `scripts/check-go-shape.go` (run by `check-architecture.sh`), proven by `scripts/test-architecture-go.sh` |
| Python layout, model data-only, usecase/service pairing, mappers, no ambient state | `scripts/check-architecture-python.sh`, `scripts/check-python-shape.py`, proven by `scripts/test-architecture-python.sh` |
| What a script cannot judge (is it thin, is the name the domain's word) | The layering checklists in `docs/harness/evaluator-rubric.md` |

All of it runs in `make check`. A new rule is added to the script and its planted-violation test in the same change, then written here and in the service's skill.
