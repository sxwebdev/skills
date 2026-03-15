.PHONY: build install test lint clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/skills ./cmd/skills

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/skills

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
