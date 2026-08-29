.PHONY: build test lint install clean

build:
	go build -o bin/e-boekhouden-pp-cli ./cmd/e-boekhouden-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/e-boekhouden-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/e-boekhouden-pp-mcp ./cmd/e-boekhouden-pp-mcp

install-mcp:
	go install ./cmd/e-boekhouden-pp-mcp

build-all: build build-mcp
