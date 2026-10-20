.PHONY: setup run build fmt check test test-integration audit up down

setup:
	sh scripts/setup-env.sh

run:
	set -a; . ./.env; set +a; go run ./cmd/server

build:
	go build -trimpath -o bin/noteflow ./cmd/server

fmt:
	gofmt -w cmd internal

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race ./...
	go build ./...

test:
	go test -race ./...

test-integration:
	sh scripts/test-integration.sh

audit:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

up:
	docker compose up --build -d --wait

down:
	docker compose down
