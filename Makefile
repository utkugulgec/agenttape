.PHONY: dev dev-all test build migrate-up migrate-down docker-build clean

DATABASE_URL ?= postgres://agenttape:agenttape@localhost:5433/agenttape?sslmode=disable
BINARY := ./bin/server

dev:
	docker compose up -d postgres adminer
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/server

dev-all:
	docker compose up -d postgres adminer
	@trap 'kill 0' SIGINT SIGTERM; \
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/server & \
	(cd frontend && npm run dev) & \
	wait

test:
	go test ./... -race

build:
	mkdir -p ./bin
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

migrate-up:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DATABASE_URL)" down

docker-build:
	docker build -t agenttape-server .
	docker build -t agenttape-frontend ./frontend

clean:
	rm -rf ./bin
	docker compose down
