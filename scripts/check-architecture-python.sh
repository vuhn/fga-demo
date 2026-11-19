#!/usr/bin/env bash
# Layer rules for the Python services (blueprint-engine, agent), mirroring the Go rules in
# check-architecture.sh. Only directories that exist are scanned. Known violations live in
# scripts/python-arch-allow.txt (`<rule><TAB><path>`); the list only shrinks and must be empty when the
# re-architecture finishes (specs/re-architecture-system/plan.md, T16).
#   ROOT=<dir> scripts/check-architecture-python.sh      (ROOT defaults to the repository root; tests plant violations)
set -u
ROOT=${ROOT:-$(cd "$(dirname "$0")/.." && pwd)}
ALLOW=${ALLOW:-$ROOT/scripts/python-arch-allow.txt}
fail=0

allowed() { [ -f "$ALLOW" ] && grep -v '^#' "$ALLOW" | grep -qxF "$1"$'	'"$2"; }
report() { # rule path why fix
  allowed "$1" "$2" && return
  printf '\nFAIL: [%s] %s\n  why: %s\n  fix: %s\n' "$1" "$2" "$3" "$4"; fail=1
}
scan() { # rule regex why fix dir...   (files under dir importing a module that matches regex)
  local rule=$1 re=$2 why=$3 fix=$4; shift 4
  local d f
  for d in "$@"; do
    [ -d "$d" ] || continue
    while IFS= read -r f; do report "$rule" "${f#$ROOT/}" "$why" "$fix"; done < <(grep -rlE "^[[:space:]]*(from|import)[[:space:]]+($re)" --include='*.py' "$d" 2>/dev/null | grep -v '/gen/')
  done
}

SDK='grpc|httpx|ollama|ezdxf|ifcopenshell|cv2|pypdfium2|rapidocr[a-z_]*|google\.protobuf|requests|aiohttp|urllib\.request|socket|fastapi|uvicorn'
# 1. Inner layers import no transport, storage or file-format SDKs and never the generated stubs or infrastructure.
inner=()
for svc in "$ROOT"/blueprint-engine/src/* "$ROOT"/agent/src/*; do
  [ -d "$svc" ] || continue
  for layer in domain usecase service; do inner+=("$svc/$layer"); done
done
scan inner-sdk "$SDK" "domain, usecase and service depend on interfaces so they stay testable and backend-agnostic" \
  "move the SDK call behind a Protocol in domain/gateway and implement it under infrastructure/" "${inner[@]}"
scan inner-infra '[a-z_.]*(infrastructure|handler|gen)(\.|[[:space:]]|$)' "inner layers never import infrastructure, handlers or generated stubs" \
  "depend on a Protocol in domain/gateway; wire.py injects the implementation" "${inner[@]}"
# 2. The engine has no LLM and no network egress.
scan engine-no-network 'httpx|ollama|requests|aiohttp|urllib\.request|socket|http\.client' \
  "the Engine is deterministic: no LLM, no database, no network" "move the call to the Agent" "$ROOT"/blueprint-engine/src/*
scan engine-no-llm '[a-z_.]*(llm|prompts)(\.|[[:space:]]|$)|\.[[:space:]]+import[[:space:]]+[a-z_, ]*(llm|prompts)' \
  "LLM code belongs to the Agent" "move it to agent/" "$ROOT"/blueprint-engine/src/*
# 3. The Agent never imports Engine code; it only calls the Engine through the generated client.
scan agent-no-engine 'fga_engine|blueprint_engine' "the Agent talks to the Engine only over gRPC" \
  "call an Engine tool through infrastructure/engine_grpc" "$ROOT"/agent/src/*

[ $fail = 0 ] && echo "python architecture: ok"
exit $fail
