GIT_HASH := $(shell git rev-parse --short=6 HEAD 2>/dev/null || echo "000000")
GIT_DIRTY := $(shell git diff --quiet HEAD 2>/dev/null && git diff --cached --quiet HEAD 2>/dev/null || echo "-dirty")
VERSION := $(GIT_HASH)$(GIT_DIRTY)
DOCKER_IMAGE := ghcr.io/bcambl/ddg-mcp
LATEST_TAG := $(shell git tag --list 'v*' --sort=-version:refname | head -n1)
RELEASE_VERSION := $(shell [ -n "$(LATEST_TAG)" ] && echo "$(LATEST_TAG)" | awk -F'[v.]' '{print "v"$$2"."$$3"."$$4+1}' || echo "v0.1.0")

.PHONY: help
help:
	@echo "ddg-mcp - DuckDuckGo Web Search MCP Server"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z0-9_-]+:/ { \
		sub(/:$$/, "", $$1); \
		sub(/:.*$$/, "", $$1); \
		desc = ""; \
		for (i = 2; i <= NF; i++) { \
			if ($$i == "##") { \
				for (j = i+1; j <= NF; j++) desc = desc (desc ? " " : "") $$j; \
				break; \
			} \
		} \
		if (desc) printf "  %-18s %s\n", $$1, desc; \
	}' $(MAKEFILE_LIST)

.PHONY: all
all: fmt test build ## Format, test, and build binary

.PHONY: fmt
fmt: ## Format code using gofmt
	go fmt ./...

.PHONY: test
test: ## Run all tests
	go test -v -race ./...

.PHONY: integration
integration: ## Run integration tests (hits real DDG)
	go test -v -tags=integration ./...

.PHONY: bench
bench: ## Run benchmark tests
	go test -bench=. -benchmem ./...

.PHONY: changelog
changelog: ## Generate CHANGELOG.md from conventional commits
	git-cliff -o CHANGELOG.md

.PHONY: vet
vet: ## Vet code for potential issues
	go vet ./...

.PHONY: build
build: clean ## Build the ddg-mcp binary
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/ddg-mcp .

.PHONY: docker
docker: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

.PHONY: clean
clean: ## Remove built artifacts (bin/)
	rm -rf bin/

.PHONY: deps
deps: ## Tidy module dependencies
	go mod tidy

.PHONY: check
check: fmt vet test ## Run all checks (fmt + vet + test)

.PHONY: release-check
release-check: ## Verify working tree is clean for release
	@git diff --quiet HEAD || (echo "ERROR: Working tree has uncommitted changes" && exit 1)
	@git diff --cached --quiet HEAD || (echo "ERROR: Index has uncommitted changes" && exit 1)
	@echo "Working tree is clean"

.PHONY: release-dry-run
release-dry-run: ## Run goreleaser locally in snapshot mode
	goreleaser release --snapshot --clean

.PHONY: release
release: check changelog release-check ## Create and push a signed release tag (auto-increments patch)
	@echo "Creating release tag $(RELEASE_VERSION)..."
	git tag -s $(RELEASE_VERSION) -m "Release $(RELEASE_VERSION)"
	git push origin $(RELEASE_VERSION)

.PHONY: tag
tag: ## Create a signed tag (auto-increments patch)
	@echo "Creating tag $(RELEASE_VERSION)..."
	git tag $(RELEASE_VERSION) -m "Release $(RELEASE_VERSION)"
