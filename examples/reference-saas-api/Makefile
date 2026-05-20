GO ?= go
API_TOOLKIT ?= $(GO) run -mod=mod github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit
OPENAPI ?= testdata/openapi.golden.json
OPENAPI_BASE ?= $(OPENAPI)
COMPOSE ?= docker compose

.PHONY: deps test fmt build openapi-check openapi-update contracts-lint contracts-diff contracts-changelog contracts-impact client-check client-ts-check asset-check observability-check deploy-check resource-check provider-check provider-live-check migrate-plan migrate-up migrate-status migrate-check migrate-verify migrate-down integration-check clean finalize

deps:
	$(GO) mod tidy

test: deps
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

build: deps
	$(GO) build -trimpath -o bin/api ./cmd/api
	$(GO) build -trimpath -o bin/worker ./cmd/worker
	$(GO) build -trimpath -o bin/migrate ./cmd/migrate

openapi-check: deps
	$(GO) test ./internal/httpapi -run TestOpenAPIGolden

openapi-update:
	UPDATE_OPENAPI=1 $(GO) test ./internal/httpapi -run TestOpenAPIGolden

contracts-lint:
	$(API_TOOLKIT) contracts lint --openapi $(OPENAPI)

contracts-diff:
	$(API_TOOLKIT) contracts diff --base $(OPENAPI_BASE) --head $(OPENAPI)

contracts-changelog:
	$(API_TOOLKIT) contracts changelog --base $(OPENAPI_BASE) --head $(OPENAPI)

contracts-impact:
	$(API_TOOLKIT) contracts impact --base $(OPENAPI_BASE) --head $(OPENAPI)

client-check: deps
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	cp internal/client/apiclient/client.go "$$tmp/client.go"; \
	$(API_TOOLKIT) clients go --openapi $(OPENAPI) --out internal/client/apiclient --package apiclient --style typed; \
	cmp -s "$$tmp/client.go" internal/client/apiclient/client.go || { echo "generated Go client is out of date"; diff -u "$$tmp/client.go" internal/client/apiclient/client.go; exit 1; }
	$(GO) test ./internal/client/apiclient

client-ts-check:
	@if [ -f internal/client/ts/src/index.ts ]; then \
		tmp=$$(mktemp -d); \
		trap 'rm -rf "$$tmp"' EXIT; \
		cp internal/client/ts/src/index.ts "$$tmp/index.ts"; \
		$(API_TOOLKIT) clients typescript --openapi $(OPENAPI) --out internal/client/ts --package-name @example/api-client --style fetch; \
		cmp -s "$$tmp/index.ts" internal/client/ts/src/index.ts || { echo "generated TypeScript client is out of date"; diff -u "$$tmp/index.ts" internal/client/ts/src/index.ts; exit 1; }; \
		if command -v npm >/dev/null 2>&1 && [ -d internal/client/ts/node_modules ]; then (cd internal/client/ts && npm run build --silent); else echo "TypeScript build skipped; run npm install in internal/client/ts to enable it"; fi; \
	else \
		echo "TypeScript client not generated"; \
	fi

asset-check:
	$(GO) run ./cmd/assetcheck all

observability-check:
	$(GO) run ./cmd/assetcheck observability

deploy-check:
	$(GO) run ./cmd/assetcheck deploy

resource-check: test openapi-check contracts-lint contracts-diff client-check

provider-check:
	@packages=""; \
	if [ -d internal/providers ]; then packages="$$packages ./internal/providers/..."; fi; \
	if [ -d internal/entitlements ]; then packages="$$packages ./internal/entitlements/..."; fi; \
	if [ -z "$$packages" ]; then echo "no provider workflows generated"; else $(GO) test $$packages; fi
	@if [ -d cmd/provider-replay ]; then \
		$(GO) test ./cmd/provider-replay; \
		$(GO) run ./cmd/provider-replay --fixture-dir testdata/providers; \
	fi

provider-live-check:
	@if [ "$${RUN_PROVIDER_LIVE_CHECKS:-}" != "true" ]; then echo "set RUN_PROVIDER_LIVE_CHECKS=true to run live provider checks"; exit 2; fi
	$(GO) test -tags=provider_live ./internal/providers/...

migrate-plan:
	$(GO) run ./cmd/migrate plan

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-status:
	$(GO) run ./cmd/migrate status

migrate-check:
	$(GO) run ./cmd/migrate check

migrate-verify:
	$(GO) run ./cmd/migrate verify

migrate-down:
	ALLOW_DANGEROUS_MIGRATION_DOWN=true $(GO) run ./cmd/migrate --allow-dangerous-down down

integration-check:
	bash scripts/integration_check.sh

clean:
	$(GO) clean -testcache

finalize: fmt test build openapi-check contracts-lint contracts-diff asset-check clean
