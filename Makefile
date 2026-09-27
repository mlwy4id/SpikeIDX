.PHONY: test vet build fmt lint run-api

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
