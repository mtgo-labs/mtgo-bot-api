# mtgo-bot-api Makefile.
# Common dev workflows: build, test, lint, format. Mirrors what CI runs.
#
# Usage:
#   make            # default: build
#   make help       # list all targets
#   make lint       # requires golangci-lint v2 (go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)

GO      ?= go
PKG     := ./...
BINARY  := mtgo-bot-api
# Stray cmd binaries that `go build ./<cmd>/` drops in the repo root.
ARTIFACTS := $(BINARY) validate scrape bench parity-cert coverage.out

.DEFAULT_GOAL := build

##@ Helpers
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 } END { printf "\n" }' $(MAKEFILE_LIST)

##@ Build
.PHONY: build
build: ## Build the server binary (static, no CGO)
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BINARY) ./cmd/mtgo-bot-api

.PHONY: install
install: ## go install the server into $$GOBIN
	CGO_ENABLED=0 $(GO) install ./cmd/mtgo-bot-api

.PHONY: run
run: ## Run the server (set --api-id/--api-hash or TELEGRAM_API_ID/HASH)
	$(GO) run ./cmd/mtgo-bot-api

##@ Testing
.PHONY: test
test: ## Run all unit tests
	$(GO) test $(PKG)

.PHONY: test-race
test-race: ## Run tests with the race detector
	$(GO) test -race $(PKG)

.PHONY: cover
cover: ## Run tests and print per-package coverage
	$(GO) test -cover $(PKG)

.PHONY: cover-html
cover-html: ## Generate an HTML coverage report (coverage.out)
	$(GO) test -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -html=coverage.out

##@ Quality
.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## Run golangci-lint (v2 config)
	golangci-lint run

.PHONY: fmt
fmt: ## Format all Go source in place
	$(GO) fmt $(PKG)
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Fail if any Go source is not gofmt-clean
	@out=$$(gofmt -l $$(git ls-files '*.go' 2>/dev/null || find . -name '*.go' -not -path './.git/*')); \
	if [ -n "$$out" ]; then echo "gofmt would reformat:"; echo "$$out"; exit 1; fi

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: vuln
vuln: ## Run govulncheck
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest $(PKG)

##@ Misc
.PHONY: clean
clean: ## Remove build artifacts and coverage files
	rm -f $(ARTIFACTS) *.prof

.PHONY: check
check: ## Full local CI equivalent: fmt-check, vet, test-race, lint
	@$(MAKE) fmt-check
	@$(MAKE) vet
	@$(MAKE) test-race
	@$(MAKE) lint
