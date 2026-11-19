#!/usr/bin/env bash
# One-shot environment readiness: tools present, .env in place, deps installed, baseline green.
set -eu
cd "$(dirname "$0")/.."

need() { command -v "$1" >/dev/null || { echo "missing: $1 ($2)"; exit 1; }; }
export PATH="$PATH:$HOME/go/bin"
need go "Go 1.26+"
need node "Node 20+"
need npm "bundled with node"
need docker "needed for make compose-up"
need uv "Python toolchain for blueprint-engine (brew install uv)"
need buf "protobuf lint and codegen (brew install buf)"
need protoc-gen-go "GOBIN=$HOME/go/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 (and add ~/go/bin to PATH)"
need protoc-gen-go-grpc "GOBIN=$HOME/go/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"

[ -f .env ] || { cp .env.example .env; echo "created .env from .env.example"; }

# ~/.npm may hold root-owned files; use a throwaway cache instead of sudo.
(cd web && npm_config_cache="$(mktemp -d)" npm ci)
(cd blueprint-engine && uv sync --frozen)
(cd agent && uv sync --frozen)

make check

cat <<'MSG'

ready. Live stack, in order:
  make compose-up && make seed && make api    # restart api after every reseed
  make web
  ollama serve && make engine                 # blueprints: native Ollama + engine on :8090
  make engine-grpc && make agent             # gRPC engine :8092 and agent :8091 (re-architecture; nothing calls the agent yet)
  make verify-live                            # integration tests (RESETS data)
MSG
