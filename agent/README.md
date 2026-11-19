# Blueprint Agent

All LLM work of the blueprint product, as a stateless gRPC service (`fga.demo.agent.v1.AgentService`, :8091). It calls the
deterministic Engine (:8092) only as tools and never imports its code. Layers: `domain` / `usecase` / `service` /
`infrastructure` / `handler`; `wire.py` is the only composition root. Contracts: `docs/service-contracts.md`.

```bash
make agent        # serve on :8091 with reload (needs the Engine on :8092 and Ollama)
make agent-test   # pytest, no LLM
```

## Layers

```text
request -> handler -> usecase (interface) -> service -> gateway (interface) -> infrastructure -> Ollama, Engine, ...
```

`handler/grpc/` is the delivery layer: the `<domain>_handler.py` endpoint classes plus the delivery plumbing around them (`servicer.py`, `server.py` with health, `errors.py`, `mapper/`, and `common/*_helper.py` for what the handlers share, e.g. `streaming_helper.py`). It decodes, calls one use case and encodes; it never holds business rules and never imports a service or an adapter (`wire.py` injects them).

## Where things go

| Need | Put it in |
| --- | --- |
| A new outside dependency (the model, the Engine, price search, page fetch) | a Protocol in `domain/gateway/<collaborator>_gateway.py`, named for what the domain asks of it, one per file, re-exported in `__init__.py`; the adapter in `infrastructure/<technology>/` |
| A concept the services pass around | `domain/model/<concept>.py` (`brief`, `plan`, `plan_check`, `design_limits`, ...), stdlib only |
| A technology's wire shape (an Ollama request, a chunk) | `infrastructure/entity/<technology>_<shape>.py` |
| Converting protobuf DTOs | `handler/grpc/mapper/<model>_mapper.py` (Agent API: domain -> response, error -> status) or `infrastructure/engine_grpc/mapper/<model>_mapper.py` (Engine response -> domain) |
| A business operation | `usecase/<operation>.py` (interface, verb methods) + `service/<operation>.py` (`<Operation>Service`, the logic), wired in `wire.py` |
| A prompt | `service/prompts/<operation>.py`, pure functions; model-specific tuning stays in `infrastructure/ollama` |
| A new Engine operation | a method in `engine.proto`, a typed method on `gateway.BlueprintEngine`, and its `@rpc` adapter in `infrastructure/engine_grpc/blueprint_engine.py` (a test fails until every `EngineService` method is used or deliberately left out) |
| An RPC | a method of `handler/grpc/<domain>_handler.py` (with its own `<Domain>Usecases`): decode, call one use case with a `CallContext`, encode through `mapper/`; `servicer.py` only delegates; errors through `errors.py`, streams through `common/streaming_helper.py` |

Calls carry a `CallContext` (request id) and a `ProgressReporter` explicitly; there are no ambient `ContextVar`s.
Mappers convert both ways (build/decode the request, encode/read the response). `scripts/check-architecture-python.sh` (with `scripts/check-python-shape.py`: thin handlers, small mappers, small functions) enforces the layer imports, one gateway per file, usecase/service pairs, no ambient state, prompts only in `service/`, dataclasses only in `domain/model/` and `infrastructure/entity/`, and protobuf DTOs only in `mapper/`.

Health (`grpc.health.v1`) is NOT_SERVING until the Engine's tool list and catalog have loaded.
