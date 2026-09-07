.PHONY: build test testacc lint vuln license clean help vet check docs release-check

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the provider
	go build -o terraform-provider-streamline

test: ## Run tests
	go test ./...

testacc: ## Run full acceptance tests (requires broker and Schema Registry fixtures)
	@test -n "$$STREAMLINE_BOOTSTRAP_SERVERS" || (echo "STREAMLINE_BOOTSTRAP_SERVERS must identify a disposable acceptance fixture" >&2; exit 1)
	@test -n "$$STREAMLINE_SCHEMA_REGISTRY_URL" || (echo "STREAMLINE_SCHEMA_REGISTRY_URL must identify the fixture's Schema Registry" >&2; exit 1)
	@test "$$STREAMLINE_SCHEMA_ACCEPTANCE_ALLOW_RETAINED_SUBJECTS" = "1" || (echo "STREAMLINE_SCHEMA_ACCEPTANCE_ALLOW_RETAINED_SUBJECTS=1 is required because schema teardown retains remote subjects and the fixture must be disposable" >&2; exit 1)
	TF_ACC=1 go test ./internal/provider/ -v -timeout 30m

lint: vet ## Run linting
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run

vuln: ## Check reachable Go vulnerabilities with a pinned tool
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

license: ## Reject forbidden or unknown runtime dependency licenses
	go run github.com/google/go-licenses/v2@v2.0.1 check .

vet: ## Run go vet
	go vet ./...

fmt: ## Format code
	go fmt ./...

clean: ## Clean build artifacts
	rm -f terraform-provider-streamline terraform-provider-streamline_v*
	go clean -cache

docs: ## Generate provider documentation
	go generate ./...

release-check: ## Validate the GoReleaser configuration
	go run github.com/goreleaser/goreleaser/v2@v2.17.0 check

check: fmt vet test lint vuln license release-check ## Run all checks
