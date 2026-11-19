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
mkdir -p "$t/agent/src/fga_agent/handler/grpc"
echo 'x = 1' >"$t/agent/src/fga_agent/handler/grpc/brief.py"
echo 'from ...service import brief_drafter' >"$t/agent/src/fga_agent/handler/grpc/estimate_handler.py"
mkdir -p "$t/agent/src/fga_agent/handler/grpc/common"
echo 'x = 1' >"$t/agent/src/fga_agent/handler/grpc/common/streaming.py"
mkdir -p "$t/agent/src/fga_agent/handler/grpc/mapper"
python3 - "$t/agent/src/fga_agent/handler/grpc" <<'PY'
import sys
d = sys.argv[1]
open(d + "/blueprint_handler.py", "w").write("class H:\n    def revise(self, r, c):\n        self._uc.a.go(r)\n        self._uc.b.go(r)\n        for x in r:\n            pass\n" + "        y = 1\n" * 16)
open(d + "/mapper/big_mapper.py", "w").write("def to_pb(x):\n" + "    y = 1\n" * 26)
PY
mkdir -p "$t/agent/src/fga_agent/domain/model" "$t/agent/src/fga_agent/usecase/helper"
printf 'import threading\nclass Token:\n    pass\n' >"$t/agent/src/fga_agent/domain/model/token.py"
printf 'from dataclasses import dataclass\n@dataclass\nclass P:\n    x: int\n    @classmethod\n    def make(cls):\n        return cls(1)\nAlias = int\n' >"$t/agent/src/fga_agent/domain/model/factory.py"
echo 'x = 1' >"$t/agent/src/fga_agent/domain/text_helper.py"
echo 'class R: ...' >"$t/agent/src/fga_agent/usecase/helper/rule.py"
mkdir -p "$t/agent/src/fga_agent/service/prompts"
printf 'from typing import Protocol\nclass Wrong(Protocol):\n    def go(self): ...\n' >"$t/agent/src/fga_agent/usecase/thing.py"
echo 'class Other: ...' >"$t/agent/src/fga_agent/service/thing.py"
echo 'class P: ...' >"$t/agent/src/fga_agent/service/prompts/draft.py"
echo 'x = 1' >"$t/agent/src/fga_agent/loose.py"
echo 'x = 1' >"$t/agent/src/fga_agent/infrastructure/loose.py"
echo 'from ...wire import App' >"$t/agent/src/fga_agent/handler/grpc/servicer.py.tmp" && mv "$t/agent/src/fga_agent/handler/grpc/servicer.py.tmp" "$t/agent/src/fga_agent/handler/grpc/wired.py"
# the Engine's delivery layer is held to the same rules
e="$t/blueprint-engine/src/fga_engine/handler/grpc"; mkdir -p "$e/mapper"
echo 'from ...wire import Engine' >"$e/wired.py"
echo 'from ...gen.fga.demo.engine.v1 import engine_pb2 as e' >"$e/pb_direct.py"
echo 'x = 1' >"$e/stray.py"
printf 'class H:\n    def Go(self, request, context):\n        self._uc.a.run()\n        for x in request:\n            self._uc.b.run()\n' >"$e/bad_handler.py"
out=$(ROOT="$t" ALLOW=/dev/null bash scripts/check-architecture-python.sh || true)
for rule in inner-sdk inner-infra engine-no-network engine-no-llm agent-no-engine gateway-per-kind usecase-service-pair agent-no-ambient-state agent-prompts-in-service agent-model-folder agent-entity-folder agent-dto-in-mapper agent-handler-shape agent-thin-handler agent-pure-mapper agent-model-data-only agent-helper-in-layer agent-usecase-interface agent-service-implements agent-layer-layout agent-wire-import; do
  echo "$out" | grep -q "\[$rule\]" || { echo "planted violation not flagged: $rule"; exit 1; }
done
for planted in wired.py pb_direct.py stray.py bad_handler.py; do
  echo "$out" | grep -q "blueprint-engine/src/fga_engine/handler/grpc/$planted" || { echo "planted Engine handler violation not flagged: $planted"; exit 1; }
done
echo "python architecture rules flag every planted violation"
