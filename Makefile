.PHONY: build install clean test lint fmt fmt-check check run release

# Binary name for this service
BINARY_NAME = cometbft-analyzer-backend
GOBIN := $(shell go env GOPATH)/bin

## Core
build:
	go build -o $(BINARY_NAME) .

install: build
	cp $(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)

test:
	go test ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout=5m ./...

fmt:
	go fmt ./...
	gofmt -s -w .

fmt-check:
	@out=$$(gofmt -l -s .); \
	if [ -n "$$out" ]; then \
		echo "Files not formatted:"; echo "$$out"; exit 1; \
	fi

check: fmt-check lint test

# Run the backend with optional env overrides
# Usage: make run [PORT=8080] [MONGODB_URI=mongodb://localhost:27017]
run:
	@PORT=$${PORT:-8080}; \
	MONGODB_URI=$${MONGODB_URI:-mongodb://localhost:27017}; \
	PORT=$$PORT MONGODB_URI=$$MONGODB_URI go run .

## Release tagging (mirrors cometbft-log-etl style)
VERSION ?=
TAG_PREFIX ?=
TAG := $(if $(TAG_PREFIX),$(TAG_PREFIX)$(VERSION),$(VERSION))

release:
	@test -n "$(VERSION)" || (echo "VERSION required, e.g.: make release VERSION=v0.1.0" && exit 1)
	@git diff --quiet || (echo "Working tree not clean; commit or stash changes first" && exit 1)
	@if git rev-parse -q --verify "refs/tags/$(TAG)" >/dev/null; then \
		echo "Tag '$(TAG)' already exists"; \
		exit 1; \
	else \
		git tag -a "$(TAG)" -m "Release $(TAG)"; \
		git push origin "$(TAG)"; \
		echo "Tagged and pushed $(TAG)"; \
	fi


