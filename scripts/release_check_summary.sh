#!/usr/bin/env bash
set -uo pipefail

run_checks=false
if [ "${1:-}" = "--run" ]; then
  run_checks=true
fi

api_base_ref="${API_BASE_REF:-}"
if [ -z "$api_base_ref" ]; then
  echo "API_BASE_REF is required to write release check summary" >&2
  exit 2
fi

gotoolchain="${GOTOOLCHAIN:-local}"
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
commit="$(git rev-parse HEAD 2>/dev/null || printf 'unknown')"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
evidence_dir="${RELEASE_EVIDENCE_DIR:-.ci-result/release-evidence}"
log_dir="$evidence_dir/logs"
release_logs_archive_path="$evidence_dir/release-evidence-logs.tgz"
allow_dirty_release_evidence=false
case "${ALLOW_DIRTY_RELEASE_EVIDENCE:-}" in
  1|true|TRUE|yes|YES) allow_dirty_release_evidence=true ;;
esac
evidence_mode="publication"
if [ "$allow_dirty_release_evidence" = true ]; then
  evidence_mode="local_audit"
fi
git_status_porcelain="$(git -C "$repo_root" status --porcelain --untracked-files=normal 2>/dev/null || true)"
git_dirty=false
if [ -n "$git_status_porcelain" ]; then
  git_dirty=true
fi

check_names=(
  "tools"
  "lint"
  "vuln"
  "gosec"
  "ci-build-smoke"
  "release-api-check"
  "contrib-release-notes-check"
  "docs-check"
  "test"
  "test-race"
  "fuzz"
  "clean"
)
check_commands=(
  "make tools"
  "make lint"
  "make vuln"
  "make gosec"
  "make ci-build-smoke"
  "make release-api-check"
  "make contrib-release-notes-check"
  "make docs-check"
  "make test"
  "make test-race"
  "make fuzz"
  "make clean"
)

json_escape() {
  local s="${1-}"
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\n'/\\n}
  s=${s//$'\r'/\\r}
  s=${s//$'\t'/\\t}
  printf '%s' "$s"
}

json_string() {
  printf '"%s"' "$(json_escape "$1")"
}

json_string_array() {
  local first=true
  printf '['
  for value in "$@"; do
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    json_string "$value"
  done
  printf ']'
}

git_state_json() {
  local branch=""
  local detached=false
  local status=""
  local dirty=false
  local staged_count=0
  local unstaged_count=0
  local untracked_count=0
  local deleted_count=0
  local line x y

  if branch="$(git -C "$repo_root" symbolic-ref --quiet --short HEAD 2>/dev/null)"; then
    detached=false
  else
    branch=""
    detached=true
  fi

  status="$git_status_porcelain"
  if [ -n "$status" ]; then
    dirty=true
  fi

  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    x="${line:0:1}"
    y="${line:1:1}"
    if [ "$x" = "?" ] && [ "$y" = "?" ]; then
      untracked_count=$((untracked_count + 1))
      continue
    fi
    if [ "$x" != " " ] && [ "$x" != "!" ]; then
      staged_count=$((staged_count + 1))
    fi
    if [ "$y" != " " ] && [ "$y" != "!" ]; then
      unstaged_count=$((unstaged_count + 1))
    fi
    if [ "$x" = "D" ] || [ "$y" = "D" ]; then
      deleted_count=$((deleted_count + 1))
    fi
  done <<< "$status"

  printf '{'
  printf '"commit":'; json_string "$commit"; printf ','
  printf '"branch":'
  if [ -n "$branch" ]; then
    json_string "$branch"
  else
    printf 'null'
  fi
  printf ','
  printf '"detached":%s,' "$detached"
  printf '"dirty":%s,' "$dirty"
  printf '"staged_count":%s,' "$staged_count"
  printf '"unstaged_count":%s,' "$unstaged_count"
  printf '"untracked_count":%s,' "$untracked_count"
  printf '"deleted_count":%s' "$deleted_count"
  printf '}'
}

provenance_policy_json() {
  local status="passed"
  local message="Publication release evidence requires a clean git worktree."

  if [ "$git_dirty" = true ] && [ "$allow_dirty_release_evidence" != true ]; then
    status="failed"
    message="Dirty worktree is not acceptable for publication release evidence. Clean the tree or set ALLOW_DIRTY_RELEASE_EVIDENCE=1 only for local audit evidence."
  elif [ "$git_dirty" = true ]; then
    status="allowed_dirty_local_audit"
    message="Dirty worktree evidence was explicitly allowed for local audit only and is not acceptable before publishing."
  fi

  printf '{'
  printf '"mode":'; json_string "$evidence_mode"; printf ','
  printf '"allow_dirty_release_evidence":%s,' "$allow_dirty_release_evidence"
  printf '"status":'; json_string "$status"; printf ','
  printf '"message":'; json_string "$message"
  printf '}'
}

check_json() {
  local name="$1"
  local command_line="$2"
  local status="$3"
  local exit_code="$4"
  local duration_ms="$5"
  local log_path="$6"
  local log_available="$7"
  local exit_json="null"
  local duration_json="null"

  if [ -n "$exit_code" ]; then
    exit_json="$exit_code"
  fi
  if [ -n "$duration_ms" ]; then
    duration_json="$duration_ms"
  fi

  printf '{'
  printf '"name":'; json_string "$name"; printf ','
  printf '"command_line":'; json_string "API_BASE_REF=$api_base_ref GOTOOLCHAIN=$gotoolchain $command_line"; printf ','
  printf '"status":'; json_string "$status"; printf ','
  printf '"exit_code":%s,' "$exit_json"
  printf '"duration_ms":%s,' "$duration_json"
  printf '"log_available":%s,' "$log_available"
  printf '"log_path":'; json_string "$log_path"; printf ','
  printf '"artifacts":[]'
  printf '}'
}

run_check() {
  local name="$1"
  local command_line="$2"
  local log_path="$log_dir/$name.log"
  local log_abs="$repo_root/$log_path"
  local start_ns end_ns duration_ms exit_code

  mkdir -p "$(dirname "$log_abs")"
  start_ns="$(date +%s%N)"
  (
    cd "$repo_root" && API_BASE_REF="$api_base_ref" GOTOOLCHAIN="$gotoolchain" bash -c "$command_line"
  ) >"$log_abs" 2>&1
  exit_code=$?
  end_ns="$(date +%s%N)"
  duration_ms=$(((end_ns - start_ns) / 1000000))

  if [ "$exit_code" -eq 0 ]; then
    check_json "$name" "$command_line" "passed" "$exit_code" "$duration_ms" "$log_path" "true"
    return 0
  fi

  check_json "$name" "$command_line" "failed" "$exit_code" "$duration_ms" "$log_path" "true"
  return "$exit_code"
}

extract_report_count() {
  local log_path="$1"
  local key="$2"
  local value=""

  if [ -n "$log_path" ] && [ -f "$repo_root/$log_path" ]; then
    value="$(awk -v key="$key" '{
      for (i = 1; i <= NF; i++) {
        split($i, part, "=")
        if (part[1] == key) {
          gsub(/[^0-9].*/, "", part[2])
          print part[2]
        }
      }
    }' "$repo_root/$log_path" | tail -n 1)"
  fi
  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf 'null'
  fi
}

contrib_drift_json() {
  local status="$1"
  local exit_code="$2"
  local duration_ms="$3"
  local log_path="$4"
  local log_available="$5"
  local exit_json="null"
  local duration_json="null"
  local packages
  local review_date
  local disposition_issues
  local missing_count
  local expired_count

  if [ -n "$exit_code" ]; then
    exit_json="$exit_code"
  fi
  if [ -n "$duration_ms" ]; then
    duration_json="$duration_ms"
  fi
  packages="$(contrib_drift_packages_from_log "$log_path")"
  review_date="$(release_review_date)"
  disposition_issues="$(contrib_disposition_issues "$packages" "$review_date")"
  missing_count="$(count_issue_status "$disposition_issues" "missing")"
  expired_count="$(count_issue_status "$disposition_issues" "expired")"

  printf '{'
  printf '"command_line":'; json_string "API_BASE_REF=$api_base_ref GOTOOLCHAIN=$gotoolchain make contrib-api-drift-report"; printf ','
  printf '"status":'; json_string "$status"; printf ','
  printf '"exit_code":%s,' "$exit_json"
  printf '"duration_ms":%s,' "$duration_json"
  printf '"log_available":%s,' "$log_available"
  printf '"artifact_path":'; json_string "$log_path"; printf ','
  printf '"disposition_manifest_path":'; json_string "docs/contrib-api-drift-dispositions.tsv"; printf ','
  printf '"review_date":'; json_string "$review_date"; printf ','
  printf '"drift_package_count":%s,' "$(extract_report_count "$log_path" "drift_packages")"
  printf '"skipped_package_count":%s,' "$(extract_report_count "$log_path" "skipped_packages")"
  printf '"compatible_drift_count":%s,' "$(extract_report_count "$log_path" "compatible_drift_packages")"
  printf '"incompatible_drift_count":%s,' "$(extract_report_count "$log_path" "incompatible_drift_packages")"
  printf '"packages":'
  drift_packages_json "$packages"
  printf ','
  printf '"missing_disposition_count":%s,' "$missing_count"
  printf '"expired_disposition_count":%s,' "$expired_count"
  printf '"disposition_issues":'
  disposition_issues_json "$disposition_issues"
  printf '}'
}

run_contrib_drift_report() {
  local log_path="$log_dir/contrib-api-drift-report.log"
  local log_abs="$repo_root/$log_path"
  local start_ns end_ns duration_ms exit_code

  mkdir -p "$(dirname "$log_abs")"
  start_ns="$(date +%s%N)"
  (
    cd "$repo_root" && API_BASE_REF="$api_base_ref" GOTOOLCHAIN="$gotoolchain" make contrib-api-drift-report
  ) >"$log_abs" 2>&1
  exit_code=$?
  end_ns="$(date +%s%N)"
  duration_ms=$(((end_ns - start_ns) / 1000000))

  if [ "$exit_code" -eq 0 ]; then
    contrib_drift_json "passed" "$exit_code" "$duration_ms" "$log_path" "true"
    return 0
  fi

  contrib_drift_json "failed" "$exit_code" "$duration_ms" "$log_path" "true"
  return "$exit_code"
}

archive_release_evidence_logs() {
  local archive_abs="$repo_root/$release_logs_archive_path"

  if [ "$run_checks" != true ] || [ ! -d "$repo_root/$log_dir" ]; then
    return 0
  fi
  if ! command -v tar >/dev/null 2>&1; then
    return 0
  fi

  mkdir -p "$(dirname "$archive_abs")"
  (cd "$repo_root/$log_dir" && tar -czf "$archive_abs" .)
}

json_string_array_from_lines() {
  local first=true
  printf '['
  while IFS= read -r value; do
    if [ -z "$value" ]; then
      continue
    fi
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    json_string "$value"
  done
  printf ']'
}

release_review_date() {
  printf '%s' "${created_at%%T*}"
}

vulnerability_ids_from_log() {
  local log_path="$1"
  local log_abs="$repo_root/$log_path"

  if [ ! -f "$log_abs" ]; then
    return 0
  fi
  sed -nE 's/^[[:space:]]*Vulnerability #[0-9]+: (GO-[0-9-]+).*/\1/p' "$log_abs" | sort -u
}

govulncheck_imported_count_from_log() {
  local log_path="$1"
  local log_abs="$repo_root/$log_path"
  local compact
  local imported_count

  if [ ! -f "$log_abs" ]; then
    printf 'null'
    return 0
  fi
  compact="$(tr '\n' ' ' < "$log_abs")"
  imported_count="$(printf '%s' "$compact" | sed -nE 's/.*found ([0-9]+) vulnerabilities in packages you import.*/\1/p')"
  if [ -n "$imported_count" ]; then
    printf '%s' "$imported_count"
  else
    printf 'null'
  fi
}

vulnerability_parser_issues() {
  local log_path="$1"
  local ids="$2"
  local imported_count

  imported_count="$(govulncheck_imported_count_from_log "$log_path")"
  case "$imported_count" in
    ''|null) return 0 ;;
    *[!0-9]*) return 0 ;;
  esac
  if [ "$imported_count" -gt 0 ] && [ -z "$ids" ]; then
    printf 'govulncheck-imported-id-parser\tmissing\t\t\timported-not-called vulnerability count is positive but no GO advisory IDs were parsed from govulncheck output\n'
  fi
}

count_issue_status() {
  local issues="$1"
  local status="$2"

  printf '%s\n' "$issues" | awk -F '\t' -v status="$status" '
    NF > 1 && $2 == status { count++ }
    END { print count + 0 }
  '
}

disposition_issues_json() {
  local issues="$1"
  local first=true
  local id status expires_on owner message

  printf '['
  while IFS=$'\t' read -r id status expires_on owner message; do
    if [ -z "$id" ]; then
      continue
    fi
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    printf '{'
    printf '"id":'; json_string "$id"; printf ','
    printf '"status":'; json_string "$status"; printf ','
    printf '"expires_on":'; json_string "$expires_on"; printf ','
    printf '"owner":'; json_string "$owner"; printf ','
    printf '"message":'; json_string "$message"
    printf '}'
  done <<< "$issues"
  printf ']'
}

vulnerability_disposition_issues() {
  local ids="$1"
  local review_date="$2"
  local manifest="$repo_root/docs/vulnerability-dispositions.tsv"

  if [ -z "$ids" ]; then
    return 0
  fi
  if [ ! -f "$manifest" ]; then
    printf '%s\n' "$ids" | awk -v manifest="docs/vulnerability-dispositions.tsv" '
      NF > 0 { printf "%s\tmissing\t\t\t%s is missing\n", $1, manifest }
    '
    return 0
  fi

  awk -F '\t' -v ids="$ids" -v review_date="$review_date" '
    BEGIN {
      split(ids, wanted_lines, "\n")
      for (i in wanted_lines) {
        if (wanted_lines[i] != "") {
          wanted[wanted_lines[i]] = 1
        }
      }
    }
    NR == 1 {
      for (i = 1; i <= NF; i++) {
        header[$i] = i
      }
      next
    }
    {
      id = $header["vulnerability_id"]
      if (!(id in wanted)) {
        next
      }
      seen[id] = 1
      owner = $header["owner"]
      expires_on = $header["expires_on"]
      called_status = $header["called_status"]
      reviewed_on = $header["reviewed_on"]
      upgrade_trigger = $header["upgrade_trigger"]
      if (owner == "" || expires_on == "" || reviewed_on == "" || upgrade_trigger == "" || called_status != "imported_not_called") {
        printf "%s\tmissing\t%s\t%s\tdisposition row is incomplete or not marked imported_not_called\n", id, expires_on, owner
        next
      }
      if (expires_on <= review_date) {
        printf "%s\texpired\t%s\t%s\tdisposition expired on or before release review date %s\n", id, expires_on, owner, review_date
      }
    }
    END {
      for (id in wanted) {
        if (!(id in seen)) {
          printf "%s\tmissing\t\t\tmissing disposition row\n", id
        }
      }
    }
  ' "$manifest"
}

drift_packages_json() {
  local packages="$1"
  local first=true
  local pkg status

  printf '['
  while IFS=$'\t' read -r pkg status; do
    if [ -z "$pkg" ]; then
      continue
    fi
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    printf '{'
    printf '"package":'; json_string "$pkg"; printf ','
    printf '"status":'; json_string "$status"
    printf '}'
  done <<< "$packages"
  printf ']'
}

contrib_drift_packages_from_log() {
  local log_path="$1"
  local log_abs="$repo_root/$log_path"

  if [ ! -f "$log_abs" ]; then
    return 0
  fi
  awk '
    /^DRIFT / {
      if (pkg != "") {
        print pkg "\t" status
      }
      pkg = $2
      status = "unknown"
      next
    }
    /^Compatible changes:/ && pkg != "" {
      status = "compatible"
      next
    }
    /^Incompatible changes:/ && pkg != "" {
      status = "incompatible"
      next
    }
    END {
      if (pkg != "") {
        print pkg "\t" status
      }
    }
  ' "$log_abs"
}

contrib_disposition_issues() {
  local packages="$1"
  local review_date="$2"
  local manifest="$repo_root/docs/contrib-api-drift-dispositions.tsv"

  if [ -z "$packages" ]; then
    return 0
  fi
  if [ ! -f "$manifest" ]; then
    printf '%s\n' "$packages" | awk -F '\t' -v manifest="docs/contrib-api-drift-dispositions.tsv" '
      NF > 0 { printf "%s\tmissing\t\t\t%s is missing\n", $1, manifest }
    '
    return 0
  fi

  awk -F '\t' -v packages="$packages" -v review_date="$review_date" '
    BEGIN {
      split(packages, package_lines, "\n")
      for (i in package_lines) {
        if (package_lines[i] == "") {
          continue
        }
        split(package_lines[i], parts, "\t")
        wanted[parts[1]] = parts[2]
      }
    }
    NR == 1 {
      for (i = 1; i <= NF; i++) {
        header[$i] = i
      }
      next
    }
    {
      pkg = $header["package"]
      if (!(pkg in wanted)) {
        next
      }
      seen[pkg] = 1
      status = $header["status"]
      owner = $header["owner"]
      expires_on = $header["expires_on"]
      reason = $header["reason"]
      reviewed_on = $header["reviewed_on"]
      release_note = $header["release_note_acknowledgement"]
      if (status != wanted[pkg]) {
        printf "%s\tmissing\t%s\t%s\tdisposition status %s does not match current drift status %s\n", pkg, expires_on, owner, status, wanted[pkg]
        next
      }
      if (owner == "" || expires_on == "" || reviewed_on == "" || reason == "") {
        printf "%s\tmissing\t%s\t%s\tdisposition row is incomplete\n", pkg, expires_on, owner
        next
      }
      if (expires_on <= review_date) {
        printf "%s\texpired\t%s\t%s\tdisposition expired on or before release review date %s\n", pkg, expires_on, owner, review_date
      }
      if (wanted[pkg] == "incompatible" && (release_note == "" || release_note == "not_required")) {
        printf "%s\tmissing\t%s\t%s\tincompatible drift disposition must reference package-tied release notes\n", pkg, expires_on, owner
      }
    }
    END {
      for (pkg in wanted) {
        if (!(pkg in seen)) {
          printf "%s\tmissing\t\t\tmissing disposition row\n", pkg
        }
      }
    }
  ' "$manifest"
}

checksum_asset_json() {
  local label="$1"
  local path="$2"
  local abs="$repo_root/$path"
  local status="missing"
  local sha=""

  if [ -f "$abs" ]; then
    status="present"
    sha="$(sha256sum "$abs" | awk '{print $1}')"
  fi
  printf '{'
  printf '"path":'; json_string "$label"; printf ','
  printf '"source_path":'; json_string "$path"; printf ','
  printf '"status":'; json_string "$status"; printf ','
  printf '"sha256":'
  if [ -n "$sha" ]; then
    json_string "$sha"
  else
    printf 'null'
  fi
  printf '}'
}

vulnerability_evidence_json() {
  local log_path="$1"
  local log_abs="$repo_root/$log_path"
  local status="not_available"
  local called_count="null"
  local imported_count="null"
  local required_count="null"
  local ids=""
  local compact=""
  local review_date
  local disposition_issues
  local missing_count
  local expired_count

  if [ -f "$log_abs" ]; then
    status="available"
    compact="$(tr '\n' ' ' < "$log_abs")"
    called_count="$(printf '%s' "$compact" | sed -nE 's/.*Your code is affected by ([0-9]+) vulnerabilities.*/\1/p')"
    imported_count="$(govulncheck_imported_count_from_log "$log_path")"
    required_count="$(printf '%s' "$compact" | sed -nE 's/.*and ([0-9]+) vulnerabilities in modules you require.*/\1/p')"
    ids="$(vulnerability_ids_from_log "$log_path")"
  fi

  if [ -z "$called_count" ]; then
    called_count="null"
  fi
  if [ -z "$imported_count" ]; then
    imported_count="null"
  fi
  if [ -z "$required_count" ]; then
    required_count="null"
  fi
  review_date="$(release_review_date)"
  disposition_issues="$({
    vulnerability_disposition_issues "$ids" "$review_date"
    vulnerability_parser_issues "$log_path" "$ids"
  })"
  missing_count="$(count_issue_status "$disposition_issues" "missing")"
  expired_count="$(count_issue_status "$disposition_issues" "expired")"

  printf '{'
  printf '"source_log_path":'; json_string "$log_path"; printf ','
  printf '"status":'; json_string "$status"; printf ','
  printf '"review_date":'; json_string "$review_date"; printf ','
  printf '"called_vulnerability_count":%s,' "$called_count"
  printf '"imported_not_called_vulnerability_count":%s,' "$imported_count"
  printf '"required_not_called_module_vulnerability_count":%s,' "$required_count"
  printf '"imported_not_called_ids":'
  printf '%s\n' "$ids" | json_string_array_from_lines
  printf ','
  printf '"disposition_manifest_path":'; json_string "docs/vulnerability-dispositions.tsv"; printf ','
  printf '"missing_disposition_count":%s,' "$missing_count"
  printf '"expired_disposition_count":%s,' "$expired_count"
  printf '"disposition_issues":'
  disposition_issues_json "$disposition_issues"
  printf ','
  printf '"review_disposition":'; json_string "Imported-but-not-called findings do not fail release evidence by themselves. Review release-check-summary.json, the vuln log, docs/dependency-risk.md, and docs/vulnerability-dispositions.tsv before publishing."
  printf '}'
}

tool_json() {
  local name="$1"
  local binary="$2"
  local command_line="$3"
  local output exit_code status

  if ! command -v "$binary" >/dev/null 2>&1; then
    printf '{'
    printf '"name":'; json_string "$name"; printf ','
    printf '"command_line":'; json_string "$command_line"; printf ','
    printf '"status":"unavailable","exit_code":127,"version":""'
    printf '}'
    return
  fi

  output="$(cd "$repo_root" && bash -c "$command_line" 2>&1)"
  exit_code=$?
  output="$(printf '%s' "$output" | sed -n '1,8p')"
  if [ "$exit_code" -eq 0 ]; then
    status="available"
  else
    status="error"
  fi

  printf '{'
  printf '"name":'; json_string "$name"; printf ','
  printf '"command_line":'; json_string "$command_line"; printf ','
  printf '"status":'; json_string "$status"; printf ','
  printf '"exit_code":%s,' "$exit_code"
  printf '"version":'; json_string "$output"
  printf '}'
}

tool_versions_json() {
  printf '['
  tool_json "go" "go" "go version"
  printf ','
  tool_json "golangci-lint" "golangci-lint" "golangci-lint version"
  printf ','
  tool_json "govulncheck" "govulncheck" 'go version -m "$(command -v govulncheck)"'
  printf ','
  tool_json "gosec" "gosec" "gosec -version"
  printf ','
  tool_json "apidiff" "apidiff" 'go version -m "$(command -v apidiff)"'
  printf ','
  tool_json "syft" "syft" "syft version"
  printf ','
  tool_json "cosign" "cosign" "cosign version"
  printf ']'
}

asset_status_json() {
  local asset="$1"
  local status="missing"
  if [ -f "$repo_root/$asset" ]; then
    status="present"
  fi
  printf '{"path":'
  json_string "$asset"
  printf ',"status":'
  json_string "$status"
  printf '}'
}

artifact_tiers_json() {
  local release_assets=(
    "$release_logs_archive_path"
    "release-asset-manifest.tsv"
    "sbom-root.spdx.json"
    "sbom-contrib.spdx.json"
    "sbom-root.spdx.json.sig"
    "sbom-root.spdx.json.pem"
    "sbom-contrib.spdx.json.sig"
    "sbom-contrib.spdx.json.pem"
  )

  printf '{'
  printf '"local_release_evidence":{'
  printf '"description":"Local release evidence reruns release checks and records logs, commands, durations, exit codes, tool versions, git working-tree state, contrib drift summary, disposition manifest paths, and the explicit API baseline. It does not generate or sign SBOMs.",'
  printf '"produces_signed_sboms":false,'
  printf '"artifacts":'; json_string_array "release-check-summary.json" "$log_dir/*.log" "$release_logs_archive_path"
  printf '},'
  printf '"github_release_workflow":{'
  printf '"description":"The GitHub release workflow produces the local evidence summary plus SBOMs, Sigstore signatures, certificates, and provenance attestations.",'
  printf '"produces_signed_sboms":true,'
  printf '"artifacts":['
  local first=true
  for asset in "${release_assets[@]}"; do
    if [ "$first" = true ]; then
      first=false
    else
      printf ','
    fi
    asset_status_json "$asset"
  done
  printf ']}'
  printf '}'
}

publication_artifact_expectations_json() {
  printf '{'
  printf '"local_evidence_assets":'; json_string_array "release-check-summary.json" "$log_dir/*.log" "$release_logs_archive_path"; printf ','
  printf '"github_draft_release_assets":'; json_string_array \
    "release-check-summary.json" \
    "release-evidence-logs.tgz" \
    "release-asset-manifest.tsv" \
    "sbom-root.spdx.json" \
    "sbom-contrib.spdx.json" \
    "sbom-root.spdx.json.sig" \
    "sbom-root.spdx.json.pem" \
    "sbom-contrib.spdx.json.sig" \
    "sbom-contrib.spdx.json.pem"; printf ','
  printf '"github_attestation_subjects":'; json_string_array \
    "release-check-summary.json" \
    "sbom-root.spdx.json" \
    "sbom-contrib.spdx.json"; printf ','
  printf '"local_generates_signed_sboms":false'
  printf '}'
}

publication_artifact_checksums_json() {
  printf '{'
  printf '"algorithm":"sha256",'
  printf '"github_draft_release_manifest":"release-asset-manifest.tsv",'
  printf '"local_evidence_assets":['
  checksum_asset_json "release-evidence-logs.tgz" "$release_logs_archive_path"
  printf ']'
  printf '}'
}

if [ "${RELEASE_CHECK_SUMMARY_SOURCE_ONLY:-}" = "1" ]; then
  return 0 2>/dev/null || exit 0
fi

sbom_status="not_generated"
if [ -f "$repo_root/sbom-root.spdx.json" ] && [ -f "$repo_root/sbom-contrib.spdx.json" ] && \
  [ -f "$repo_root/sbom-root.spdx.json.sig" ] && [ -f "$repo_root/sbom-contrib.spdx.json.sig" ]; then
  sbom_status="generated_and_signed"
fi

check_jsons=()
contrib_drift_json_value="$(contrib_drift_json "not_run" "" "" "$log_dir/contrib-api-drift-report.log" "false")"
overall_status="passed"
overall_exit=0

if [ "$run_checks" = true ]; then
  mkdir -p "$repo_root/$log_dir"
  if [ "$git_dirty" = true ] && [ "$allow_dirty_release_evidence" != true ]; then
    overall_status="failed"
    overall_exit=1
    for i in "${!check_names[@]}"; do
      check_jsons+=("$(check_json "${check_names[$i]}" "${check_commands[$i]}" "skipped_by_provenance_policy" "" "" "" "false")")
    done
  else
    for i in "${!check_names[@]}"; do
      if [ "$overall_status" != "passed" ]; then
        check_jsons+=("$(check_json "${check_names[$i]}" "${check_commands[$i]}" "skipped_after_failure" "" "" "" "false")")
        continue
      fi
      json="$(run_check "${check_names[$i]}" "${check_commands[$i]}")"
      exit_code=$?
      check_jsons+=("$json")
      if [ "$exit_code" -ne 0 ]; then
        overall_status="failed"
        overall_exit="$exit_code"
      fi
    done
    if [ "$overall_status" = "passed" ]; then
      contrib_drift_json_value="$(run_contrib_drift_report)"
      contrib_exit=$?
      if [ "$contrib_exit" -ne 0 ]; then
        overall_status="failed"
        overall_exit="$contrib_exit"
      else
        vuln_ids="$(vulnerability_ids_from_log "$log_dir/vuln.log")"
        vuln_issues="$({
          vulnerability_disposition_issues "$vuln_ids" "$(release_review_date)"
          vulnerability_parser_issues "$log_dir/vuln.log" "$vuln_ids"
        })"
        contrib_packages="$(contrib_drift_packages_from_log "$log_dir/contrib-api-drift-report.log")"
        contrib_issues="$(contrib_disposition_issues "$contrib_packages" "$(release_review_date)")"
      fi
      if [ -n "${vuln_issues:-}" ] || [ -n "${contrib_issues:-}" ]; then
        overall_status="failed"
        overall_exit=1
        if [ -n "${contrib_issues:-}" ]; then
          contrib_drift_json_value="$(contrib_drift_json "failed_disposition_review" "1" "" "$log_dir/contrib-api-drift-report.log" "true")"
        fi
      fi
    else
      contrib_drift_json_value="$(contrib_drift_json "skipped_after_failure" "" "" "$log_dir/contrib-api-drift-report.log" "false")"
    fi
  fi
else
  if [ "$git_dirty" = true ] && [ "$allow_dirty_release_evidence" != true ]; then
    overall_status="failed"
    overall_exit=1
  fi
  for i in "${!check_names[@]}"; do
    check_jsons+=("$(check_json "${check_names[$i]}" "${check_commands[$i]}" "not_run" "" "" "" "false")")
  done
fi

archive_release_evidence_logs

publication_eligible=false
if [ "$run_checks" = true ] && [ "$overall_status" = "passed" ] && [ "$git_dirty" != true ] && [ "$allow_dirty_release_evidence" != true ]; then
  publication_eligible=true
fi

printf '{\n'
printf '  "schema": "github.com/aatuh/api-toolkit/release-check-summary/v2",\n'
printf '  "created_at": '; json_string "$created_at"; printf ',\n'
printf '  "commit": '; json_string "$commit"; printf ',\n'
printf '  "git_state": '; git_state_json; printf ',\n'
printf '  "provenance_policy": '; provenance_policy_json; printf ',\n'
printf '  "api_base_ref": '; json_string "$api_base_ref"; printf ',\n'
printf '  "quality_command": '; json_string "API_BASE_REF=$api_base_ref GOTOOLCHAIN=$gotoolchain make release-check"; printf ',\n'
if [ "$allow_dirty_release_evidence" = true ]; then
  printf '  "evidence_command": '; json_string "ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=$api_base_ref GOTOOLCHAIN=$gotoolchain make release-evidence"; printf ',\n'
else
  printf '  "evidence_command": '; json_string "API_BASE_REF=$api_base_ref GOTOOLCHAIN=$gotoolchain make release-evidence"; printf ',\n'
fi
printf '  "status": '; json_string "$overall_status"; printf ',\n'
printf '  "publication_eligible": %s,\n' "$publication_eligible"
printf '  "checks": [\n'
for i in "${!check_jsons[@]}"; do
  if [ "$i" -gt 0 ]; then
    printf ',\n'
  fi
  printf '    %s' "${check_jsons[$i]}"
done
printf '\n  ],\n'
printf '  "tool_versions": '; tool_versions_json; printf ',\n'
printf '  "vulnerability_evidence": '; vulnerability_evidence_json "$log_dir/vuln.log"; printf ',\n'
printf '  "contrib_drift": %s,\n' "$contrib_drift_json_value"
printf '  "artifact_tiers": '; artifact_tiers_json; printf ',\n'
printf '  "publication_artifact_expectations": '; publication_artifact_expectations_json; printf ',\n'
printf '  "publication_artifact_checksums": '; publication_artifact_checksums_json; printf ',\n'
printf '  "sbom_status": '; json_string "$sbom_status"; printf ',\n'
printf '  "sbom_assets": '; json_string_array \
  "sbom-root.spdx.json" \
  "sbom-contrib.spdx.json" \
  "sbom-root.spdx.json.sig" \
  "sbom-root.spdx.json.pem" \
  "sbom-contrib.spdx.json.sig" \
  "sbom-contrib.spdx.json.pem"; printf '\n'
printf '}\n'

exit "$overall_exit"
