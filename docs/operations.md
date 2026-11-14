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
Postgres :5432         OpenFGA HTTP :8080        blueprint-engine :8090 (host)
  db fgademo             gRPC :8081                  |
  db openfga             playground :3000            v
                                                   Ollama :11434 (host, Metal)
```

The blueprint engine (`blueprint-engine/`, Python FastAPI, `make engine`) and Ollama run natively on the host, because Docker on macOS has no GPU. Only the API calls the engine; it is stateless and has no auth. Pull the models once: `ollama pull qwen3` (text, the 8B, installed as `qwen3:latest`; `qwen3:14b` with 32 GB) and `ollama pull qwen2.5vl:7b` (vision, only for sketch upload). Engine variables: `OLLAMA_URL` (default `http://localhost:11434`), `BLUEPRINT_TEXT_MODEL` (default `qwen3:latest`), `BLUEPRINT_VISION_MODEL` (default `qwen2.5vl:7b`). Without the engine or a model, blueprint draft, create, revise, edit, and drawing routes return 502 `engine_unavailable`; everything else works.

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
```

`cmd/api` and `cmd/seed` run from `api/` and load the first `.env` they find (`api/.env`, then the repository root `.env`), without overriding variables already set in the process. A missing `.env` falls back to the defaults above. `OPENFGA_MODEL_PATH` is relative to `api/`.

`.gitignore` includes `.env`, `web/node_modules`, and `web/dist`.

