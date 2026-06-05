#!/usr/bin/env bash
# Verifies that stable root packages stay free of contrib, provider, scaffold,
# generated-app, and example dependencies.
set -euo pipefail

GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK="${GOWORK:-off}" "${GO:-go}" test ./docscheck -count=1 -run '^TestStableCoreDependencyBoundariesExcludeContribProvidersAndGeneratedApps$'
