compose-up:
	docker compose -f deploy/docker-compose.yml up -d

seed:
	cd api && go run ./cmd/seed

api:
	cd api && go run ./cmd/api

web:
	cd web && npm install && npm run dev

engine:
	cd blueprint-engine && uv run uvicorn blueprint_engine.app:app --port 8090 --reload

engine-test:
	cd blueprint-engine && uv run pytest -q

engine-eval:
	cd blueprint-engine && uv run python eval/run.py

engine-eval-estimate:
	cd blueprint-engine && uv run python eval/estimate_run.py

test:
	cd api && go test ./...

test-integration:
	cd api && OPENFGA_API_URL=http://localhost:8080 go test -tags=integration ./internal/infrastructure/fga/

check:
	cd api && go vet ./... && go vet -tags=integration ./... && go test ./...
	bash scripts/check-architecture.sh
	bash scripts/check-migrations.sh
	bash scripts/check-specs.sh
	cd blueprint-engine && uv run pytest -q
	cd web && npm run build

verify-live: compose-up seed test-integration

init:
	bash scripts/init.sh

.PHONY: compose-up seed api web engine engine-test engine-eval engine-eval-estimate test test-integration check verify-live init
