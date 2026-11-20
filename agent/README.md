# Blueprint Agent

All LLM work of the blueprint product, as a stateless gRPC service (`fga.demo.agent.v1.AgentService`, :8091). It calls the deterministic Engine (:8092) only as tools. How it is built: [ARCHITECTURE.md](ARCHITECTURE.md). Contracts: `../docs/service-contracts.md`.

## Run

```bash
make agent        # serve on :8091 with reload (needs the Engine on :8092 and Ollama with qwen3:latest)
make agent-test   # pytest, no LLM
make agent-eval           # golden brief prompts on the live LLM
make agent-eval-estimate  # estimate reading, rules on and off
```

## Configuration

Read by `wire.py`: `GRPC_ADDR` (default `0.0.0.0:8091`), `GRPC_WORKERS` (8), `ENGINE_ADDR` (`localhost:8092`), `OLLAMA_URL` (`http://localhost:11434`), `BLUEPRINT_TEXT_MODEL` (`qwen3:latest`), `BLUEPRINT_STREAM_THINKING`, and for the estimate: `ESTIMATE_SEARCH_PROVIDER` (`brave`, `serper`, `demo`, `none`), `ESTIMATE_SEARCH_API_KEY`, `ESTIMATE_SEARCH_PARALLEL` (3), `ESTIMATE_USD_VND` (25000), `ESTIMATE_RULES` (`on`, `off` drops the keyword layer), `ESTIMATE_TURN_SECONDS` (1800). Full list: `../docs/operations.md`.
