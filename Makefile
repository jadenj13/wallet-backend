.PHONY: build build-api build-tss run-api run-tss test lint tidy docker-up docker-down docker-build

# Build both binaries
build: build-api build-tss

build-api:
	go build -o bin/api ./cmd/api

build-tss:
	go build -o bin/tss ./cmd/tss

# Run services locally
run-api:
	go run ./cmd/api

run-tss:
	go run ./cmd/tss

# Test & lint
test:
	go test ./...

lint:
	golangci-lint run ./...

# Dependency management
tidy:
	go mod tidy

# Docker
docker-up:
	docker compose up

docker-down:
	docker compose down

docker-build:
	docker compose up --build
