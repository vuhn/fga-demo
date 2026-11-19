#!/usr/bin/env bash
# Executable architecture rules from CLAUDE.md. Each failure prints WHAT / WHY / FIX.
set -u
cd "$(dirname "$0")/.."

fail=0
violation() { # what why fix
  printf '\nFAIL: %s\n  why: %s\n  fix: %s\n' "$1" "$2" "$3"
  fail=1
}

# 1. Inner layers never import transport/storage SDKs.
for layer in domain usecase service; do
  hits=$(grep -rlE 'gin-gonic|jackc/pgx|openfga/go-sdk|google\.golang\.org/(grpc|protobuf)|infrastructure/grpc(gen|util)' --include='*.go' "api/internal/$layer" 2>/dev/null)
  if [ -n "$hits" ]; then
    violation "api/internal/$layer imports Gin, pgx, the OpenFGA SDK, gRPC, protobuf or the generated stubs:
$hits" \
      "domain, usecase and service depend only on interfaces so they stay testable and backend-agnostic" \
      "move the SDK call behind a gateway interface in domain/gateway and implement it under infrastructure/"
  fi
done

# 2. Tuple formatting lives only in infrastructure/fga.
hits=$(grep -rnE '"(user|folder|document|album|photo|blueprint|tenant|role|product|platform|system)[:#]' --include='*.go' api/internal api/cmd \
  | grep -v 'api/internal/infrastructure/fga/' | grep -v '_test\.go' | grep -v 'api/internal/seed/' \
  | grep -vE '^\S+:[0-9]+:\s*//' | grep -v 'domain/model/authz.go')
if [ -n "$hits" ]; then
  violation "tuple or object-id literal outside infrastructure/fga:
$hits" \
  "services pass model.Tuple through; only the *Relations builders format 'type:id#relation' strings" \
  "add or reuse a builder in api/internal/infrastructure/fga/*_relations.go and call it through the relation gateway"
fi

# 3. Never permit anything by branching on role/affiliation strings.
allow='member_helper.go|member_role_changer.go|member_assignment.go'
hits=$(grep -rnE '\.(Role|Affiliation) *(==|!=)|case +model\.(Role|Affiliation)' --include='*.go' api/internal/service api/internal/usecase api/internal/handler \
  | grep -v '_test\.go' | grep -vE "$allow")
if [ -n "$hits" ]; then
  violation "branching on memberships.role/affiliation:
$hits" \
  "role and affiliation are display labels; OpenFGA Check is the only allow/deny decision" \
  "call usecase.Authorizer (Check/PlatformOr) with the decision-catalog relation instead; if this is a stored invariant, add the file to the allowlist in scripts/check-architecture.sh"
fi

# 4. Model stays allow-only.
if grep -nE '\bbut not\b|\bcondition\b' api/fga/model.fga >/dev/null; then
  violation "api/fga/model.fga uses 'but not' or 'condition'" \
  "the model is allow-only; 'private wins' is expressed as intersection" \
  "rewrite with 'and' (intersection) or an explicit tuple; see docs/authorization-model.md"
fi

# 5. model.fga equals the DSL block in docs/authorization-model.md.
if ! diff <(awk '/^```fga$/{f=1;next} /^```$/{f=0} f' docs/authorization-model.md) api/fga/model.fga >/dev/null; then
  violation "api/fga/model.fga differs from the fga block in docs/authorization-model.md" \
  "the spec and the model are one source of truth; the docs block must equal the file exactly" \
  "update docs/authorization-model.md first, then copy the block into api/fga/model.fga (diff <(awk '/^\`\`\`fga\$/{f=1;next} /^\`\`\`\$/{f=0} f' docs/authorization-model.md) api/fga/model.fga)"
fi

# 6. Web components never name OpenFGA relations.
hits=$(grep -rnE "[\"'\`]can_[a-z_]+" web/src --include='*.ts' --include='*.tsx' \
  | grep -vE 'web/src/(permissions\.tsx|pages/Inspector\.tsx)')
if [ -n "$hits" ]; then
  violation "OpenFGA relation named outside web/src/permissions.tsx:
$hits" \
  "permissions.tsx is the only mapping from relations to UI (FEATURES / ACTIONS)" \
  "add a rule to FEATURES or ACTIONS and use useCan / useAllows / <Can> / canAct in the component"
fi

# 7. Go layer layout (the same words as the Python services): business data lives in domain/model (never domain/entity),
# and a handler's DTOs and view conversions live in the domain's <domain>_mapper.go, not in the handler file.
if [ -d api/internal/domain/entity ]; then
  violation "api/internal/domain/entity exists" \
    "business data is domain/model in every service; 'entity' only names a technology's own row or wire shape" \
    "git mv api/internal/domain/entity api/internal/domain/model"
fi
hits=$(grep -nE '^(func [a-z][A-Za-z]*(View|Views|Summary)\(|type [a-z][A-Za-z]*JSON )' api/internal/handler/http/*_handler.go api/internal/handler/http/middleware.go 2>/dev/null)
if [ -n "$hits" ]; then
  violation "DTO or view conversion inside a handler file:
$hits" \
    "a handler decodes, calls one use case and encodes; the JSON shapes and the model <-> DTO conversions live in <domain>_mapper.go (one per domain, like the Python mapper/ folders)" \
    "move it to api/internal/handler/http/<domain>_mapper.go"
fi

# 8. Layer rules of the Python services (their own script; known violations are ratcheted down).
bash scripts/check-architecture-python.sh || fail=1

if [ "$fail" -ne 0 ]; then
  printf '\narchitecture check failed\n'
  exit 1
fi
echo "architecture check ok"
