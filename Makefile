SHELL := /bin/bash

GO ?= go

TOOLS := golangci-lint gosec govulncheck
GOLANGCI_LINT_VERSION ?= latest
GOSEC_VERSION ?= latest
GOVULNCHECK_VERSION ?= latest


.PHONY: help test test-race lint gosec vuln tidy fmt tools clean finalize

help: ## Show help
	@awk 'BEGIN {FS=":.*## "}; \
		/^[a-zA-Z0-9_.-]+:.*## / { \
			if (match($$0, /## .*## /)) { \
				printf "error: multiple ## in help comment for target %s\n", $$1; exit 1; \
			} \
			printf "  %-14s %s\n", $$1, $$2 \
		}' $(MAKEFILE_LIST)

tools: ## Install lint/vuln tools
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

fmt: ## Run gofmt
	$(GO) fmt ./...

lint: tools ## Run golangci-lint
	golangci-lint run ./...

vuln: tools ## Run govulncheck
	govulncheck ./...

gosec: tools ## Run gosec
	gosec ./...

tidy: ## Run go mod tidy
	$(GO) mod tidy

test: ## Run unit tests
	$(GO) test ./...

test-race: ## Run unit tests with race detector
	$(GO) test ./... -race -count=1

clean: ## Clean test cache
	@$(GO) clean -testcache

finalize: ## Run every quality assurance tool
	$(MAKE) tools
	$(MAKE) fmt
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) gosec
	$(MAKE) tidy
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) clean
