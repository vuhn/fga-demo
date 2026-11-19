#!/usr/bin/env bash
# Generate gRPC stubs from proto/. Go stubs by buf (local plugins), Python stubs by grpcio-tools.
#   scripts/gen-proto.sh           write the stubs into the tree
#   scripts/gen-proto.sh --check   generate into a temp dir and fail if the committed stubs differ (drift)
# Pinned generators: protoc-gen-go v1.36.12, protoc-gen-go-grpc 1.5.1, grpcio-tools from the engine's uv.lock.
set -eu
cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/go/bin"
MODE=${1:-write}
WANT_GO=v1.36.12; WANT_GRPC=1.5.1

[ "$(protoc-gen-go --version | awk '{print $2}')" = "$WANT_GO" ] || { echo "protoc-gen-go $WANT_GO required: GOBIN=\$HOME/go/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@$WANT_GO" >&2; exit 1; }
[ "$(protoc-gen-go-grpc --version | awk '{print $2}')" = "$WANT_GRPC" ] || { echo "protoc-gen-go-grpc $WANT_GRPC required: GOBIN=\$HOME/go/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$WANT_GRPC" >&2; exit 1; }

# Python packages that get stubs: the engine always, the agent once it exists.
PY=()
for pkg in blueprint-engine/src/fga_engine agent/src/fga_agent; do
  [ -d "$pkg" ] && PY+=("$pkg")
done

gen_into() { # root: where the repo-relative outputs are written
  local root=$1
  mkdir -p "$root/api/internal/infrastructure/grpcgen"
  local tpl; tpl=$(mktemp -d)/buf.gen.yaml
  sed "s#../api/internal/infrastructure/grpcgen#$root/api/internal/infrastructure/grpcgen#" proto/buf.gen.yaml >"$tpl"
  (cd proto && buf generate --template "$tpl"); rm -rf "$(dirname "$tpl")"
  for pkg in "${PY[@]}"; do
    local out="$root/$pkg/gen" mod; mod=$(basename "$pkg")
    mkdir -p "$out"
    (cd blueprint-engine && uv run --quiet python -m grpc_tools.protoc -I ../proto --python_out="$out" --grpc_python_out="$out" \
      $(cd ../proto && find fga -name '*.proto'))
    # protoc writes `from fga.demo...`; make the imports package-relative to the stub root.
    find "$out" -name '*.py' -exec sed -i.bak -E "s/^from fga\./from $mod.gen.fga./; s/^import fga\./import $mod.gen.fga./" {} + && find "$out" -name '*.bak' -delete
    find "$out" -type d -exec sh -c 'for d; do [ -f "$d/__init__.py" ] || : > "$d/__init__.py"; done' _ {} +
  done
}

if [ "$MODE" = "--check" ]; then
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  gen_into "$tmp"
  bad=0
  for d in api/internal/infrastructure/grpcgen "${PY[@]/%//gen}"; do
    diff -r -x __pycache__ "$tmp/$d" "$d" >/dev/null 2>&1 || { echo "stub drift in $d: run scripts/gen-proto.sh and commit" >&2; bad=1; }
  done
  exit $bad
fi
gen_into "$PWD"
