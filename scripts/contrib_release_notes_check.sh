#!/usr/bin/env bash
# Review gate: contrib adapter/integration public behavior changes need release notes.
set -euo pipefail

base_ref="${CONTRIB_RELEASE_BASE_REF:-${API_BASE_REF:-HEAD~1}}"
if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  echo "Base ref $base_ref not found; skipping contrib release notes check."
  exit 0
fi

changed_contrib="$(git diff --name-only "$base_ref" -- \
  'contrib/adapters' \
  'contrib/integrations' \
  'contrib/middleware/auth/clerk' \
  'contrib/middleware/auth/devheaders' \
  | grep -E '\.go$' \
  | grep -Ev '(^|/)([^/]+_test\.go|doc\.go)$' || true)"

if [ -z "$changed_contrib" ]; then
  echo "No contrib adapter/integration public behavior files changed."
  exit 0
fi

release_notes_diff="$(git diff "$base_ref" -- docs/release-notes.md || true)"
if [ -z "$release_notes_diff" ]; then
  echo "Contrib adapter/integration files changed without docs/release-notes.md updates:" >&2
  printf '%s\n' "$changed_contrib" >&2
  exit 1
fi

set +e
drift_output="$(CONTRIB_API_BASE_REF="$base_ref" API_BASE_REF="$base_ref" scripts/contrib_api_drift_report.sh 2>&1)"
drift_status=$?
set -e
if [ "$drift_status" -ne 0 ]; then
  echo "Could not generate contrib API drift report for release-note review:" >&2
  printf '%s\n' "$drift_output" >&2
  exit "$drift_status"
fi

incompatible_drift_count="$(printf '%s\n' "$drift_output" | awk '{
  for (i = 1; i <= NF; i++) {
    split($i, part, "=")
    if (part[1] == "incompatible_drift_packages") {
      gsub(/[^0-9].*/, "", part[2])
      print part[2]
    }
  }
}' | tail -n 1)"
incompatible_drift_count="${incompatible_drift_count:-0}"

if [ "$incompatible_drift_count" -gt 0 ]; then
  incompatible_packages="$(printf '%s\n' "$drift_output" | awk '
    /^DRIFT / { pkg=$2 }
    /^Incompatible changes:/ && pkg != "" { print pkg }
  ' | sort -u)"
  disposition_manifest="docs/contrib-api-drift-dispositions.tsv"
  if [ ! -f "$disposition_manifest" ]; then
    echo "Incompatible report-only contrib API drift requires $disposition_manifest." >&2
    exit 1
  fi
  if ! printf '%s\n' "$release_notes_diff" | grep -Eiq 'incompatible.*contrib|contrib.*incompatible'; then
    echo "Incompatible report-only contrib API drift requires an explicit release note or upgrade note acknowledgement." >&2
    echo "This remains a review signal and does not make contrib part of the stable API promise." >&2
    exit 1
  fi
  while IFS= read -r pkg; do
    if [ -z "$pkg" ]; then
      continue
    fi
    pkg_suffix="${pkg#github.com/aatuh/api-toolkit/contrib/v2/}"
    if ! printf '%s\n' "$release_notes_diff" | grep -Fq "$pkg" && \
       ! printf '%s\n' "$release_notes_diff" | grep -Fq "$pkg_suffix"; then
      echo "Incompatible report-only contrib API drift requires release notes tied to package $pkg." >&2
      echo "Mention the full import path or contrib-relative package path alongside the incompatible contrib acknowledgement." >&2
      exit 1
    fi
    disposition_row="$(awk -F '\t' -v pkg="$pkg" '
      NR == 1 {
        for (i = 1; i <= NF; i++) {
          header[$i] = i
        }
        next
      }
      $header["package"] == pkg {
        print $header["status"] "\t" $header["expires_on"] "\t" $header["release_note_acknowledgement"] "\t" $header["owner"]
      }
    ' "$disposition_manifest")"
    if [ -z "$disposition_row" ]; then
      echo "Incompatible report-only contrib API drift requires a disposition row for package $pkg." >&2
      exit 1
    fi
    IFS=$'\t' read -r disposition_status expires_on release_note_ack owner <<< "$disposition_row"
    review_date="$(date -u +%Y-%m-%d)"
    if [ "$disposition_status" != "incompatible" ]; then
      echo "Disposition row for $pkg must have status=incompatible, got $disposition_status." >&2
      exit 1
    fi
    if [ -z "$owner" ] || [ -z "$expires_on" ] || [ "$expires_on" \< "$review_date" ] || [ "$expires_on" = "$review_date" ]; then
      echo "Disposition row for $pkg must be owned and non-expired after $review_date." >&2
      exit 1
    fi
    if [ -z "$release_note_ack" ] || [ "$release_note_ack" = "not_required" ]; then
      echo "Disposition row for $pkg must reference package-tied release notes." >&2
      exit 1
    fi
  done <<< "$incompatible_packages"
  echo "Incompatible contrib API drift has package-tied release-note acknowledgement and non-expired disposition coverage."
fi

echo "Contrib adapter/integration changes have release notes coverage."
