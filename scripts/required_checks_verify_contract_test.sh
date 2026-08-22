#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
subject="$repo_root/scripts/required_checks_verify.sh"
governance_subject="$repo_root/scripts/github_governance_check.sh"

if [ ! -x "$subject" ] || [ ! -x "$governance_subject" ]; then
  echo "required-checks contract: executable verifier or governance audit is missing" >&2
  exit 1
fi

fixture_root="$(mktemp -d)"
trap 'rm -rf "$fixture_root"' EXIT
mkdir -p "$fixture_root/scripts" "$fixture_root/docs" "$fixture_root/.github/workflows" "$fixture_root/bin"
cp "$subject" "$fixture_root/scripts/required_checks_verify.sh"
cp "$governance_subject" "$fixture_root/scripts/github_governance_check.sh"

write_workflow() {
  cat >"$fixture_root/.github/workflows/ci.yml" <<'YAML'
name: ci
jobs:
  test:
    name: test (${{ matrix.go-version }})
    runs-on: ubuntu-latest
  lint:
    name: lint
    runs-on: ubuntu-latest
  release-preflight:
    name: release-preflight
    runs-on: ubuntu-latest
YAML
}

write_renamed_workflow() {
  cat >"$fixture_root/.github/workflows/ci.yml" <<'YAML'
name: ci
jobs:
  renamed-test:
    name: test (${{ matrix.go-version }})
    runs-on: ubuntu-latest
  lint:
    name: lint
    runs-on: ubuntu-latest
  release-preflight:
    name: release-preflight
    runs-on: ubuntu-latest
YAML
}

write_manifest() {
  cat >"$fixture_root/docs/required-checks.json" <<'JSON'
[
  {
    "check_name": "test (1.26.x)",
    "workflow_file": ".github/workflows/ci.yml",
    "job_id": "test",
    "job_name": "test (${{ matrix.go-version }})",
    "app_id": 15368,
    "required_for_pr": true,
    "required_for_release": true,
    "owner": "test-engineering"
  },
  {
    "check_name": "lint",
    "workflow_file": ".github/workflows/ci.yml",
    "job_id": "lint",
    "job_name": "lint",
    "app_id": 15368,
    "required_for_pr": true,
    "required_for_release": true,
    "owner": "build-engineering"
  },
  {
    "check_name": "release-preflight",
    "workflow_file": ".github/workflows/ci.yml",
    "job_id": "release-preflight",
    "job_name": "release-preflight",
    "app_id": 15368,
    "required_for_pr": false,
    "required_for_release": true,
    "owner": "release-engineering"
  }
]
JSON
}

write_protection() {
  cat >"$fixture_root/protection.json" <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "checks": [
      {"context": "lint", "app_id": 15368},
      {"context": "test (1.26.x)", "app_id": 15368}
    ]
  }
}
JSON
}

expect_pass() {
  local description="$1"
  shift
  if ! "$@" >"$fixture_root/stdout" 2>"$fixture_root/stderr"; then
    echo "required-checks contract: expected pass: $description" >&2
    cat "$fixture_root/stdout" >&2
    cat "$fixture_root/stderr" >&2
    exit 1
  fi
}

expect_fail() {
  local description="$1"
  shift
  if "$@" >"$fixture_root/stdout" 2>"$fixture_root/stderr"; then
    echo "required-checks contract: expected failure: $description" >&2
    cat "$fixture_root/stdout" >&2
    exit 1
  fi
}

verify_branch_fixture() {
  "$fixture_root/scripts/required_checks_verify.sh" --branch-protection - <"$fixture_root/protection.json"
}

write_workflow
write_manifest
write_protection
expect_pass "valid local manifest" "$fixture_root/scripts/required_checks_verify.sh"
expect_pass "exact app-bound branch protection" verify_branch_fixture

write_renamed_workflow
expect_fail "renamed workflow job" "$fixture_root/scripts/required_checks_verify.sh"
write_workflow

cat >"$fixture_root/docs/required-checks.json" <<'JSON'
[
  {
    "check_name": "lint",
    "workflow_file": "../ci.yml",
    "job_id": "lint",
    "job_name": "lint",
    "app_id": 15368,
    "required_for_pr": true,
    "required_for_release": true,
    "owner": "build-engineering"
  }
]
JSON
expect_fail "non-canonical workflow path" "$fixture_root/scripts/required_checks_verify.sh"

write_manifest
cat >"$fixture_root/protection.json" <<'JSON'
{"required_status_checks":{"strict":true,"checks":[{"context":"lint","app_id":15368}]}}
JSON
expect_fail "missing required branch check" verify_branch_fixture

cat >"$fixture_root/protection.json" <<'JSON'
{"required_status_checks":{"strict":true,"checks":[{"context":"lint","app_id":15368},{"context":"test (1.26.x)","app_id":15368},{"context":"stale-check","app_id":15368}]}}
JSON
expect_fail "unmanifested branch check" verify_branch_fixture

cat >"$fixture_root/protection.json" <<'JSON'
{"required_status_checks":{"strict":true,"checks":[{"context":"lint","app_id":1},{"context":"test (1.26.x)","app_id":15368}]}}
JSON
expect_fail "wrong check app binding" verify_branch_fixture

cat >"$fixture_root/protection.json" <<'JSON'
{"required_status_checks":{"strict":false,"checks":[{"context":"lint","app_id":15368},{"context":"test (1.26.x)","app_id":15368}]}}
JSON
expect_fail "non-strict branch checks" verify_branch_fixture

printf '{not-json' >"$fixture_root/protection.json"
expect_fail "malformed provider response" verify_branch_fixture

expect_fail "unsupported verifier argument" "$fixture_root/scripts/required_checks_verify.sh" --manifest "$fixture_root/docs/required-checks.json"

cat >"$fixture_root/bin/gh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

mode="${GH_FAKE_MODE:-success}"
if [ "${1:-}" = auth ] && [ "${2:-}" = status ]; then
  exit 0
fi
if [ "${1:-}" != api ] || [ "${2:-}" != --method ] || [ "${3:-}" != GET ] || [ "$#" -ne 4 ]; then
  exit 2
fi
if [ "$mode" = unavailable ]; then
  exit 1
fi

case "$4" in
  repos/example/repo/branches/master)
    printf '%s' '{"protected":true}'
    ;;
  repos/example/repo/branches/master/protection)
    if [ "$mode" = malformed ]; then
      printf '%s' '{"private":"super-secret-provider-payload"'
    else
      printf '%s' '{"required_status_checks":{"strict":true,"checks":[{"context":"lint","app_id":15368},{"context":"test (1.26.x)","app_id":15368}]},"enforce_admins":{"enabled":true},"required_linear_history":{"enabled":true},"allow_force_pushes":{"enabled":false},"allow_deletions":{"enabled":false}}'
    fi
    ;;
  'repos/example/repo/rulesets?includes_parents=true&per_page=100')
    printf '%s' '[{"id":1},{"id":2}]'
    ;;
  repos/example/repo/rulesets/1)
    printf '%s' '{"id":1,"target":"branch","conditions":{"ref_name":{"include":["refs/heads/master"]}},"bypass_actors":[],"rules":[{"type":"pull_request","parameters":{"required_approving_review_count":0,"require_code_owner_review":false,"require_last_push_approval":false,"required_review_thread_resolution":true}},{"type":"code_scanning","parameters":{"code_scanning_tools":[{"tool":"CodeQL","alerts_threshold":"errors_and_warnings","security_alerts_threshold":"high_or_higher"}]}}]}'
    ;;
  repos/example/repo/rulesets/2)
    printf '%s' '{"id":2,"target":"tag","conditions":{"ref_name":{"include":["refs/tags/v*","refs/tags/contrib/v*"]}},"bypass_actors":[],"rules":[]}'
    ;;
  *)
    exit 1
    ;;
esac
SH
chmod +x "$fixture_root/bin/gh"

verify_governance_fixture() {
  local mode="$1"
  GH_FAKE_MODE="$mode" \
    GITHUB_REPOSITORY=example/repo \
    GITHUB_DEFAULT_BRANCH=master \
    PATH="$fixture_root/bin:$PATH" \
    "$fixture_root/scripts/github_governance_check.sh"
}

verify_invalid_repository() {
  GH_FAKE_MODE=success \
    GITHUB_REPOSITORY=../example/repo \
    GITHUB_DEFAULT_BRANCH=master \
    PATH="$fixture_root/bin:$PATH" \
    "$fixture_root/scripts/github_governance_check.sh"
}

write_workflow
write_manifest
expect_pass "authenticated provider fixture" verify_governance_fixture success
expect_fail "authenticated provider unavailable" verify_governance_fixture unavailable
expect_fail "authenticated provider malformed response" verify_governance_fixture malformed
if grep -Fq 'super-secret-provider-payload' "$fixture_root/stdout" "$fixture_root/stderr"; then
  echo "required-checks contract: governance audit disclosed a raw provider payload" >&2
  exit 1
fi
expect_fail "invalid repository input" verify_invalid_repository

echo "required-checks contract tests passed"
