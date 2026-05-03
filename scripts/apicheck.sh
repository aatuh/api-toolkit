#!/usr/bin/env bash
# Compares public Go APIs against a base git ref using apidiff.
#
# API_BASE_REF selects an explicit base ref. Without API_BASE_REF, local
# development checks fall back to GITHUB_BASE_REF, then HEAD~1, then skip when no
# base is available. Set API_CHECK_REQUIRE_BASE=1 for release checks; that mode
# requires API_BASE_REF and fails closed when the ref is missing or invalid.
#
# The script writes exports to a temp dir, skips packages missing in either tree,
# and fails on incompatible changes.
set -euo pipefail

required_base=false
case "${API_CHECK_REQUIRE_BASE:-}" in
  1|true|TRUE|yes|YES) required_base=true ;;
esac

base_ref="${API_BASE_REF:-}"
base_source="api_base_ref"
if [ -z "$base_ref" ]; then
  if [ "$required_base" = true ]; then
    echo "API_BASE_REF is required when API_CHECK_REQUIRE_BASE=1." >&2
    exit 2
  fi
  if [ -n "${GITHUB_BASE_REF:-}" ]; then
    base_ref="origin/${GITHUB_BASE_REF}"
    base_source="github_base_ref"
  elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    base_ref="HEAD~1"
    base_source="head_parent"
  else
    echo "No base ref available. Skipping API check."
    exit 0
  fi
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  if [ "$base_source" = "github_base_ref" ] && [ -n "${GITHUB_BASE_REF:-}" ]; then
    git fetch origin "${GITHUB_BASE_REF}:refs/remotes/origin/${GITHUB_BASE_REF}" --depth=1 >/dev/null 2>&1 || true
  fi
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  if [ "$required_base" = true ] || [ -n "${API_BASE_REF:-}" ]; then
    echo "Base ref $base_ref not found; set API_BASE_REF to a fetched supported tag or branch." >&2
    exit 2
  fi
  echo "Base ref $base_ref not found. Skipping API check."
  exit 0
fi

if ! command -v apidiff >/dev/null 2>&1; then
  echo "apidiff not found. Install with: make tools" >&2
  exit 1
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
  "github.com/aatuh/api-toolkit/v2/binding"
  "github.com/aatuh/api-toolkit/v2/compat/billing"
  "github.com/aatuh/api-toolkit/v2/contracttest"
  "github.com/aatuh/api-toolkit/v2/email"
  "github.com/aatuh/api-toolkit/v2/endpoints/docs"
  "github.com/aatuh/api-toolkit/v2/endpoints/health"
  "github.com/aatuh/api-toolkit/v2/endpoints/list"
  "github.com/aatuh/api-toolkit/v2/endpoints/pprof"
  "github.com/aatuh/api-toolkit/v2/endpoints/version"
  "github.com/aatuh/api-toolkit/v2/fielderrors"
  "github.com/aatuh/api-toolkit/v2/httpcache"
  "github.com/aatuh/api-toolkit/v2/httpx"
  "github.com/aatuh/api-toolkit/v2/httpx/identity"
  "github.com/aatuh/api-toolkit/v2/httpx/recover"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/apikey"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/authz"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/jwt"
  "github.com/aatuh/api-toolkit/v2/middleware/auth/tenant"
  "github.com/aatuh/api-toolkit/v2/middleware/deprecation"
  "github.com/aatuh/api-toolkit/v2/middleware/idempotency"
  "github.com/aatuh/api-toolkit/v2/middleware/json"
  "github.com/aatuh/api-toolkit/v2/middleware/maxbody"
  "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
  "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
  "github.com/aatuh/api-toolkit/v2/middleware/secure"
  "github.com/aatuh/api-toolkit/v2/middleware/timeout"
  "github.com/aatuh/api-toolkit/v2/middleware/trace"
  "github.com/aatuh/api-toolkit/v2/negotiation"
  "github.com/aatuh/api-toolkit/v2/operations"
  "github.com/aatuh/api-toolkit/v2/ports"
  "github.com/aatuh/api-toolkit/v2/queryparams"
  "github.com/aatuh/api-toolkit/v2/response_writer"
  "github.com/aatuh/api-toolkit/v2/routecontracts"
  "github.com/aatuh/api-toolkit/v2/routepolicy"
  "github.com/aatuh/api-toolkit/v2/scheduler"
  "github.com/aatuh/api-toolkit/v2/scheduler/migrations"
  "github.com/aatuh/api-toolkit/v2/securityprofile"
  "github.com/aatuh/api-toolkit/v2/specs"
  "github.com/aatuh/api-toolkit/v2/swagstub"
  "github.com/aatuh/api-toolkit/v2/webhooks"
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
