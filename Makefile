.PHONY: build test test-race test-cover vet lint run clean deps help ci-test ci-lint ci-build

GO        ?= /usr/local/go/bin/go
PKG       := ./...
BIN       := bin/devops-toolkit
COVERFILE := coverage.out

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Download and tidy module dependencies
	$(GO) mod download
	$(GO) mod tidy

build: ## Compile all packages and produce binary
	mkdir -p bin
	$(GO) build -o $(BIN) ./cmd/devops-toolkit

test: ## Run all unit and integration tests
	$(GO) test $(PKG)

test-race: ## Run tests with race detector
	$(GO) test -race $(PKG)

test-cover: ## Run tests with coverage report
	$(GO) test -coverprofile=$(COVERFILE) $(PKG)
	$(GO) tool cover -func=$(COVERFILE) | tail -1
	$(GO) tool cover -html=$(COVERFILE) -o coverage.html

vet: ## Run go vet
	$(GO) vet $(PKG)

lint: ## Run basic linting (vet + gofmt)
	$(GO) vet $(PKG)
	@files=$$($(GO) list -f '{{.Dir}}' $(PKG) | xargs -I {} sh -c 'ls {}/**/*.go 2>/dev/null'); \
	if [ -n "$$files" ]; then \
		out=$$($(GO)fmt -l $$files 2>/dev/null); \
		if [ -n "$$out" ]; then echo "Unformatted files:"; echo "$$out"; exit 1; fi; \
	fi
	@echo "lint ok"

run: build ## Build and run the server (default port 18080; override with APP__PORT=NNNN)
	CONFIG_PATH=configs/templates/config-dev.yaml LOG_FORMAT=text LOG_LEVEL=info APP__PORT=18080 ./$(BIN)

clean: ## Remove build artifacts
	rm -rf bin coverage.out coverage.html

# ===== CI targets (consumed by .github/workflows/ci.yml) =====
#
# Each target delegates to the corresponding scripts/ci-*.sh entry so the
# standard interface (deploy|status|logs|teardown|help) is exercised in CI
# exactly as it is on a developer laptop.
ci-test: ## Run go test -race + 75% coverage gate (scripts/ci-test.sh deploy)
	bash scripts/ci-test.sh deploy

ci-lint: ## Run go vet + gofmt -l (scripts/ci-lint.sh deploy)
	bash scripts/ci-lint.sh deploy

ci-build: ## Build binary and assert < 50MB (scripts/ci-build.sh deploy)
	bash scripts/ci-build.sh deploy
