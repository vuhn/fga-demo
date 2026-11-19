compose-up:
	docker compose -f deploy/docker-compose.yml up -d

seed:
	cd api && go run ./cmd/seed

api:
	cd api && go run ./cmd/api

web:
	cd web && npm install && npm run dev

engine:
	cd blueprint-engine && uv run watchfiles --filter python fga_engine.serve.main src

agent:
	cd agent && uv run watchfiles --filter python fga_agent.serve.main src

agent-test:
	cd agent && uv run pytest -q

engine-test:
	cd blueprint-engine && uv run pytest -q

agent-eval:
	cd agent && uv run python eval/run.py

agent-eval-estimate:
	cd agent && uv run python eval/estimate_run.py

test:
	cd api && go test ./...

test-integration:
	cd api && OPENFGA_API_URL=http://localhost:8080 go test -tags=integration ./internal/infrastructure/fga/

proto:
	cd proto && buf lint
	@if git cat-file -e main:proto/buf.yaml 2>/dev/null; then cd proto && buf breaking --against '../.git#branch=main,subdir=proto'; else echo "proto: no baseline on main yet, skipping buf breaking"; fi
	python3 scripts/check-proto-identity.py proto
	@python3 scripts/check-proto-identity.py scripts/testdata/proto-identity-bad >/dev/null 2>&1 && { echo "proto identity check did not flag its planted violation"; exit 1; } || true
	bash scripts/gen-proto.sh --check

proto-gen:
	bash scripts/gen-proto.sh

check:
	$(MAKE) proto
	cd api && go vet ./... && go vet -tags=integration ./... && go test ./...
	bash scripts/test-architecture-python.sh
	bash scripts/test-git-guard.sh
	bash scripts/check-architecture.sh
	bash scripts/check-migrations.sh
	bash scripts/check-specs.sh
	cd blueprint-engine && uv run pytest -q
	cd agent && uv run pytest -q
	cd web && npm run build

verify-live: compose-up seed test-integration

init:
	bash scripts/init.sh

.PHONY: proto proto-gen compose-up seed api web engine agent agent-test engine-test agent-eval agent-eval-estimate test test-integration check verify-live init
