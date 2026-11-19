# Blueprint Agent

All LLM work of the blueprint product, as a stateless gRPC service (`fga.demo.agent.v1.AgentService`, :8091). It calls the
deterministic Engine (:8092) only as tools and never imports its code. Layers: `domain` / `usecase` / `service` /
`infrastructure` / `handler`; `wire.py` is the only composition root. Contracts: `docs/service-contracts.md`.

```bash
make agent        # serve on :8091 with reload (needs the Engine on :8092 and Ollama)
make agent-test   # pytest, no LLM
```

Health (`grpc.health.v1`) is NOT_SERVING until the Engine's tool list and catalog have loaded.
