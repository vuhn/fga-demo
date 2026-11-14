compose-up:
	docker compose -f deploy/docker-compose.yml up -d

seed:
	cd api && go run ./cmd/seed

api:
	cd api && go run ./cmd/api

web:
	cd web && npm install && npm run dev

test:
	cd api && go test ./...

test-integration:
	cd api && OPENFGA_API_URL=http://localhost:8080 go test -tags=integration ./internal/infrastructure/fga/

check:
	cd api && go vet ./... && go vet -tags=integration ./... && go test ./...
	bash scripts/check-architecture.sh
	cd web && npm run build

verify-live: compose-up seed test-integration

init:
	bash scripts/init.sh

.PHONY: compose-up seed api web test test-integration check verify-live init
