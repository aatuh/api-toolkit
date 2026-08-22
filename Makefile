MODULES := . contrib
GO ?= go
WORKSPACE := $(abspath go.work)
FUZZTIME ?= 10s
BENCHTIME ?= 1x
COVERAGE_TREND_RELEASE ?=
COVERAGE_TREND_COMMIT ?=
MUTATION_PACKAGES ?= ./binding,./queryparams,./negotiation,./webhooks
MUTATION_LIMIT ?= 12
MUTATION_TIMEOUT ?= 30s
MUTATION_GATE_PACKAGES ?= ./binding,./queryparams,./negotiation,./webhooks
MUTATION_GATE_PER_PACKAGE_LIMIT ?= 3
MUTATION_GATE_MIN_KILL_RATE ?= 0.75
# Standard tools.
TOOLS := golangci-lint gosec govulncheck
TOOLS_DIR ?= .tools
TOOLS_BIN := $(abspath $(TOOLS_DIR)/bin)
GOLANGCI_LINT_VERSION ?= v2.11.4
GOSEC_VERSION ?= v2.25.0
GOVULNCHECK_VERSION ?= v1.2.0
APIDIFF_VERSION ?= v0.0.0-20260410095643-746e56fc9e2f

# Keep Go-installed tooling project-local and make every recipe (including
# release-evidence helper scripts) resolve that pinned toolchain first.
export PATH := $(TOOLS_BIN):$(PATH)

# Tools used by local CI steps.
OUTPUT_DIR ?= .ci-result
MUTATION_OUT ?= $(OUTPUT_DIR)/mutation/mutation-smoke.tsv
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

.PHONY: help tools api-check release-api-check api-check-contract api-inventory api-inventory-check api-additions-check api-additions-check-contract docs-site docs-site-check dead-code-todo-check dead-code-todo-contract contrib-api-drift-report contrib-release-notes-check dependency-report dependency-boundary-check full-profile-scaffold-check generated-integration-check generated-integration-check-minio generated-integration-contract generated-soak-check generated-soak-contract generated-failure-check generated-failure-contract generated-upgrade-compat-check generated-upgrade-compat-contract upgrade-smoke-check upgrade-smoke-contract reference-service-check reference-service-coverage reference-service-load reference-service-load-contract reference-service-evidence reference-service-evidence-contract test-postgres test-redis supported-adapter-check v3-readiness-check contrib-review-contract actions-audit actions-audit-contract sbom-license-report-contract release-artifact-verify-contract release-evidence-parser-contract release-tag-consistency-check release-tag-consistency-contract release-quality-baseline-contract version-consistency-check version-consistency-contract pr-title-check pr-title-check-contract required-checks-verify required-checks-verify-contract docs-check fmt lint vuln gosec tidy test example-compile-check coverage coverage-check coverage-trend-record coverage-trend-check benchmark-baseline-check fast-check test-race timeout-determinism-check fuzz fuzz-contract mutation-smoke mutation-check benchmark-smoke clean finalize audit-check reviewer-gate release-check release-evidence release-review-summary release-artifact-verify release-artifact-verify-fixture ci-build-smoke codeql-local .codeql-local-build scorecard-local sbom-local github-governance-check

help: ## Show help
	@awk 'BEGIN {FS=":.*## "}; \
		/^[a-zA-Z0-9_.-]+:.*## / { \
			if (match($$0, /## .*## /)) { \
				printf "error: multiple ## in help comment for target %s\n", $$1; exit 1; \
			} \
			printf "  %-14s %s\n", $$1, $$2 \
		}' $(MAKEFILE_LIST)

tools: ## Install pinned lint/vuln/API tools into the project-local tool cache
	@mkdir -p "$(TOOLS_BIN)"
	@GOBIN="$(TOOLS_BIN)" $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@GOBIN="$(TOOLS_BIN)" $(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	@GOBIN="$(TOOLS_BIN)" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	@GOBIN="$(TOOLS_BIN)" $(GO) install golang.org/x/exp/cmd/apidiff@$(APIDIFF_VERSION)

# In api-check you can set API_BASE_REF to compare to a specific tag (for example API_BASE_REF=v4.0.0).
# Without API_BASE_REF it uses local development fallback behavior and is not release evidence.
api-check: tools ## Run local API compatibility check with fallback base selection
	@scripts/apicheck.sh

release-api-check: tools ## Run fail-closed API compatibility check; requires API_BASE_REF
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for release-api-check; use the latest published v4 baseline, for example API_BASE_REF=v4.0.0"; exit 2; }
	@API_CHECK_REQUIRE_BASE=1 scripts/apicheck.sh

api-check-contract: ## Run API compatibility script mode contract tests
	@env -u API_BASE_REF -u API_CHECK_REQUIRE_BASE -u GITHUB_BASE_REF scripts/apicheck_contract_test.sh

api-inventory: ## Regenerate docs/api-inventory.md from stable package source
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(GO) run ./internal/tools/apiinventory

api-inventory-check: ## Verify docs/api-inventory.md is current
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/api_inventory_check.sh

api-additions-check: ## Require evidence for new stable exported identifiers
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/api_additions_check.sh

api-additions-check-contract: ## Run API additions script mode contract tests
	@env -u API_ADDITIONS_BASE_REF -u API_BASE_REF -u GITHUB_BASE_REF scripts/api_additions_check_contract_test.sh

docs-site: ## Regenerate the static API docs site
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(GO) run ./internal/tools/docsite

docs-site-check: ## Verify the static API docs site is current
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(GO) run ./internal/tools/docsite -check

dead-code-todo-check: ## Reject unclassified TODO/FIXME in stable packages
	@scripts/dead_code_todo_check.sh

dead-code-todo-contract: ## Run stable TODO gate contract tests
	@scripts/dead_code_todo_contract_test.sh

contrib-api-drift-report: tools ## Check selected contrib API drift without making contrib stable
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for contrib-api-drift-report; for v4 patch/minor releases use the latest published v4 tag, for example API_BASE_REF=v4.0.0"; exit 2; }
	@API_BASE_REF="$(API_BASE_REF)" scripts/contrib_api_drift_report.sh

contrib-release-notes-check: tools ## Review gate requiring release notes for supported contrib behavior/runtime changes
	@CONTRIB_RELEASE_BASE_REF="$${CONTRIB_RELEASE_BASE_REF:-$${API_BASE_REF:-HEAD~1}}" scripts/contrib_release_notes_check.sh

dependency-report: ## Write root/contrib dependency footprint and optional API_BASE_REF diff
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GO="$(GO)" scripts/dependency_report.sh

dependency-boundary-check: ## Verify stable core import boundaries
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" GO="$(GO)" scripts/dependency_boundary_check.sh

full-profile-scaffold-check: ## Generate and validate the saas-api-full scaffold, clients, contracts, resource generator, providers, and web profile
	@(cd contrib && GOWORK="$(WORKSPACE)" $(GO) test ./cmd/api-toolkit -count=1 -run '^(TestNewServiceGeneratesBuildableSaaSAPIFull|TestNewServiceGeneratesBuildableSaaSAPIFullWithOIDC|TestNewServiceGeneratesBuildableSaaSAPIFullWithJWT|TestNewServiceGeneratesBuildableSaaSAPIFullWithClerk|TestGenerateResourceSupportsAppOwnedReplacementErgonomics|TestNewServiceGeneratesFullProfileProviderWorkflows|TestNewServiceGeneratesFullProfileTypeScriptClientAndEntitlements|TestNewServiceGeneratesSaaSWebSessionProfile)$$')

generated-integration-check: ## Opt-in Docker-backed generated saas-api-full integration check
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/generated_integration_check.sh

generated-integration-check-minio: ## Opt-in generated saas-api-full integration check with MinIO
	@INCLUDE_MINIO=true GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/generated_integration_check.sh

generated-integration-contract: ## Run generated integration script contract tests
	@scripts/generated_integration_contract_test.sh

generated-soak-check: ## Opt-in nightly generated saas-api-full soak check
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/generated_soak_check.sh

generated-soak-contract: ## Run generated soak script contract tests
	@scripts/generated_soak_contract_test.sh

generated-failure-check: ## Opt-in nightly generated saas-api-full chaos/failure check
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/generated_failure_check.sh

generated-failure-contract: ## Run generated failure script contract tests
	@scripts/generated_failure_contract_test.sh

generated-upgrade-compat-check: ## Opt-in generated service upgrade compatibility check from the prior v3 baseline
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/generated_upgrade_compat_check.sh

generated-upgrade-compat-contract: ## Run generated upgrade compatibility script contract tests
	@scripts/generated_upgrade_compat_contract_test.sh

upgrade-smoke-check: ## Verify a small downstream module pinned to the prior v3 tag works after replacing to this checkout
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK=off scripts/upgrade_smoke_check.sh

upgrade-smoke-contract: ## Run upgrade smoke script contract tests
	@scripts/upgrade_smoke_contract_test.sh

reference-service-check: ## Verify the checked-in reference saas-api-full service without Docker
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(MAKE) -C examples/reference-saas-api test openapi-check contracts-lint contracts-diff client-check asset-check observability-check deploy-check

reference-service-coverage: ## Record non-Docker reference service coverage separately from root/contrib thresholds
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" GO="$(GO)" scripts/reference_service_coverage.sh

reference-service-load: ## Record non-Docker reference service load smoke baseline
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" GO="$(GO)" scripts/reference_service_load.sh

reference-service-load-contract: ## Run reference service load smoke script contract tests
	@scripts/reference_service_load_contract_test.sh

reference-service-evidence: ## Record non-blocking reference service evidence under .ci-result/reference-service
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" scripts/reference_service_evidence.sh

reference-service-evidence-contract: ## Run reference service evidence script contract tests
	@scripts/reference_service_evidence_contract_test.sh

test-postgres: ## Run isolated real PostgreSQL harness tests (Docker starts locally when no test DSN is configured)
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" bash scripts/test_postgres.sh

test-redis: ## Run isolated real Redis harness tests (Docker starts locally when no test URL is configured)
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" bash scripts/test_redis.sh

supported-adapter-check: ## Verify supported PostgreSQL and Redis real-service evidence wiring
	@scripts/supported_adapter_check.sh

github-governance-check: ## Optional authenticated GitHub branch/tag protection verification
	@scripts/github_governance_check.sh

required-checks-verify: ## Verify the required-check manifest against stable workflow job identities
	@scripts/required_checks_verify.sh

required-checks-verify-contract: ## Exercise required-check manifest and branch-protection failure modes
	@bash scripts/required_checks_verify_contract_test.sh

v3-readiness-check: ## Run compatibility-sensitive v3 readiness guardrails
	@$(GO) test ./docscheck -count=1 -run 'TestCompatibilitySensitivePortsManifestIsCurrent|TestContribPackageClassificationAndCompatibilityPolicy|TestCompatibilityShimLifecycleRoadmap|TestIdempotencyCompatibilityMetricDocsStayBounded|TestResponseWriterInventoryMatchesCurrentImports|TestPublicExamplesDoNotTeachLegacyCompatibilitySurfaces|TestV3RemovalMatrixHasExecutableEvidence|TestV3DebtChecklistRowsStayExecutable|TestCompatibilityRoadmapCoversDocumentedSensitiveSurfaces|TestCompatibilitySensitivePortsGovernanceDocs|TestCompatibilitySensitivePackageDocsPointToReplacements|TestExamplesAndGuidesPreferCompatibilityReplacements|TestReleaseNotesIncludeStableSurfaceChecklist|TestDeprecatedBillingPortsPointToCompatPackage|TestDeprecatedBillingPortsStayInCompatibilitySource|TestDatabaseStatsStayInCompatibilityOrAdapterSource|TestAdapterLegacyRecoveryTelemetryRedactsKeysByDefault|TestIdempotencyCaptureDoesNotUseLegacyResponseWriter'

contrib-review-contract: ## Run contrib drift/release-note script contract tests
	@env -u API_BASE_REF -u CONTRIB_API_BASE_REF -u CONTRIB_RELEASE_BASE_REF scripts/contrib_review_contract_test.sh

actions-audit: ## Audit GitHub Actions pins and generated workflow template versions
	@scripts/actions_audit.sh

actions-audit-contract: ## Run GitHub Actions audit script contract tests
	@scripts/actions_audit_contract_test.sh

sbom-license-report-contract: ## Run SPDX dependency license report contract tests
	@bash scripts/sbom_license_report_contract_test.sh

docs-check: ## Run documentation contract checks
	@$(GO) test ./docscheck -count=1
	@$(MAKE) required-checks-verify
	@$(MAKE) required-checks-verify-contract
	@$(MAKE) version-consistency-check
	@$(MAKE) version-consistency-contract
	@$(MAKE) coverage-trend-check
	@$(MAKE) release-quality-baseline-contract
	@$(MAKE) docs-site-check
	@$(MAKE) api-inventory-check
	@$(MAKE) api-additions-check
	@$(MAKE) api-additions-check-contract
	@$(MAKE) dead-code-todo-contract
	@$(MAKE) api-check-contract
	@$(MAKE) contrib-review-contract
	@$(MAKE) fuzz-contract
	@$(MAKE) actions-audit-contract
	@$(MAKE) sbom-license-report-contract
	@$(MAKE) release-artifact-verify-contract
	@$(MAKE) release-evidence-parser-contract
	@$(MAKE) release-tag-consistency-contract
	@$(MAKE) pr-title-check-contract
	@$(MAKE) generated-integration-contract
	@$(MAKE) generated-soak-contract
	@$(MAKE) generated-failure-contract
	@$(MAKE) generated-upgrade-compat-contract
	@$(MAKE) upgrade-smoke-contract
	@$(MAKE) reference-service-load-contract
	@$(MAKE) reference-service-evidence-contract

fmt: ## Run gofmt and rewrite formatted Go files
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) fmt ./...); \
	done

lint: tools ## Run golangci-lint
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" golangci-lint run ./...); \
	done
	@$(MAKE) dead-code-todo-check

vuln: tools ## Run govulncheck
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" govulncheck -show verbose ./...); \
	done

gosec: tools ## Run gosec
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		if [ "$$mod" = "." ]; then \
			(cd $$mod && GOWORK="$(WORKSPACE)" gosec -exclude-dir=contrib ./...); \
		else \
			(cd $$mod && GOWORK="$(WORKSPACE)" gosec ./...); \
		fi; \
	done

tidy: ## Run go mod tidy and rewrite module files
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) mod tidy); \
	done

test: ## Run unit tests
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) test ./...); \
	done

example-compile-check: ## Compile package examples and tested README snippets
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod examples"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) test ./... -run '^Example'); \
	done
	@$(GO) test ./docscheck -count=1 -run 'TestReadmeGoSnippetsMatchTestedSources|TestStableAPIPackagesHaveCompileCheckedExamples|TestExampleOnlyPackagesBuildSmoke'

coverage: ## Run unit tests with coverage reporting
	@GO="$(GO)" scripts/coverage_check.sh

coverage-check: ## Run unit tests with coverage thresholds
	@GO="$(GO)" scripts/coverage_check.sh --check

coverage-trend-record: ## Record the current package coverage summary for a release tag
	@test -n "$(COVERAGE_TREND_RELEASE)" || { echo "COVERAGE_TREND_RELEASE is required, for example v3.1.3"; exit 2; }
	@test -n "$(COVERAGE_TREND_COMMIT)" || { echo "COVERAGE_TREND_COMMIT is required, for example $$(git rev-parse --verify HEAD)"; exit 2; }
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(GO) run ./internal/tools/coveragetrend -record "$(COVERAGE_TREND_RELEASE)" -commit "$(COVERAGE_TREND_COMMIT)"

coverage-trend-check: ## Verify public package coverage trend documentation
	@GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" GOWORK="$${GOWORK:-off}" $(GO) run ./internal/tools/coveragetrend -check

benchmark-baseline-check: ## Verify release-specific coverage and benchmark baselines
	@RELEASE_QUALITY_RELEASE="$${RELEASE_TAG:-$${BENCHMARK_BASELINE_RELEASE:-v4.0.1}}" bash scripts/release_quality_baseline_check.sh

fast-check: ## Run non-installing local checks without rewriting files
	@$(MAKE) docs-check
	@$(MAKE) example-compile-check
	@$(MAKE) test

test-race: ## Run unit tests with race detector
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) test ./... -race -count=1); \
	done

timeout-determinism-check: ## Run repeated timeout middleware determinism and race checks
	@$(GO) test ./middleware/timeout -count=100 -run TestHardTimeoutWritesProblemAndDiscardsLateHandlerResponse
	@$(GO) test ./middleware/timeout -race -count=100 -run TestHardTimeoutWritesProblemAndDiscardsLateHandlerResponse
	@$(GO) test ./middleware/idempotency ./middleware/timeout ./middleware/ratelimit ./scheduler -race -count=1

fuzz: ## Run fuzz smoke tests
	@GO="$(GO)" GOWORK="$(WORKSPACE)" FUZZTIME="$(FUZZTIME)" scripts/fuzz_check.sh $(MODULES)

fuzz-contract: ## Run fuzz smoke script contract tests
	@scripts/fuzz_check_contract_test.sh

mutation-smoke: ## Run non-blocking stable-core mutation smoke
	@GO="$(GO)" MUTATION_PACKAGES="$(MUTATION_PACKAGES)" MUTATION_LIMIT="$(MUTATION_LIMIT)" MUTATION_TIMEOUT="$(MUTATION_TIMEOUT)" MUTATION_OUT="$(MUTATION_OUT)" scripts/mutation_smoke.sh

mutation-check: ## Run blocking selected-package mutation gate
	@GO="$(GO)" MUTATION_PACKAGES="$(MUTATION_GATE_PACKAGES)" MUTATION_LIMIT=0 MUTATION_PER_PACKAGE_LIMIT="$(MUTATION_GATE_PER_PACKAGE_LIMIT)" MUTATION_MIN_KILL_RATE="$(MUTATION_GATE_MIN_KILL_RATE)" MUTATION_TIMEOUT="$(MUTATION_TIMEOUT)" MUTATION_OUT="$(OUTPUT_DIR)/mutation/mutation-check.tsv" scripts/mutation_smoke.sh

benchmark-smoke: ## Run package benchmark smoke without performance claims
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) test ./... -run='^$$' -bench='Benchmark' -benchmem -benchtime=$(BENCHTIME)); \
	done

clean: ## Clean test cache
	@set -e; for mod in $(MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && GOWORK="$(WORKSPACE)" $(GO) clean -testcache); \
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
	$(MAKE) actions-audit
	$(MAKE) docs-check
	$(MAKE) example-compile-check
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) timeout-determinism-check
	$(MAKE) fuzz

reviewer-gate: ## Run non-mutating reviewer checks plus release evidence policy preflight
	$(MAKE) audit-check
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for reviewer-gate; for v4 patch/minor releases use the latest published v4 tag, for example API_BASE_REF=v4.0.0"; exit 2; }
	@API_BASE_REF="$(API_BASE_REF)" GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" scripts/release_check_summary.sh >/dev/null

release-check: ## Run release readiness checks; requires explicit API_BASE_REF
	$(MAKE) tools
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) gosec
	$(MAKE) ci-build-smoke
	$(MAKE) required-checks-verify
	$(MAKE) release-api-check
	$(MAKE) contrib-api-drift-report
	$(MAKE) contrib-release-notes-check
	$(MAKE) dependency-report
	$(MAKE) full-profile-scaffold-check
	$(MAKE) v3-readiness-check
	$(MAKE) docs-check
	$(MAKE) example-compile-check
	$(MAKE) coverage-check
	$(MAKE) benchmark-baseline-check
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) fuzz
	$(MAKE) mutation-check
	$(MAKE) clean

release-evidence: ## Run release readiness and write release-check-summary.json
	@test -n "$(API_BASE_REF)" || { echo "API_BASE_REF is required for release-evidence; for v4 patch/minor releases use the verified baseline, for example API_BASE_REF=v4.0.1"; exit 2; }
	@test -n "$(RELEASE_TAG)" || test -n "$${GITHUB_REF_NAME:-}" || { case "$${ALLOW_DIRTY_RELEASE_EVIDENCE:-}" in 1|true|TRUE|yes|YES) exit 0 ;; *) echo "RELEASE_TAG is required for release-evidence outside a tag-triggered GitHub workflow"; exit 2 ;; esac; }
	@if test -n "$(RELEASE_TAG)" || test -n "$${GITHUB_REF_NAME:-}"; then $(MAKE) release-tag-consistency-check; fi
	@tmp="$$(mktemp)"; \
	status=0; \
	API_BASE_REF="$(API_BASE_REF)" GOTOOLCHAIN="$${GOTOOLCHAIN:-local}" scripts/release_check_summary.sh --run > "$$tmp" || status=$$?; \
	mv "$$tmp" release-check-summary.json || { rm -f "$$tmp"; exit 1; }; \
	exit $$status

release-review-summary: ## Print reviewer decision fields from release-check-summary.json
	@bash scripts/release_review_summary.sh "$${RELEASE_SUMMARY:-release-check-summary.json}"

release-artifact-verify: ## Verify release assets; publication mode validates the tag, checksums, signatures, and attestations
	@bash scripts/verify-release.sh "$${RELEASE_ASSET_DIR:-.}"

release-artifact-verify-fixture: ## Exercise release artifact verification against a synthetic local fixture
	@bash scripts/release_artifact_verify_fixture.sh

release-artifact-verify-contract: ## Run release artifact verifier contract tests
	@bash scripts/release_artifact_verify_contract_test.sh

release-evidence-parser-contract: ## Run release evidence parser contract tests
	@bash scripts/release_evidence_parser_contract_test.sh

release-tag-consistency-check: ## Enforce root/contrib tag, module, docs, branch, and release-baseline coherence
	@bash scripts/release_tag_consistency.sh

release-tag-consistency-contract: ## Run release tag and module coherence contract tests
	@bash scripts/release_tag_consistency_contract_test.sh

release-quality-baseline-contract: ## Run release quality baseline validation contract tests
	@bash scripts/release_quality_baseline_contract_test.sh

version-consistency-check: ## Reject stale current-version guidance and root imports
	@bash scripts/version_consistency_check.sh

version-consistency-contract: ## Run current-version consistency contract tests
	@bash scripts/version_consistency_contract_test.sh

pr-title-check: ## Validate PR_TITLE against the conventional commit title policy
	@bash scripts/pr_title_check.sh

pr-title-check-contract: ## Run pull-request title validation contract tests
	@bash scripts/pr_title_check_contract_test.sh

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
