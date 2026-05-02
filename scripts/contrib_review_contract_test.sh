#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

require_failure() {
  local name="$1"
  shift
  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    printf 'expected %s to fail, but it passed\noutput:\n%s\n' "$name" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

require_success() {
  local name="$1"
  shift
  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    printf 'expected %s to pass, but it failed with %s\noutput:\n%s\n' "$name" "$status" "$output" >&2
    exit 1
  fi
  printf '%s' "$output"
}

make_fake_apidiff() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/apidiff" <<'FAKE'
#!/usr/bin/env sh
if [ "$1" = "-w" ]; then
  printf 'fake export for %s\n' "$3" >"$2"
  exit 0
fi
if [ "${FAKE_APIDIFF_INCOMPATIBLE:-}" = "1" ]; then
  printf 'Incompatible changes:\n- fake incompatible contrib drift\n'
elif [ "${FAKE_APIDIFF_COMPATIBLE:-}" = "1" ]; then
  printf 'Compatible changes:\n- fake compatible contrib drift\n'
fi
exit 0
FAKE
  chmod +x "$bin_dir/apidiff"
}

init_repo() {
  local dir="$1"
  mkdir -p "$dir/scripts" "$dir/docs" "$dir/contrib/middleware/auth/devheaders" "$dir/contrib/adapters/idempotency"
  cp "$repo_root/scripts/contrib_api_drift_report.sh" "$dir/scripts/contrib_api_drift_report.sh"
  cp "$repo_root/scripts/contrib_release_notes_check.sh" "$dir/scripts/contrib_release_notes_check.sh"
  chmod +x "$dir/scripts/"*.sh
  cat >"$dir/docs/contrib-api-drift-packages.txt" <<'MANIFEST'
github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders
MANIFEST
  cat >"$dir/docs/release-notes.md" <<'NOTES'
# Release Notes
NOTES
  cat >"$dir/docs/package-classification.tsv" <<'TSV'
# Public package classification manifest.
# Columns: import_path	api_status	test_status	notes
github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders	supported-adapter	direct-tests	Dev-header auth middleware.
github.com/aatuh/api-toolkit/contrib/v2/adapters/idempotency	experimental	direct-tests	In-memory idempotency adapter.
TSV
  cat >"$dir/contrib/middleware/auth/devheaders/devheaders.go" <<'GO'
package devheaders

type Config struct {
	Enabled bool
}
GO
  cat >"$dir/contrib/adapters/idempotency/memory.go" <<'GO'
package idempotency

type Store struct {
	Enabled bool
}
GO
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email "contrib-review-contract@example.invalid"
  git -C "$dir" config user.name "contrib review contract"
  git -C "$dir" add .
  git -C "$dir" commit -qm initial
  git -C "$dir" tag v-base
}

write_drift_disposition_manifest() {
  local dir="$1"
  local expires_on="$2"

  {
    printf 'package\tstatus\treason\trelease_note_acknowledgement\treviewed_on\texpires_on\towner\n'
    printf '%s\tincompatible\t%s\t%s\t2026-05-01\t%s\tcontrib-auth-maintainers\n' \
      "github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders" \
      "Fake incompatible drift for contract tests." \
      "docs/release-notes.md package-tied incompatible contrib acknowledgement" \
      "$expires_on"
  } >"$dir/docs/contrib-api-drift-dispositions.tsv"
}

run_script_in_dir() {
  local dir="$1"
  shift
  (cd "$dir" && env "$@")
}

fake_bin="$tmp/bin"
make_fake_apidiff "$fake_bin"

missing_dir="$tmp/missing-baseline"
init_repo "$missing_dir"
missing_output="$(require_failure missing-baseline run_script_in_dir "$missing_dir" PATH="$fake_bin:$PATH" scripts/contrib_api_drift_report.sh)"
case "$missing_output" in
  *"Set CONTRIB_API_BASE_REF or API_BASE_REF"*) ;;
  *) printf 'missing baseline failure changed unexpectedly:\n%s\n' "$missing_output" >&2; exit 1 ;;
esac

no_change_dir="$tmp/no-change"
init_repo "$no_change_dir"
no_change_output="$(require_success no-change-report run_script_in_dir "$no_change_dir" PATH="$fake_bin:$PATH" API_BASE_REF=v-base scripts/contrib_api_drift_report.sh)"
case "$no_change_output" in
  *"Package manifest: docs/contrib-api-drift-packages.txt"*) ;;
  *) printf 'no-change drift report did not name the manifest:\n%s\n' "$no_change_output" >&2; exit 1 ;;
esac
case "$no_change_output" in
  *"drift_packages=0"*) ;;
  *) printf 'no-change drift report did not report zero drift:\n%s\n' "$no_change_output" >&2; exit 1 ;;
esac
notes_no_change_output="$(require_success no-change-notes run_script_in_dir "$no_change_dir" PATH="$fake_bin:$PATH" CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$notes_no_change_output" in
  *"No supported-tier contrib adapter/integration public behavior files changed."*) ;;
  *) printf 'no-change notes output changed unexpectedly:\n%s\n' "$notes_no_change_output" >&2; exit 1 ;;
esac

experimental_dir="$tmp/experimental"
init_repo "$experimental_dir"
cat >"$experimental_dir/contrib/adapters/idempotency/memory.go" <<'GO'
package idempotency

type Store struct {
	Enabled bool
	UnsafeExperimentalKnob bool
}
GO
experimental_output="$(require_success experimental-change-notes run_script_in_dir "$experimental_dir" PATH="$fake_bin:$PATH" CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$experimental_output" in
  *"No supported-tier contrib adapter/integration public behavior files changed."*) ;;
  *) printf 'experimental notes output changed unexpectedly:\n%s\n' "$experimental_output" >&2; exit 1 ;;
esac

incompatible_dir="$tmp/incompatible"
init_repo "$incompatible_dir"
cat >"$incompatible_dir/contrib/middleware/auth/devheaders/devheaders.go" <<'GO'
package devheaders

type Config struct {
	Enabled        bool
	TrustedProxies []string
}
GO
cat >>"$incompatible_dir/docs/release-notes.md" <<'NOTES'

- Dev header behavior changed.
NOTES

missing_disposition_output="$(require_failure incompatible-missing-disposition run_script_in_dir "$incompatible_dir" PATH="$fake_bin:$PATH" FAKE_APIDIFF_INCOMPATIBLE=1 CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$missing_disposition_output" in
  *"requires docs/contrib-api-drift-dispositions.tsv"*) ;;
  *) printf 'incompatible drift failure did not require disposition manifest:\n%s\n' "$missing_disposition_output" >&2; exit 1 ;;
esac

write_drift_disposition_manifest "$incompatible_dir" "2099-01-01"
incompatible_output="$(require_failure incompatible-without-ack run_script_in_dir "$incompatible_dir" PATH="$fake_bin:$PATH" FAKE_APIDIFF_INCOMPATIBLE=1 CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$incompatible_output" in
  *"requires an explicit release note"*) ;;
  *) printf 'incompatible drift failure did not require explicit acknowledgement:\n%s\n' "$incompatible_output" >&2; exit 1 ;;
esac

cat >>"$incompatible_dir/docs/release-notes.md" <<'NOTES'
- Incompatible contrib drift is acknowledged for this release.
NOTES
generic_ack_output="$(require_failure incompatible-generic-ack run_script_in_dir "$incompatible_dir" PATH="$fake_bin:$PATH" FAKE_APIDIFF_INCOMPATIBLE=1 CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$generic_ack_output" in
  *"requires release notes tied to package"*) ;;
  *) printf 'generic incompatible drift acknowledgement unexpectedly passed:\n%s\n' "$generic_ack_output" >&2; exit 1 ;;
esac

cat >>"$incompatible_dir/docs/release-notes.md" <<'NOTES'
- Incompatible contrib drift for contrib/middleware/auth/devheaders is acknowledged for this release.
NOTES

write_drift_disposition_manifest "$incompatible_dir" "2000-01-01"
expired_output="$(require_failure incompatible-expired-disposition run_script_in_dir "$incompatible_dir" PATH="$fake_bin:$PATH" FAKE_APIDIFF_INCOMPATIBLE=1 CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$expired_output" in
  *"must be owned and non-expired"*) ;;
  *) printf 'expired incompatible drift disposition unexpectedly passed:\n%s\n' "$expired_output" >&2; exit 1 ;;
esac

write_drift_disposition_manifest "$incompatible_dir" "2099-01-01"
ack_output="$(require_success incompatible-with-ack run_script_in_dir "$incompatible_dir" PATH="$fake_bin:$PATH" FAKE_APIDIFF_INCOMPATIBLE=1 CONTRIB_RELEASE_BASE_REF=v-base scripts/contrib_release_notes_check.sh)"
case "$ack_output" in
  *"Incompatible contrib API drift has package-tied release-note acknowledgement and non-expired disposition coverage."*) ;;
  *) printf 'incompatible drift acknowledgement output changed unexpectedly:\n%s\n' "$ack_output" >&2; exit 1 ;;
esac
