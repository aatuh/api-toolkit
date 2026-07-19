#!/usr/bin/env bash
# Review gate for new stable exported identifiers. It compares the generated
# public API inventory against a base ref, then requires each new symbol to have
# source docs, a compile-checked example or exact exception row, and release
# notes coverage.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

base_ref="${API_ADDITIONS_BASE_REF:-${API_BASE_REF:-}}"
base_source="api_additions_base_ref"
if [ -z "$base_ref" ]; then
  if [ -n "${GITHUB_BASE_REF:-}" ]; then
    base_ref="origin/${GITHUB_BASE_REF}"
    base_source="github_base_ref"
  elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    base_ref="HEAD~1"
    base_source="head_parent"
  else
    echo "No base ref available. Skipping API additions check."
    exit 0
  fi
fi

case "$base_ref" in
  ""|-*)
    echo "Invalid API additions base ref: $base_ref" >&2
    exit 2
    ;;
esac

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  if [ "$base_source" = "github_base_ref" ] && [ -n "${GITHUB_BASE_REF:-}" ]; then
    git fetch origin "${GITHUB_BASE_REF}:refs/remotes/origin/${GITHUB_BASE_REF}" --depth=1 >/dev/null 2>&1 || true
  fi
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  if [ -n "${API_ADDITIONS_BASE_REF:-}" ] || [ -n "${API_BASE_REF:-}" ]; then
    echo "Base ref $base_ref not found; set API_ADDITIONS_BASE_REF or API_BASE_REF to a fetched tag or branch." >&2
    exit 2
  fi
  echo "Base ref $base_ref not found. Skipping API additions check."
  exit 0
fi

worktree="$(mktemp -d)"
tmp_dir="$(mktemp -d)"
cleanup() {
  git worktree remove --force "$worktree" >/dev/null 2>&1 || true
  rm -rf "$worktree" "$tmp_dir"
}
trap cleanup EXIT

base_inventory="$tmp_dir/base-api-inventory.md"
git worktree add "$worktree" "$base_ref" --quiet

base_module="$(awk '$1 == "module" { print $2; exit }' "$worktree/go.mod")"
current_module="$(awk '$1 == "module" { print $2; exit }' "$repo_root/go.mod")"
if [ -z "$base_module" ] || [ -z "$current_module" ]; then
  echo "Unable to determine module path for API additions check." >&2
  exit 2
fi
if [ "$base_module" != "$current_module" ]; then
  echo "API module path changed from $base_module to $current_module; skipping API additions check because the gate applies only within a stable major module line."
  exit 0
fi

if [ -f "$worktree/internal/tools/apiinventory/main.go" ]; then
  (
    cd "$worktree"
    GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK=off go run ./internal/tools/apiinventory -out "$base_inventory"
  )
elif [ -f "$worktree/docs/api-inventory.md" ]; then
  cp "$worktree/docs/api-inventory.md" "$base_inventory"
else
  echo "Base ref $base_ref has no API inventory generator or docs/api-inventory.md; skipping API additions check." >&2
  exit 0
fi

(
  cd "$repo_root"
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOWORK=off go run ./internal/tools/apiadditions -base-inventory "$base_inventory"
)
