SQLC_VERSION := 1.31.1
GOOSE_VERSION := 3.27.3

.PHONY: run worker build test test-race fmt vet tidy tools sqlc migrate-up migrate-down migrate-status migrate-create swagger docker-up docker-down

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v$(SQLC_VERSION)
	go install github.com/pressly/goose/v3/cmd/goose@v$(GOOSE_VERSION)

sqlc:
	sqlc generate

migrate-up:
	goose -dir database/migrations mysql "$(DB_DSN)" up

migrate-down:
	goose -dir database/migrations mysql "$(DB_DSN)" down

migrate-status:
	goose -dir database/migrations mysql "$(DB_DSN)" status

migrate-create:
	goose -dir database/migrations create "$(name)" sql

swagger:
	@echo "Swagger UI: /swagger/index.html"

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down
