#!/usr/bin/env bash
# Compares public Go APIs against a base git ref using apidiff.
# Writes exports to a temp dir, skips packages missing in either tree, and fails
# on incompatible changes.
set -euo pipefail

if ! command -v apidiff >/dev/null 2>&1; then
  echo "apidiff not found. Install with: go install golang.org/x/exp/cmd/apidiff@latest" >&2
  exit 1
fi

base_ref="${API_BASE_REF:-}"
if [ -z "$base_ref" ]; then
  if [ -n "${GITHUB_BASE_REF:-}" ]; then
    base_ref="origin/${GITHUB_BASE_REF}"
  elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    base_ref="HEAD~1"
  else
    echo "No base ref available. Skipping API check."
    exit 0
  fi
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  if [ -n "${GITHUB_BASE_REF:-}" ]; then
    git fetch origin "${GITHUB_BASE_REF}" --depth=1 >/dev/null 2>&1 || true
  fi
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  echo "Base ref $base_ref not found. Skipping API check."
  exit 0
fi

repo_root="$(git rev-parse --show-toplevel)"
worktree="$(mktemp -d)"
tmpdir="$(mktemp -d)"
cleanup() {
  git worktree remove --force "$worktree" >/dev/null 2>&1 || true
  rm -rf "$worktree" "$tmpdir"
}
trap cleanup EXIT

git worktree add "$worktree" "$base_ref" --quiet
if [ -f "$worktree/go.mod" ]; then
  (cd "$worktree" && go mod tidy)
fi

packages=(
  "github.com/aatuh/api-toolkit/v2/authorization"
  "github.com/aatuh/api-toolkit/v2/email"
  "github.com/aatuh/api-toolkit/v2/endpoints/docs"
  "github.com/aatuh/api-toolkit/v2/endpoints/health"
  "github.com/aatuh/api-toolkit/v2/endpoints/list"
  "github.com/aatuh/api-toolkit/v2/endpoints/pprof"
  "github.com/aatuh/api-toolkit/v2/endpoints/version"
  "github.com/aatuh/api-toolkit/v2/fielderrors"
  "github.com/aatuh/api-toolkit/v2/httpx"
  "github.com/aatuh/api-toolkit/v2/httpx/identity"
  "github.com/aatuh/api-toolkit/v2/httpx/recover"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/authz"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/tenant"
  "github.com/aatuh/api-toolkit/v2/middleware/idempotency"
  "github.com/aatuh/api-toolkit/v2/middleware/json"
  "github.com/aatuh/api-toolkit/v2/middleware/maxbody"
  "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
  "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
  "github.com/aatuh/api-toolkit/v2/middleware/secure"
  "github.com/aatuh/api-toolkit/v2/middleware/timeout"
  "github.com/aatuh/api-toolkit/v2/middleware/trace"
  "github.com/aatuh/api-toolkit/v2/ports"
  "github.com/aatuh/api-toolkit/v2/response_writer"
  "github.com/aatuh/api-toolkit/v2/scheduler"
  "github.com/aatuh/api-toolkit/v2/scheduler/migrations"
  "github.com/aatuh/api-toolkit/v2/securityprofile"
  "github.com/aatuh/api-toolkit/v2/specs"
  "github.com/aatuh/api-toolkit/v2/swagstub"
)

status=0
for pkg in "${packages[@]}"; do
  rel="${pkg#github.com/aatuh/api-toolkit/v2}"
  rel="${rel#/}"
  old_path="$worktree/${rel}"
  new_path="$repo_root/${rel}"
  if [ ! -d "$old_path" ] || [ ! -d "$new_path" ]; then
    continue
  fi
  (cd "$worktree" && apidiff -w "$tmpdir/old.export" "$pkg")
  apidiff -w "$tmpdir/new.export" "$pkg"
  diff_output="$(apidiff -incompatible "$tmpdir/old.export" "$tmpdir/new.export" || true)"
  if [ -n "$diff_output" ]; then
    printf "Incompatible API changes detected in %s:\n%s\n" "$pkg" "$diff_output"
    status=1
  fi
done

exit "$status"
