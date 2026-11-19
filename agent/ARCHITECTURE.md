# Agent architecture

The Agent (`agent/`, gRPC :8091) owns every LLM call. It is stateless, has no auth and never sees a tenant or user id; only the API calls it, after its OpenFGA Checks. The model only *proposes* (a brief, edit operations, claims read from a message, search queries); the Engine's tools validate, repair and lay out, and code decides the flow. Contracts: `proto/fga/demo/agent/v1/agent.proto`, summary in `docs/service-contracts.md`. Setup, env and "where things go": `README.md`.

```text
API :8088 --gRPC--> Agent :8091 --gRPC tools--> Engine :8092
                       |
                       +--> Ollama :11434 (language model)
                       +--> web search and pages (price research only)
```

## Layers (dependencies point inward)

`domain/{model,gateway}` + `errors.py` -> `usecase/<op>.py` (+ `usecase/helper/`) -> `service/<op>.py` (+ `service/helper/`, `service/prompts/`) -> `infrastructure/<technology>/` (+ `infrastructure/entity/`) -> `handler/grpc/` (+ `mapper/`, `common/`). `wire.py` is the only composition root and `serve.py` the entry point. Rules and the "touch exactly these" checklist: CLAUDE.md and `.claude/skills/python-agent/SKILL.md`; `scripts/check-architecture-python.sh` and `scripts/check-python-shape.py` enforce what a script can.

## Operations

| RPC | Use case (service) | What it does |
| --- | --- | --- |
| `BriefFromText` | `BriefDrafter` | Model drafts a brief from the prompt (opt-in enhancer first); the Engine normalizes it. |
| `Revise` | `BriefReviser` | Model proposes ops for a chat message; the Engine applies and rebuilds; a refusal goes back to the model with the error, up to 3 attempts. Streams progress. |
| `RevisePlan` | `PlanReviser` | The same for an imported plan (no brief). |
| `Review` | `BriefReviewer` | Up to 3 rounds fixing failed checks; a round is kept only when the issue score drops. |
| `EstimateTurn` | `EstimateTurner` | One chat turn: `EstimateReader` (four narrow claim readings, grounded in the message) and `EstimateInterview` (slots decided by code) or, when the plan can be priced, `EstimatePricer` (`PriceResearcher` for the web) and `EstimateReplier` (reply from the figures only). Streams events; a 30 minute ceiling. |

## Boundaries

- The Engine is reached only through the typed `BlueprintEngine` gateway (`infrastructure/engine_grpc`); a test fails until every Engine RPC is used or deliberately left out.
- Streams: a refusal before the first message is a gRPC status with an `ErrorDetail`; after it, an in-band `Error` event (422, 502, 504 `timeout`, 500). `common/streaming_helper.py` runs a stream on a worker thread and cancels it when the client leaves (the progress callback raises `Cancelled`).
- The conversation state is opaque to the API (JSON, same shape as before the split); the API stores it and sends it back each turn.
