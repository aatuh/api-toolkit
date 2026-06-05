#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

tmp_out="$tmp_dir/api-inventory.md"
(
  cd "$repo_root"
  GOWORK=off go run ./internal/tools/apiinventory -out "$tmp_out"
)

if ! diff -u "$repo_root/docs/api-inventory.md" "$tmp_out"; then
  echo "docs/api-inventory.md is stale; run GOTOOLCHAIN=local make api-inventory" >&2
  exit 1
fi
