compose-up:
	docker compose -f deploy/docker-compose.yml up -d

seed:
	go run ./cmd/seed

api:
	go run ./cmd/api

web:
	cd web && npm install && npm run dev

test:
	go test ./...

test-integration:
	OPENFGA_API_URL=http://localhost:8080 go test -tags=integration ./internal/adapter/openfga/

.PHONY: compose-up seed api web test test-integration
