.PHONY: test vet build fmt lint run-api py-sync py-test py-lint py-notebook

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./...

fmt:
	gofmt -l .
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

run-api:
	go run ./cmd/api

py-sync:
	cd research && uv sync

py-test:
	cd research && uv run pytest

py-lint:
	cd research && uv run ruff check . && uv run ruff format --check .

py-notebook:
	cd research && uv run jupyter lab notebooks/
