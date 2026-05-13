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

.PHONY: help tools api-check release-api-check api-check-contract contrib-api-drift-report contrib-release-notes-check contrib-review-contract release-artifact-verify-contract release-evidence-parser-contract docs-check fmt lint vuln gosec tidy test coverage coverage-check fast-check test-race fuzz clean finalize audit-check reviewer-gate release-check release-evidence release-review-summary release-artifact-verify release-artifact-verify-fixture ci-build-smoke codeql-local .codeql-local-build scorecard-local sbom-local

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

# In api-check you can set API_BASE_REF to compare to a specific tag (for example API_BASE_REF=v2.1.0).
# Without API_BASE_REF it uses local development fallback behavior and is not release evidence.
api-check: tools ## Run local API compatibility check with fallback base selection
	@scripts/apicheck.sh

release-api-check: tools ## Run fail-closed API compatibility check; requires API_BASE_REF
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for release-api-check; for v2.x releases use API_BASE_REF=v2.1.0"; exit 2; }
	@API_CHECK_REQUIRE_BASE=1 scripts/apicheck.sh

api-check-contract: ## Run API compatibility script mode contract tests
	@env -u API_BASE_REF -u API_CHECK_REQUIRE_BASE -u GITHUB_BASE_REF scripts/apicheck_contract_test.sh

contrib-api-drift-report: tools ## Check selected contrib API drift without making contrib stable
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for contrib-api-drift-report; for v2.x releases use API_BASE_REF=v2.1.0"; exit 2; }
	@API_BASE_REF="$(API_BASE_REF)" scripts/contrib_api_drift_report.sh

contrib-release-notes-check: tools ## Review gate requiring release notes for supported contrib behavior/runtime changes
	@CONTRIB_RELEASE_BASE_REF="$${CONTRIB_RELEASE_BASE_REF:-$${API_BASE_REF:-HEAD~1}}" scripts/contrib_release_notes_check.sh

contrib-review-contract: ## Run contrib drift/release-note script contract tests
	@env -u API_BASE_REF -u CONTRIB_API_BASE_REF -u CONTRIB_RELEASE_BASE_REF scripts/contrib_review_contract_test.sh

docs-check: ## Run documentation contract checks
	@$(GO) test ./docscheck -count=1
	@$(MAKE) api-check-contract
	@$(MAKE) contrib-review-contract
	@$(MAKE) release-artifact-verify-contract
	@$(MAKE) release-evidence-parser-contract

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
		(cd $$mod && govulncheck -show verbose ./...); \
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

coverage: ## Run unit tests with coverage reporting
	@GO="$(GO)" scripts/coverage_check.sh

coverage-check: ## Run unit tests with coverage thresholds
	@GO="$(GO)" scripts/coverage_check.sh --check

fast-check: ## Run non-installing local checks without rewriting files
	@$(MAKE) docs-check
	@$(MAKE) test

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
	$(MAKE) docs-check
	$(MAKE) tidy
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) fuzz
	$(MAKE) clean

audit-check: ## Run non-mutating review checks without fmt or tidy
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) gosec
	$(MAKE) ci-build-smoke
	$(MAKE) docs-check
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) fuzz

reviewer-gate: ## Run non-mutating reviewer checks plus release evidence policy preflight
	$(MAKE) audit-check
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for reviewer-gate; for v2.x releases use API_BASE_REF=v2.1.0"; exit 2; }
	@API_BASE_REF="$(API_BASE_REF)" GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" scripts/release_check_summary.sh >/dev/null

release-check: ## Run release readiness checks; requires explicit API_BASE_REF
	$(MAKE) tools
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) gosec
	$(MAKE) ci-build-smoke
	$(MAKE) release-api-check
	$(MAKE) contrib-api-drift-report
	$(MAKE) contrib-release-notes-check
	$(MAKE) docs-check
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) fuzz
	$(MAKE) clean

release-evidence: ## Run release readiness and write release-check-summary.json
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for release-evidence; for v2.x releases use API_BASE_REF=v2.1.0"; exit 2; }
	@tmp="$$(mktemp)"; \
	status=0; \
	API_BASE_REF="$(API_BASE_REF)" GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" scripts/release_check_summary.sh --run > "$$tmp" || status=$$?; \
	mv "$$tmp" release-check-summary.json || { rm -f "$$tmp"; exit 1; }; \
	exit $$status

release-review-summary: ## Print reviewer decision fields from release-check-summary.json
	@bash scripts/release_review_summary.sh "$${RELEASE_SUMMARY:-release-check-summary.json}"

release-artifact-verify: ## Verify downloaded draft release assets, checksums, SBOM signatures, and retained logs
	@bash scripts/release_artifact_verify.sh "$${RELEASE_ASSET_DIR:-.}"

release-artifact-verify-fixture: ## Exercise release artifact verification against a synthetic local fixture
	@bash scripts/release_artifact_verify_fixture.sh

release-artifact-verify-contract: ## Run release artifact verifier contract tests
	@bash scripts/release_artifact_verify_contract_test.sh

release-evidence-parser-contract: ## Run release evidence parser contract tests
	@bash scripts/release_evidence_parser_contract_test.sh

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
