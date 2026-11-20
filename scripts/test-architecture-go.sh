#!/usr/bin/env bash
# Plants one violation per Go shape rule in a temp tree and asserts scripts/check-go-shape.go flags each one
# (and passes a clean tree).
set -eu
cd "$(dirname "$0")/.."
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
a="$t/api/internal"
mkdir -p "$a"/usecase/{input,output,helper} "$a"/domain/{model,gateway} "$a"/service

good() {
  cat >"$a/usecase/folder_creator.go" <<'G'
package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type FolderCreator interface {
	Create(ctx context.Context, in input.FolderCreator) (output.FolderCreator, error)
}

type FolderDeleter interface {
	Delete(ctx context.Context, in input.FolderDeleter) error
}

type FolderStreamer interface {
	Stream(ctx context.Context, in input.FolderStreamer, onProgress func(string)) (output.FolderStreamer, error)
}
G
  printf 'package input\n\ntype FolderCreator struct{ Actor string }\ntype FolderDeleter struct{ ID string }\ntype FolderStreamer struct{ ID string }\n' >"$a/usecase/input/folder.go"
  printf 'package output\n\ntype FolderCreator struct{ ID string }\ntype FolderStreamer struct{ ID string }\n' >"$a/usecase/output/folder.go"
  printf 'package gateway\n\nimport "context"\n\ntype FolderGateway interface {\n\tGet(ctx context.Context, id string) error\n\tList(ctx context.Context, a, b, c string) error\n}\n' >"$a/domain/gateway/folder_gateway.go"
  printf 'package service\n\nimport (\n\t"context"\n\t"fmt"\n\n\t"github.com/vuhn/fga-demo/api/internal/usecase"\n)\n\nvar _ = fmt.Sprint\nvar _ usecase.FolderCreator\nvar _ context.Context\n' >"$a/service/folder_creator.go"
}

# expect <label> <pattern>: the check must fail and print the pattern
expect() {
  if out=$(go run scripts/check-go-shape.go "$t" 2>&1); then echo "FAIL: $1 was not flagged"; exit 1; fi
  if ! grep -q "$2" <<<"$out"; then echo "FAIL: $1 flagged without '$2':"; echo "$out"; exit 1; fi
  echo "ok: $1"
}

good
go run scripts/check-go-shape.go "$t" >/dev/null || { echo "FAIL: the clean tree was flagged"; exit 1; }
echo "ok: clean tree"

# a. operation shape
good; cat >>"$a/usecase/folder_creator.go" <<'G'

type FolderRenamer interface {
	Rename(ctx context.Context, actor, id string) (output.FolderCreator, error)
}
G
expect "extra params on a usecase method" "FolderRenamer.Rename does not have the operation shape"

good; cat >>"$a/usecase/folder_creator.go" <<'G'

type FolderMissing interface {
	Make(ctx context.Context, in input.FolderMissing) error
}
G
expect "missing input struct" "input.FolderMissing is not declared"

good; cat >>"$a/usecase/folder_creator.go" <<'G'

type FolderOut interface {
	Make(ctx context.Context, in input.FolderOut) (output.FolderOut, error)
}
G
printf 'type FolderOut struct{}\n' >>"$a/usecase/input/folder.go"
expect "missing output struct" "output.FolderOut is not declared"

good; cat >>"$a/usecase/folder_creator.go" <<'G'

type FolderModel interface {
	Make(ctx context.Context, in input.FolderCreator) (model.Folder, error)
}
G
expect "a model result" "FolderModel.Make does not have the operation shape"

# b. output fields are copied, never model types
good; printf 'package output\n\nimport "github.com/vuhn/fga-demo/api/internal/domain/model"\n\ntype Bad struct{ F model.Folder }\n' >"$a/usecase/output/bad.go"
expect "a model field in an output" "Bad uses a domain/model type"

# c. gateway params
good; printf 'package gateway\n\nimport "context"\n\ntype WideGateway interface {\n\tUpdate(ctx context.Context, a, b, c, d string) error\n\tAssign(a, b, c, d string) error\n}\n' >"$a/domain/gateway/wide_gateway.go"
expect "4 gateway params after ctx" "WideGateway.Update has 4 params after ctx"
expect "4 gateway params without ctx" "WideGateway.Assign has 4 params"

# d. business layer imports
for imp in github.com/gin-gonic/gin github.com/vuhn/fga-demo/api/internal/infrastructure/postgres github.com/vuhn/fga-demo/api/internal/handler/http; do
  good; printf 'package service\n\nimport _ "%s"\n' "$imp" >"$a/service/bad.go"
  expect "service imports $imp" "service/bad.go imports $imp"
done
good; printf 'package usecase\n\nimport _ "github.com/google/uuid"\n' >"$a/usecase/bad.go"
expect "usecase imports a third-party package" "usecase/bad.go imports github.com/google/uuid"
echo "go architecture tests ok"
