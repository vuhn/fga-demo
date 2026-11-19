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
mkdir -p "$t/agent/src/fga_agent/domain/gateway"
printf 'class A: ...\nclass B: ...\n' >"$t/agent/src/fga_agent/domain/gateway/__init__.py"
printf 'class A: ...\nclass B: ...\n' >"$t/agent/src/fga_agent/domain/gateway/two_gateway.py"
mkdir -p "$t/agent/src/fga_agent/"{usecase,infrastructure}
echo 'class X: ...' >"$t/agent/src/fga_agent/usecase/orphan.py"
echo 'from contextvars import ContextVar' >"$t/agent/src/fga_agent/domain/ambient.py"
echo 'PROMPT = "x"' >"$t/agent/src/fga_agent/infrastructure/prompts.py"
mkdir -p "$t/agent/src/fga_agent/"{handler/grpc/mapper,infrastructure/ollama}
printf '@dataclass\nclass M: ...\n' >"$t/agent/src/fga_agent/domain/loose_model.py"
printf '@dataclass\nclass E: ...\n' >"$t/agent/src/fga_agent/infrastructure/ollama/wire.py"
echo 'from ...gen.fga.demo.agent.v1 import agent_pb2 as a' >"$t/agent/src/fga_agent/handler/grpc/servicer.py"
echo 'x = 1' >"$t/agent/src/fga_agent/handler/grpc/mapper/helpers.py"
out=$(ROOT="$t" ALLOW=/dev/null bash scripts/check-architecture-python.sh || true)
for rule in inner-sdk inner-infra engine-no-network engine-no-llm agent-no-engine gateway-per-kind usecase-service-pair agent-no-ambient-state agent-prompts-in-service agent-model-folder agent-entity-folder agent-dto-in-mapper; do
  echo "$out" | grep -q "\[$rule\]" || { echo "planted violation not flagged: $rule"; exit 1; }
done
echo "python architecture rules flag every planted violation"
