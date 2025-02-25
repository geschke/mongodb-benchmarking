GO ?= go
GOLANGCI_LINT ?= golangci-lint
BINARY_NAME=mongo-bench

.PHONY: all
all: build ## Builds the binary

.PHONY: build
build: test ## Builds the binary
	@echo "🔹 Building binary ..."
	$(GO) build -o $(BINARY_NAME) *.go
	@echo "Build complete: $(BINARY_NAME)"

.PHONY: run
run: ## Runs the application with THREADS, DOCS, and URI variables
	@echo "Running $(BINARY_NAME) with THREADS=$(THREADS), DOCS=$(DOCS), URI=$(URI)"
	./$(BINARY_NAME) -threads $(THREADS) -docs $(DOCS) -uri $(URI)

.PHONY: clean
clean: ## Remove generated binary
	@echo "🔹 Deleting go binary exporter_proxy"
	@rm -rf mongo-bench
	@echo "✅  Environment cleaned!"

.PHONY: test
test: ## Run tests
	@echo "🔹 Running tests ..."
	$(GO) test -v ./...
	@echo "✅  Tests OK!"

.PHONY: format
format: ## Format Go code
	@echo "🔹 Formatting Go code..."
	@$(GO) fmt ./...
	@echo "✅  Code formatted!"

.PHONY: lint
lint: ## Run Go linter
	@echo "🔹 Running linter..."
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "⚠️  golangci-lint not found! Installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	@$(GOLANGCI_LINT) run
	@echo "✅  Linting complete!"

.PHONY: format-lint
format-lint: format lint  ## Run format and lint checks
	@echo "🎯 Formatting & Linting completed successfully!"

.PHONY: update
update: ## Update dependencies and tidy the go.mod file
	@echo "Updating dependencies" \
		&& go get -u ./... \
	    && go mod tidy

.PHONY: help
help:
	@echo "📌 Available make targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "🎯 \033[36m%-20s\033[0m %s\n", $$1, $$2}'
