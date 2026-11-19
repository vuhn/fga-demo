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

# 4. One gateway per file, like api/internal/domain/gateway/<kind>_gateway.go: `domain/gateway/<kind>_gateway.py`
# declares exactly one Protocol; `__init__.py` only re-exports.
for svc in "$ROOT"/blueprint-engine/src/* "$ROOT"/agent/src/*; do
  gw="$svc/domain/gateway"
  [ -d "$gw" ] || continue
  why="a gateway file per kind keeps each interface easy to find and review (same as the Go API)"
  if grep -qE '^(class|def) ' "$gw/__init__.py" 2>/dev/null; then
    report gateway-per-kind "${gw#$ROOT/}/__init__.py" "$why" "move each Protocol to <kind>_gateway.py and re-export it from __init__.py"
  fi
  for f in "$gw"/*.py; do
    [ "$(basename "$f")" = "__init__.py" ] && continue
    case "$f" in *_gateway.py) ;; *) report gateway-per-kind "${f#$ROOT/}" "$why" "name the file <kind>_gateway.py"; continue;; esac
    [ "$(grep -cE '^class ' "$f")" = 1 ] || report gateway-per-kind "${f#$ROOT/}" "$why" "declare exactly one Protocol per gateway file"
  done
done

# 5. One use case = one interface file in usecase/ and one implementation file of the same name in service/.
for svc in "$ROOT"/blueprint-engine/src/* "$ROOT"/agent/src/*; do
  for pair in "usecase service" "service usecase"; do
    set -- $pair
    [ -d "$svc/$1" ] || continue
    for f in "$svc/$1"/*.py; do
      n=$(basename "$f"); [ "$n" = "__init__.py" ] && continue
      [ -f "$svc/$2/$n" ] || report usecase-service-pair "${f#$ROOT/}" "a business operation is an interface in usecase/ and its implementation in service/, same file name" \
        "add $2/$n (or remove the orphan)"
    done
  done
done
# 6. The Agent passes dependencies explicitly (CallContext, a progress callback), never through ambient ContextVars.
scan agent-no-ambient-state 'contextvars' "hidden global state makes a use case's inputs invisible in its signature" \
  "pass a CallContext or a progress callback as an argument" "$ROOT"/agent/src/*
# 7. Prompts are business behavior: they live in service/ (service/prompts/), never in infrastructure, handler or domain.
for svc in "$ROOT"/agent/src/*; do
  for layer in infrastructure handler domain; do
    [ -d "$svc/$layer" ] || continue
    while IFS= read -r f; do
      report agent-prompts-in-service "${f#$ROOT/}" "prompts encode what the Agent asks the model; infrastructure/ollama only talks to the model" \
        "move it to service/prompts/"
    done < <(find "$svc/$layer" \( -iname '*prompt*.py' -o -type d -iname '*prompt*' \) -not -path '*/gen/*' 2>/dev/null)
  done
done

# 8. Where data shapes live in the Agent: business models in domain/model/, adapter-internal wire shapes in
# infrastructure/entity/, protobuf DTO conversion in mapper/ folders (named <model>_mapper.py).
for svc in "$ROOT"/agent/src/*; do
  while IFS= read -r f; do
    case "$f" in */domain/model/*|*/gen/*) ;; *) report agent-model-folder "${f#$ROOT/}" "business data is a domain model, one file per concept" "move the dataclass to domain/model/<concept>.py";; esac
  done < <(grep -rlE '^@dataclass' --include='*.py' "$svc/domain" 2>/dev/null)
  while IFS= read -r f; do
    case "$f" in */infrastructure/entity/*) ;; *) report agent-entity-folder "${f#$ROOT/}" "a technology's wire shapes are infrastructure entities, kept out of the adapter logic" "move the dataclass to infrastructure/entity/<technology>_<shape>.py";; esac
  done < <(grep -rlE '^@dataclass' --include='*.py' "$svc/infrastructure" 2>/dev/null | grep -v '/gen/')
  # protobuf message modules (requests, responses, details) are touched only by mapper/, which converts both ways
  for dir in handler/grpc infrastructure/engine_grpc; do
    [ -d "$svc/$dir" ] || continue
    while IFS= read -r f; do
      case "$f" in
        */mapper/*) ;;
        *) report agent-dto-in-mapper "${f#$ROOT/}" "converting protobuf DTOs to and from domain models is the mappers' job" "move the conversion to $dir/mapper/<model>_mapper.py";;
      esac
    done < <(grep -rlE '^[[:space:]]*from[[:space:]]+[A-Za-z_.]*gen\.fga[A-Za-z_.]*[[:space:]]+import[[:space:]]+[a-z]+_pb2([[:space:]]|$)' --include='*.py' "$svc/$dir" 2>/dev/null)
    for f in "$svc/$dir"/mapper/*.py; do
      [ -f "$f" ] || continue
      n=$(basename "$f"); case "$n" in __init__.py|*_mapper.py) ;; *) report agent-dto-in-mapper "${f#$ROOT/}" "mapper files are named after the model they convert" "rename to <model>_mapper.py";; esac
    done
  done
done

# 9. handler/ is the delivery layer, like Go's handler/http: `<domain>_handler.py` endpoint classes (one per domain, own
# use cases, thin: decode, call one use case, encode through mapper/) plus delivery plumbing (servicer, server, errors, mapper/, and common/<kind>_helper.py for what the handlers share,
# errors, mapper/). Delivery code imports no service, adapter or domain logic (only models, errors and gateway types).
for svc in "$ROOT"/agent/src/*; do
  dir="$svc/handler/grpc"
  [ -d "$dir" ] || continue
  for f in "$dir"/*.py; do
    n=$(basename "$f")
    case "$n" in __init__.py|errors.py|server.py|servicer.py|*_handler.py) ;; *) report agent-handler-shape "${f#$ROOT/}" "an endpoint file is named after its domain, like Go's <domain>_handler.go; the only other files are the delivery plumbing (servicer, server, errors) and common/" "name it <domain>_handler.py";; esac
  done
  for f in "$dir"/common/*.py; do
    [ -f "$f" ] || continue
    n=$(basename "$f"); case "$n" in __init__.py|*_helper.py) ;; *) report agent-handler-shape "${f#$ROOT/}" "helpers shared by the handlers are named <kind>_helper.py" "rename to <kind>_helper.py";; esac
  done
  while IFS= read -r f; do
    report agent-handler-shape "${f#$ROOT/}" "the delivery layer decodes, calls one use case and encodes; business logic belongs in service/" \
      "move the logic to a service and call it through its use case"
  done < <(grep -rlE '^[[:space:]]*from[[:space:]]+\.\.\.domain\.(progress|cancel|call_context)|^[[:space:]]*from[[:space:]]+\.\.\.(service|infrastructure)' --include='*.py' "$dir" 2>/dev/null)
done

# 10. Shape rules that need the syntax tree: thin handlers, small mappers, small functions (scripts/check-python-shape.py).
ROOT="$ROOT" python3 "$(cd "$(dirname "$0")" && pwd)/check-python-shape.py" || fail=1

# 11. Common code stays in the layer that uses it: a handler helper in handler/, a service helper in service/helper/ (its
# interface in usecase/helper/), an infrastructure helper in infrastructure/. Nowhere else (domain has no helpers).
for svc in "$ROOT"/agent/src/*; do
  while IFS= read -r f; do
    rel=${f#$svc/}
    case "$rel" in
      handler/*/common/*_helper.py|usecase/helper/*.py|service/helper/*.py|infrastructure/*) ;;
      *) report agent-helper-in-layer "${f#$ROOT/}" "common code stays in the layer that uses it (handler/*/common, usecase/helper + service/helper, infrastructure)" \
           "move it into the layer that calls it";;
    esac
  done < <(find "$svc" -type f -name '*.py' \( -name '*helper*' -o -path '*/common/*' -o -name '*util*' \) -not -path '*/gen/*' -not -name '__init__.py' 2>/dev/null)
done
# usecase/helper/<x>.py has its implementation in service/helper/<x>.py, like usecase/<x>.py has service/<x>.py.
for svc in "$ROOT"/agent/src/*; do
  for f in "$svc"/usecase/helper/*.py; do
    [ -f "$f" ] || continue
    n=$(basename "$f"); [ "$n" = "__init__.py" ] && continue
    [ -f "$svc/service/helper/$n" ] || report usecase-service-pair "${f#$ROOT/}" "a shared business rule is an interface in usecase/helper/ and its implementation in service/helper/, same file name" "add service/helper/$n"
  done
done

# 12. The layer layout of the Agent, as in the Go API: domain{model,gateway}, usecase{helper}, service{helper,prompts},
# infrastructure/<technology>, handler/<transport>; the package root holds only the composition root and the entrypoint.
layout() { # dir allowed-entry...   (every entry of dir must be one of the names; __pycache__ and gen are ignored)
  local dir=$1; shift
  [ -d "$dir" ] || return 0
  local e n ok a
  for e in "$dir"/*; do
    n=$(basename "$e"); [ "$n" = "__pycache__" ] && continue
    ok=0; for a in "$@"; do [ "$n" = "$a" ] && ok=1; done
    [ $ok = 1 ] || report agent-layer-layout "${e#$ROOT/}" "the layers have a fixed layout: ${dir#$ROOT/} holds only: $*" "put it in the layer that owns it (domain/model, domain/gateway, usecase, service, infrastructure/<technology>, handler/<transport>)"
  done
}
for svc in "$ROOT"/agent/src/*; do
  [ -d "$svc" ] || continue
  layout "$svc" __init__.py py.typed wire.py serve.py gen domain usecase service infrastructure handler
  layout "$svc/domain" __init__.py errors.py model gateway
  layout "$svc/handler" __init__.py grpc http
  # infrastructure holds technology folders only (entity holds their wire shapes)
  if [ -d "$svc/infrastructure" ]; then
    for e in "$svc/infrastructure"/*; do
      n=$(basename "$e"); [ "$n" = "__pycache__" ] && continue
      [ -d "$e" ] || [ "$n" = "__init__.py" ] || report agent-layer-layout "${e#$ROOT/}" "infrastructure holds one folder per technology (ollama, engine_grpc, entity), no loose modules" "move it into the folder of the technology it adapts"
    done
  fi
  # usecase: one interface file per operation, plus helper/ for shared business rules
  if [ -d "$svc/usecase" ]; then
    for e in "$svc/usecase"/*; do
      n=$(basename "$e"); [ "$n" = "__pycache__" ] && continue
      [ -f "$e" ] || [ "$n" = helper ] || report agent-layer-layout "${e#$ROOT/}" "usecase holds one interface file per operation, plus helper/" "move it"
    done
  fi
  # service: its use case implementations (files), plus helper/ and prompts/
  if [ -d "$svc/service" ]; then
    for e in "$svc/service"/*; do
      n=$(basename "$e"); [ "$n" = "__pycache__" ] && continue
      [ -f "$e" ] || [ "$n" = helper ] || [ "$n" = prompts ] || report agent-layer-layout "${e#$ROOT/}" "service holds use case implementations, helper/ and prompts/" "move it"
    done
  fi
done

[ $fail = 0 ] && echo "python architecture: ok"
exit $fail
