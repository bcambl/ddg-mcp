 
GIT_HASH := $(shell git rev-parse --short=6 HEAD 2>/dev/null || echo "000000")
GIT_DIRTY := $(shell git diff --quiet HEAD 2>/dev/null && git diff --cached --quiet HEAD 2>/dev/null || echo "-dirty")
VERSION := $(GIT_HASH)$(GIT_DIRTY)
DOCKER_IMAGE := ghcr.io/bcambl/ddg-mcp

.PHONY: help
help:
	@echo "ddg-mcp - DuckDuckGo Web Search MCP Server"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Build:"
	@echo "  all              Format, test, and build binary"
	@echo "  build            Build the ddg-mcp binary"
	@echo "  docker           Build Docker image"
	@echo "  clean            Remove built artifacts (bin/)"
	@echo ""
	@echo "Development:"
	@echo "  fmt              Format code using gofmt"
	@echo "  test             Run all tests"
	@echo "  integration      Run integration tests (hits real DDG)"
	@echo "  bench            Run benchmark tests"
	@echo "  changelog        Generate CHANGELOG.md from conventional commits"
	@echo "  vet              Vet code for potential issues"
	@echo "  deps             Tidy module dependencies"
	@echo "  check            Run all checks (fmt + vet + test)"

.PHONY: all
all: fmt test build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	go test -v -race ./...

.PHONY: integration
integration:
	go test -v -tags=integration ./...

.PHONY: bench
bench:
	go test -bench=. -benchmem ./...

.PHONY: changelog
changelog:
	git-cliff -o CHANGELOG.md

.PHONY: vet
vet:
	go vet ./...

.PHONY: build
build: clean
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/ddg-mcp .

.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE) .

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: deps
deps:
	go mod tidy

.PHONY: check
check: fmt vet test