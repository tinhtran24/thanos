.PHONY: build test lint check

build:
	go build -o bin/thanos ./cmd/thanos

test:
	go test ./...

lint:
	go vet ./...

check: build test lint
