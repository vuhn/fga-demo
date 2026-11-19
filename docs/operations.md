# Operations reference

Local runtime, Docker Compose, and configuration. Overview: [ARCHITECTURE.md](../ARCHITECTURE.md).

## Runtime

```text
browser :5173
    |  /api  proxied to :8088
    v
Gin API :8088
    |                 \
    |                  \
Postgres :5432         OpenFGA HTTP :8080        blueprint-engine :8090 (host); engine gRPC :8092 (host)
  db fgademo             gRPC :8081                  |
  db openfga             playground :3000            v
                                                   Ollama :11434 (host, Metal)
```

The blueprint engine (`blueprint-engine/`, Python FastAPI, `make engine`) and Ollama run natively on the host, because Docker on macOS has no GPU. Only the API calls the engine (the Agent will call its tools too); it is stateless and has no auth. The Engine also serves gRPC (`make engine-grpc`, `python -m fga_engine.serve`, address `GRPC_ADDR`, default `0.0.0.0:8092`, `GRPC_WORKERS` default 8; restarts on source changes). It runs next to the legacy HTTP engine and the API calls it for every deterministic blueprint operation (layout, plan edits, DXF import, convert, patch, render) when `BLUEPRINT_ENGINE_ADDR` is set (for example `localhost:8092`); unset, those calls stay on the HTTP engine. The blueprint LLM operations (draft from text, chat revise of a brief or a plan, review) go to the Agent over gRPC when `AGENT_ADDR` is set (for example `localhost:8091`; progress lines of a chat stream through unchanged); unset, they stay on the HTTP engine. The estimate chat (`EstimateTurn`) goes to the Agent too, with a 31 minute ceiling (the Agent's own is 30 and answers with a 504 `timeout`); the browser's SSE events are unchanged, and a browser that leaves cancels the Agent's turn and saves nothing. Restart the API after changing it; a gRPC engine that is down is the same 502 `engine_unavailable`, and a refusal is the same 400. The API sends only `x-request-id` (no tenant or user id). Health: `grpc.health.v1` on the same port. The Agent (`agent/`, `make agent`, `python -m fga_agent.serve`, `GRPC_ADDR` default `0.0.0.0:8091`, `GRPC_WORKERS` default 8, `ENGINE_ADDR` default `localhost:8092`, plus the `OLLAMA_URL` / `BLUEPRINT_TEXT_MODEL` / `BLUEPRINT_STREAM_THINKING` variables) is the scaffold of the LLM half: its health is NOT_SERVING until the Engine's tool list and catalog have loaded (it retries with a pause that grows from 2 s to 30 s and logs one warning per distinct error, so start order does not matter and a down Engine is not a log flood), its RPCs answer UNIMPLEMENTED until T11a onward, and nothing calls it yet. `make agent-test` runs its pytest (no LLM, no Engine needed). Pull the models once: `ollama pull qwen3` (text, the 8B, installed as `qwen3:latest`; `qwen3:14b` with 32 GB) (text; only describe and chat need it, uploads of DXF, PDF and pictures use no model). Engine variables: `OLLAMA_URL` (default `http://localhost:11434`), `BLUEPRINT_TEXT_MODEL` (default `qwen3:latest`). Without the engine or a model, blueprint draft, create, revise, edit, and drawing routes return 502 `engine_unavailable`; everything else works. While a chat shows the model's reasoning (`BLUEPRINT_STREAM_THINKING`, default on), the reasoning has a budget: `BLUEPRINT_THINKING_MAX_CHARS` (default 6000) and `BLUEPRINT_THINKING_MAX_SECONDS` (default 90). A model that runs past either (qwen3 can loop in its reasoning at temperature 0) is cut and the same call is redone without thinking, with a `retry` progress line.

`deploy/docker-compose.yml` runs Postgres, the OpenFGA migrate one-shot, and OpenFGA. The API and Vite run on the host so rebuilds stay fast.

Postgres healthcheck uses `pg_isready`. The migrate service `depends_on` Postgres with `condition: service_healthy` and runs:

```text
migrate
```

OpenFGA `depends_on` the migrate service completing successfully.

Environment for migrate and OpenFGA:

```text
OPENFGA_DATASTORE_ENGINE=postgres
OPENFGA_DATASTORE_URI=postgres://openfga:password@postgres:5432/openfga?sslmode=disable
OPENFGA_PLAYGROUND_ENABLED=true
```

Leave the ListObjects deadline at its default. OpenFGA v1.22 refuses to start when `OPENFGA_LIST_OBJECTS_DEADLINE` exceeds the request timeout (6s by default).

Do not set `OPENFGA_AUTHN_METHOD`. Local demo traffic is unauthenticated at the OpenFGA boundary. The application API is the authenticated edge.

Published ports: Postgres `5432`, OpenFGA HTTP `8080`, gRPC `8081`, playground `3000`.

`deploy/postgres/init.sql` runs on first Postgres start:

```sql
CREATE USER fgademo WITH PASSWORD 'password';
CREATE DATABASE fgademo OWNER fgademo;
CREATE USER openfga WITH PASSWORD 'password';
CREATE DATABASE openfga OWNER openfga;
```

The compose `POSTGRES_USER` is `postgres` and `POSTGRES_PASSWORD` is `password`. That superuser creates the two databases above.

## Configuration

`.env.example` contains:

```text
HTTP_ADDR=:8088
DATABASE_URL=postgres://fgademo:password@localhost:5432/fgademo?sslmode=disable
OPENFGA_API_URL=http://localhost:8080
OPENFGA_STORE_NAME=fga-demo
OPENFGA_MODEL_PATH=fga/model.fga
JWT_SECRET=dev-secret-change-me
JWT_TTL=12h
CORS_ORIGIN=http://localhost:5173
BLUEPRINT_ENGINE_URL=http://localhost:8090
# BLUEPRINT_ENGINE_ADDR=localhost:8092   # optional: deterministic blueprint calls over gRPC
# AGENT_ADDR=localhost:8091              # optional: blueprint LLM calls over gRPC (blueprint chat and estimate chat)
```

Cost estimation (engine variables, read by `make engine`): the agent researches unit prices on the web, so it needs a search API. `ESTIMATE_SEARCH_PROVIDER` is `brave` (Brave Search API) or `serper` (serper.dev) and `ESTIMATE_SEARCH_API_KEY` its key; `demo` needs no key: a fake search returns invented price pages (host `demo.invalid`) for every item, the LLM and the quote checks run for real, and the estimate opens with a "DEMO DATA" warning, so use it to see the whole flow without a key, never for a real total; with `none` (the default) or no key every price comes back "not found", the estimate lists the lines as unpriced, and the person can enter rates by hand. `ESTIMATE_SEARCH_PARALLEL` (default 3) is how many prices are researched at once, `ESTIMATE_USD_VND` (default 25000) converts a price quoted in USD. Rates are cached in memory for 7 days per (item, region, quality), an optimisation only. Search queries carry only an item description, unit, region and quality grade, never tenant, project, user or plan data nor the person's words; the engine fetches the result pages itself (robots.txt respected, 2 MB cap, text only) and treats their text as untrusted data. The agent's language model is `BLUEPRINT_TEXT_MODEL`. Once the Agent serves the estimate (`make agent`), the same variables are read by the Agent, not the engine: `ESTIMATE_SEARCH_PROVIDER`, `ESTIMATE_SEARCH_API_KEY`, `ESTIMATE_SEARCH_PARALLEL`, `ESTIMATE_USD_VND`, `ESTIMATE_RULES` and `ESTIMATE_TURN_SECONDS` (the Agent's own ceiling for a turn, default 1800). Each session is capped at 40 turns by the API. An agent turn is not bound by the engine client's usual 5-minute timeout (a turn that researches many prices on a local model runs longer): the API allows it up to 30 minutes, and closing the connection cancels it. The local qwen3 takes about 20 s per researched price on one model, so a first estimate with search takes minutes; progress and each price stream into the chat. Inside one price, the searches and the page fetches also run at once; the model reads the pages in turn. Ollama serves one request at a time by default, so more than one price at a time only helps the network side: to let the model work on several prices together, start Ollama with `OLLAMA_NUM_PARALLEL=3` (costs more memory for the context) and keep `ESTIMATE_SEARCH_PARALLEL` equal to it.

`cmd/api` and `cmd/seed` run from `api/` and load the first `.env` they find (`api/.env`, then the repository root `.env`), without overriding variables already set in the process. A missing `.env` falls back to the defaults above. `OPENFGA_MODEL_PATH` is relative to `api/`.

`.gitignore` includes `.env`, `web/node_modules`, and `web/dist`.

