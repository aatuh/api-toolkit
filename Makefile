MODULES := . contrib
GO ?= go

# Standard tools.
TOOLS := golangci-lint gosec govulncheck
GOLANGCI_LINT_VERSION ?= v2.11.4
GOSEC_VERSION ?= v2.25.0
GOVULNCHECK_VERSION ?= v1.2.0
APIDIFF_VERSION ?= v0.0.0-20260410095643-746e56fc9e2f

# Tools used by local CI steps.
OUTPUT_DIR ?= .ci-result
CODEQL ?= codeql
CODEQL_DB_DIR ?= $(OUTPUT_DIR)/codeql-db
CODEQL_QUERIES ?= codeql/go-queries
SCORECARD ?= scorecard
SCORECARD_REPO ?= github.com/aatuh/api-toolkit
SYFT ?= syft
COSIGN ?= cosign
COSIGN_KEY ?= cosign.key

# Env vars only used by local CI steps.
# Create an .env file containing env key-value pairs.
ifneq (,$(wildcard .env))
include .env
export
endif
GITHUB_AUTH_TOKEN ?= $(GITHUB_TOKEN) # GitHub PAT.

.PHONY: help tools api-check docs-check fmt lint vuln gosec tidy test test-race fuzz clean finalize ci-build-smoke codeql-local .codeql-local-build scorecard-local sbom-local

help: ## Show help
	@awk 'BEGIN {FS=":.*## "}; \
		/^[a-zA-Z0-9_.-]+:.*## / { \
			if (match($$0, /## .*## /)) { \
				printf "error: multiple ## in help comment for target %s\n", $$1; exit 1; \
			} \
			printf "  %-14s %s\n", $$1, $$2 \
		}' $(MAKEFILE_LIST)

tools: ## Install lint/vuln/API tools into the Go tool cache
	@$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	@$(GO) install golang.org/x/exp/cmd/apidiff@$(APIDIFF_VERSION)

# In api-check you can set API_BASE_REF to compare to specific tag (e.g. API_BASE_REF=2.0.0)
api-check: ## Run API compatibility check.
	@scripts/apicheck.sh

docs-check: ## Run documentation contract checks
	@$(GO) test ./docscheck -count=1

fmt: ## Run gofmt and rewrite formatted Go files
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) fmt ./...); \
	done

lint: tools ## Run golangci-lint
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && golangci-lint run ./...); \
	done

vuln: tools ## Run govulncheck
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && govulncheck ./...); \
	done

gosec: tools ## Run gosec
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		if [ "$$mod" = "." ]; then \
			(cd $$mod && gosec -exclude-dir=contrib ./...); \
		else \
			(cd $$mod && gosec ./...); \
		fi; \
	done

tidy: ## Run go mod tidy and rewrite module files
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) mod tidy); \
	done

test: ## Run unit tests
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) test ./...); \
	done

test-race: ## Run unit tests with race detector
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) test ./... -race -count=1); \
	done

fuzz: ## Run fuzz smoke tests
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) test ./... -run=^$ -fuzz=Fuzz -fuzztime=10s); \
	done

clean: ## Clean test cache
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && $(GO) clean -testcache); \
	done

finalize: ## Run QA; installs tools and may rewrite formatted/tidy files
	$(MAKE) tools
	$(MAKE) fmt
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) gosec
	$(MAKE) ci-build-smoke
	$(MAKE) api-check
	$(MAKE) docs-check
	$(MAKE) tidy
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) fuzz
	$(MAKE) clean

ci-build-smoke: ## Build root and contrib modules via the local CI build target
	$(MAKE) .codeql-local-build

ci-local: ## Run local CI security steps; requires CodeQL, Scorecard token, and Syft
	rm -rf "$(OUTPUT_DIR)"
	$(MAKE) codeql-local
	$(MAKE) scorecard-local
	$(MAKE) sbom-local

codeql-local: ## Create/analyze a CodeQL DB; requires the codeql binary
	rm -rf "$(OUTPUT_DIR)/codeql"
	mkdir -p "$(OUTPUT_DIR)/codeql"

	codeql database create "$(OUTPUT_DIR)/codeql" \
		--language=go \
		--source-root=. \
		--command="make .codeql-local-build"

	codeql database analyze .ci-result/codeql \
		--download \
		codeql/go-queries \
		--format=sarifv2.1.0 \
		--output=.ci-result/codeql.sarif

	codeql database print-baseline .ci-result/codeql

.codeql-local-build:
	@$(GO) build ./...
	@cd contrib && $(GO) build ./...

scorecard-local: ## Run OpenSSF Scorecard locally; requires GITHUB_AUTH_TOKEN and scorecard
	@test -n "$(GITHUB_AUTH_TOKEN)" || { echo "GITHUB_AUTH_TOKEN is empty"; exit 2; }
	@test -n "$(SCORECARD_REPO)" || { echo "SCORECARD_REPO is empty"; exit 2; }
	rm -rf "$(OUTPUT_DIR)/scorecard"
	mkdir -p "$(OUTPUT_DIR)/scorecard"
	$(SCORECARD) --repo="$(SCORECARD_REPO)" --format=json > "$(OUTPUT_DIR)/scorecard/scorecard.json"

sbom-local: ## Generate SPDX-JSON SBOMs into OUTPUT_DIR; requires syft
	rm -rf "$(OUTPUT_DIR)/sbom"
	mkdir -p "$(OUTPUT_DIR)/sbom"
	"$(SYFT)" dir:. -o spdx-json >"$(OUTPUT_DIR)/sbom/sbom-root.spdx.json"
	"$(SYFT)" dir:contrib -o spdx-json >"$(OUTPUT_DIR)/sbom/sbom-contrib.spdx.json"
