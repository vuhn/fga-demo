#!/usr/bin/env bash
# Generate the shared gRPC stubs from proto/ into proto/go and proto/python. Services import them; no service
# holds generated code. Go stubs by buf (local plugins), Python stubs by grpcio-tools.
#   scripts/gen-proto.sh           write the stubs into proto/go and proto/python
#   scripts/gen-proto.sh --check   generate into a temp dir and fail if the committed stubs differ (drift)
# Pinned generators: protoc-gen-go v1.36.12, protoc-gen-go-grpc 1.5.1, grpcio-tools from proto/python/uv.lock.
set -eu
cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/go/bin"
MODE=${1:-write}
WANT_GO=v1.36.12; WANT_GRPC=1.5.1

[ "$(protoc-gen-go --version | awk '{print $2}')" = "$WANT_GO" ] || { echo "protoc-gen-go $WANT_GO required: GOBIN=\$HOME/go/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@$WANT_GO" >&2; exit 1; }
[ "$(protoc-gen-go-grpc --version | awk '{print $2}')" = "$WANT_GRPC" ] || { echo "protoc-gen-go-grpc $WANT_GRPC required: GOBIN=\$HOME/go/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$WANT_GRPC" >&2; exit 1; }

gen_into() { # root: directory that receives go/ and python/fga/ (the committed targets are proto/go and proto/python/fga)
  local root=$1
  mkdir -p "$root/go" "$root/python/fga" proto/python/fga; [ -f proto/python/fga/__init__.py ] || : > proto/python/fga/__init__.py  # uv builds the package to run grpcio-tools
  local tpl; tpl=$(mktemp -d)/buf.gen.yaml
  sed "s#out: go#out: $root/go#" proto/buf.gen.yaml >"$tpl"
  (cd proto && buf generate --template "$tpl"); rm -rf "$(dirname "$tpl")"
  # protoc writes `from fga.demo.common.v1 import ...`, which is exactly how services import the package.
  uv run --quiet --project proto/python python -m grpc_tools.protoc -I proto \
    --python_out="$root/python" --grpc_python_out="$root/python" $(cd proto && find fga -name '*.proto' | sort)
  find "$root/python/fga" -type d -exec sh -c 'for d; do [ -f "$d/__init__.py" ] || : > "$d/__init__.py"; done' _ {} +
}

if [ "$MODE" = "--check" ]; then
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  gen_into "$tmp"
  bad=0
  diff -r -x go.mod -x go.sum "$tmp/go" proto/go >/dev/null 2>&1 || { echo "stub drift in proto/go: run scripts/gen-proto.sh and commit" >&2; bad=1; }
  diff -r -x __pycache__ -x pyproject.toml -x uv.lock -x .venv -x '*.egg-info' "$tmp/python" proto/python >/dev/null 2>&1 || { echo "stub drift in proto/python: run scripts/gen-proto.sh and commit" >&2; bad=1; }
  exit $bad
fi
gen_into "$PWD/proto"
