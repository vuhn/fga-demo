# Blueprint Agent

All LLM work of the blueprint product, as a stateless gRPC service (`fga.demo.agent.v1.AgentService`, :8091). It calls the
deterministic Engine (:8092) only as tools and never imports its code. Layers: `domain` / `usecase` / `service` /
`infrastructure` / `handler`; `wire.py` is the only composition root. Contracts: `docs/service-contracts.md`.

```bash
make agent        # serve on :8091 with reload (needs the Engine on :8092 and Ollama)
make agent-test   # pytest, no LLM
```

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
| An RPC | `handler/grpc/servicer.py`: decode, call one use case with a `CallContext`, encode; errors through `handler/grpc/errors.py`, streams through `streaming.py` |

Calls carry a `CallContext` (request id) and a `ProgressReporter` explicitly; there are no ambient `ContextVar`s.
`scripts/check-architecture-python.sh` enforces the layer imports, one gateway per file, usecase/service pairs, no ambient state, prompts only in `service/`, dataclasses only in `domain/model/` and `infrastructure/entity/`, and protobuf DTOs only in `mapper/`.

Health (`grpc.health.v1`) is NOT_SERVING until the Engine's tool list and catalog have loaded.
