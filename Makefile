.PHONY: build test testacc lint clean help vet check docs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the provider
	go build -o terraform-provider-streamline

test: ## Run tests
	go test ./...

testacc: ## Run acceptance tests (requires STREAMLINE_BOOTSTRAP_SERVERS)
	TF_ACC=1 go test ./internal/provider/ -v -timeout 30m

lint: vet ## Run linting
	golangci-lint run 2>/dev/null || go vet ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format code
	go fmt ./...

clean: ## Clean build artifacts
	rm -f terraform-provider-streamline
	go clean -cache

docs: ## Generate provider documentation
	go generate ./...

check: fmt vet test ## Run all checks
