#!/usr/bin/env bash
# One-shot environment readiness: tools present, .env in place, deps installed, baseline green.
set -eu
cd "$(dirname "$0")/.."

need() { command -v "$1" >/dev/null || { echo "missing: $1 ($2)"; exit 1; }; }
need go "Go 1.26+"
need node "Node 20+"
need npm "bundled with node"
need docker "needed for make compose-up"
need uv "Python toolchain for blueprint-engine (brew install uv)"

[ -f .env ] || { cp .env.example .env; echo "created .env from .env.example"; }

# ~/.npm may hold root-owned files; use a throwaway cache instead of sudo.
(cd web && npm_config_cache="$(mktemp -d)" npm ci)
(cd blueprint-engine && uv sync --frozen)

make check

cat <<'MSG'

ready. Live stack, in order:
  make compose-up && make seed && make api    # restart api after every reseed
  make web
  ollama serve && make engine                 # blueprints: native Ollama + engine on :8090
  make verify-live                            # integration tests (RESETS data)
MSG
