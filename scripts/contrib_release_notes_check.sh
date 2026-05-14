#!/usr/bin/env bash
# Review gate: supported contrib adapter/integration/middleware/tooling behavior
# changes need release notes or explicit no-user-impact rationale. Production
# generator behavior and full-profile runtime assets under contrib/cmd/api-toolkit
# are included, including *.tmpl and *.yaml templates.
set -euo pipefail

base_ref="${CONTRIB_RELEASE_BASE_REF:-${API_BASE_REF:-HEAD~1}}"
classification_manifest="docs/package-classification.tsv"
if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  echo "Base ref $base_ref not found; skipping contrib release notes check."
  exit 0
fi

is_release_note_candidate_path() {
  local path="$1"
  case "$path" in
    *_test.go|*/doc.go|*.md)
      return 1
      ;;
    *.go|*.sql|*.json|*.yaml|*.yml|*.toml|*.tmpl|*.tpl|*.html|*.txt|*.csv|*.cue|*.rego|*.graphql|*.graphqls)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

reviewed_import_path_for_changed_path() {
  local path="$1"
  local package_path="${path%/*}"
  local import_path

  while [ -n "$package_path" ] && [ "$package_path" != "." ] && [ "$package_path" != "contrib" ]; do
    case "$package_path" in
      contrib/*)
        import_path="github.com/aatuh/api-toolkit/contrib/v2/${package_path#contrib/}"
        ;;
      *)
        import_path="github.com/aatuh/api-toolkit/${package_path}"
        ;;
    esac
    if awk -F '\t' -v import_path="$import_path" '
      $1 == import_path && $2 == "supported-adapter" { found = 1 }
      $1 == "github.com/aatuh/api-toolkit/contrib/v2/cmd/api-toolkit" && $1 == import_path && $2 == "tooling" { found = 1 }
      END { exit found ? 0 : 1 }
    ' "$classification_manifest"; then
      printf '%s\n' "$import_path"
      return 0
    fi
    case "$package_path" in
      */*) package_path="${package_path%/*}" ;;
      *) break ;;
    esac
  done
  return 1
}

changed_contrib_candidates="$(git diff --name-only "$base_ref" -- \
  'contrib/adapters' \
  'contrib/integrations' \
  'contrib/bootstrap' \
  'contrib/cmd/api-toolkit' \
  'contrib/middleware/auth/clerk' \
  'contrib/middleware/auth/devheaders' \
  'contrib/middleware/cors' \
  'contrib/middleware/metrics' \
  'contrib/middleware/openapi' \
  'contrib/middleware/oteltrace' \
  'contrib/middleware/requestlog' \
  'contrib/telemetry' \
  | while IFS= read -r path; do
      if is_release_note_candidate_path "$path"; then
        printf '%s\n' "$path"
      fi
    done || true)"

changed_contrib=""
if [ -n "$changed_contrib_candidates" ]; then
  if [ ! -f "$classification_manifest" ]; then
    echo "Contrib release notes check requires $classification_manifest." >&2
    exit 1
  fi
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    if import_path="$(reviewed_import_path_for_changed_path "$path")"; then
      changed_contrib="${changed_contrib}${path}"$'\n'
    fi
  done <<< "$changed_contrib_candidates"
  changed_contrib="$(printf '%s' "$changed_contrib" | sed '/^$/d' || true)"
fi

if [ -z "$changed_contrib" ]; then
  echo "No supported-tier contrib public behavior files changed."
  exit 0
fi

release_notes_diff="$(git diff "$base_ref" -- docs/release-notes.md || true)"
if [ -z "$release_notes_diff" ]; then
  echo "Contrib supported-tier behavior files changed without docs/release-notes.md updates:" >&2
  printf '%s\n' "$changed_contrib" >&2
  exit 1
fi

set +e
drift_output="$(CONTRIB_API_BASE_REF="$base_ref" API_BASE_REF="$base_ref" scripts/contrib_api_drift_report.sh 2>&1)"
drift_status=$?
set -e
if [ "$drift_status" -ne 0 ]; then
  if printf '%s\n' "$drift_output" | grep -Fq "Report complete:"; then
    :
  else
  echo "Could not generate contrib API drift report for release-note review:" >&2
  printf '%s\n' "$drift_output" >&2
  exit "$drift_status"
  fi
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

echo "Contrib supported-tier changes have release notes coverage."
