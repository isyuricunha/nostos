SHELL := /usr/bin/env bash
VERSION ?= 0.1.0
BUILD_COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo development)
BUILD_TIMESTAMP ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_LDFLAGS := -X main.version=$(VERSION) -X main.buildCommit=$(BUILD_COMMIT) -X main.buildTimestamp=$(BUILD_TIMESTAMP)

.PHONY: dev dev-web dev-server dev-worker build test test-go test-web test-integration lint format migrate docker-build docker-up docker-up-local-db docker-up-sqlite docker-down doctor

dev:
	$(MAKE) -j2 dev-server dev-web

dev-web:
	pnpm --dir web dev

dev-server:
	APP_ENV=development DATABASE_DRIVER=sqlite DATABASE_URL=data/dev.db go run ./cmd/app server

dev-worker:
	APP_ENV=development DATABASE_DRIVER=sqlite DATABASE_URL=data/dev.db go run ./cmd/app worker

build:
	pnpm --dir web build
	go build -ldflags "$(GO_LDFLAGS)" ./cmd/app

test: test-go test-web

test-go:
	go test ./...

test-web:
	pnpm --dir web check
	pnpm --dir web test

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...
	pnpm --dir web lint

format:
	gofmt -w cmd internal
	pnpm --dir web format

migrate:
	go run ./cmd/app migrate

docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_COMMIT=$(BUILD_COMMIT) \
		--build-arg BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) \
		-t nostos:$(VERSION) \
		-t nostos:latest .

docker-up:
	BUILD_COMMIT=$(BUILD_COMMIT) BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) docker compose -f compose.yaml up -d --build

docker-up-local-db:
	BUILD_COMMIT=$(BUILD_COMMIT) BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) docker compose -f compose.yaml -f compose.local-db.yaml up -d --build

docker-up-sqlite:
	BUILD_COMMIT=$(BUILD_COMMIT) BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) docker compose -f compose.yaml -f compose.sqlite.yaml up -d --build

docker-down:
	docker compose -f compose.yaml -f compose.local-db.yaml -f compose.sqlite.yaml down

doctor:
	go run -ldflags "$(GO_LDFLAGS)" ./cmd/app doctor
