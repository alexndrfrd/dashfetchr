.PHONY: help tidy build test test-contract test-contract-all lint run-api run-dispatcher run-webhook migrate migrate-down docker-up docker-down clean

help:
	@echo "DashFetchr Makefile"
	@echo ""
	@echo "Dev workflow:"
	@echo "  make tidy             - go mod tidy"
	@echo "  make build            - build all binaries"
	@echo "  make test             - run all tests"
	@echo "  make test-contract CARRIER=bolt  - run contract tests for one carrier"
	@echo "  make test-contract-all - run contract tests for every carrier"
	@echo "  make lint             - golangci-lint + go-arch-lint"
	@echo "  make docker-up        - start local Postgres + Redis + LocalStack S3"
	@echo "  make docker-down      - stop local services"
	@echo "  make migrate          - apply migrations"
	@echo "  make migrate-down     - revert migrations"
	@echo "  make run-api          - run the API locally"
	@echo "  make run-dispatcher   - run the dispatcher locally"
	@echo "  make run-webhook      - run the webhook listener locally"
	@echo ""

GO          ?= go
DB_URL      ?= postgres://dashfetchr:dashfetchr@localhost:5432/dashfetchr?sslmode=disable
MIGRATE     ?= migrate

tidy:
	$(GO) mod tidy

build:
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/dispatcher ./cmd/dispatcher
	$(GO) build -o bin/webhook-listener ./cmd/webhook-listener

test:
	$(GO) test ./...

# Run contract tests for a specific carrier.
# Usage: make test-contract CARRIER=bolt
test-contract:
	@if [ -z "$(CARRIER)" ]; then echo "Usage: make test-contract CARRIER=bolt"; exit 1; fi
	$(GO) test ./internal/adapters/carriers/$(CARRIER)/... -run Contract -v

test-contract-all:
	$(GO) test ./internal/adapters/carriers/... -run Contract -v

lint:
	golangci-lint run ./...
	@command -v go-arch-lint >/dev/null && go-arch-lint check || echo "go-arch-lint not installed; skipping"

run-api:
	$(GO) run ./cmd/api

run-dispatcher:
	$(GO) run ./cmd/dispatcher

run-webhook:
	$(GO) run ./cmd/webhook-listener

migrate:
	$(MIGRATE) -path migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DB_URL)" down 1

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin
