#!/usr/bin/env bash
# Plants one violation per rule in a temp tree and asserts the Python architecture check flags each one.
set -eu
cd "$(dirname "$0")/.."
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
mkdir -p "$t/blueprint-engine/src/fga_engine/"{domain,service,usecase,infrastructure} "$t/agent/src/fga_agent/"{domain,service}
echo 'import ezdxf' >"$t/blueprint-engine/src/fga_engine/domain/bad.py"
echo 'from fga_engine.infrastructure import x' >"$t/blueprint-engine/src/fga_engine/service/bad.py"
echo 'import httpx' >"$t/blueprint-engine/src/fga_engine/infrastructure/net.py"
echo 'from . import prompts' >"$t/blueprint-engine/src/fga_engine/infrastructure/ask.py"
echo 'from fga_engine.domain import x' >"$t/agent/src/fga_agent/service/bad.py"
echo 'import grpc' >"$t/agent/src/fga_agent/domain/bad.py"
out=$(ROOT="$t" ALLOW=/dev/null bash scripts/check-architecture-python.sh || true)
for rule in inner-sdk inner-infra engine-no-network engine-no-llm agent-no-engine; do
  echo "$out" | grep -q "\[$rule\]" || { echo "planted violation not flagged: $rule"; exit 1; }
done
echo "python architecture rules flag every planted violation"
