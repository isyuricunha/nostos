SHELL := /usr/bin/env bash

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
	go build ./cmd/app

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
	docker build -t nostos:latest .

docker-up:
	docker compose -f compose.yaml up -d

docker-up-local-db:
	docker compose -f compose.yaml -f compose.local-db.yaml up -d

docker-up-sqlite:
	docker compose -f compose.yaml -f compose.sqlite.yaml up -d

docker-down:
	docker compose -f compose.yaml -f compose.local-db.yaml -f compose.sqlite.yaml down

doctor:
	go run ./cmd/app doctor
