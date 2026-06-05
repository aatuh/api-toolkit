#!/usr/bin/env bash
set -euo pipefail

go_cmd="${GO:-go}"
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
out_dir="${OUTPUT_DIR:-.ci-result}/dependencies"
api_base_ref="${API_BASE_REF:-}"

mkdir -p "$out_dir"

require_counts() {
  local mod_file="$1"
  awk '
    /^require \(/ { inblock = 1; next }
    inblock && /^\)/ { inblock = 0; next }
    inblock && NF >= 2 {
      if ($0 ~ /\/\/ indirect/) indirect++
      else direct++
      next
    }
    /^require[ \t]+/ {
      if ($0 ~ /\/\/ indirect/) indirect++
      else direct++
    }
    END { printf "%d\t%d\n", direct, indirect }
  ' "$mod_file"
}

module_list() {
  local dir="$1"
  (cd "$dir" && GOWORK=off "$go_cmd" list -m all) | tail -n +2 | sort
}

write_module_report() {
  local name="$1"
  local dir="$2"
  local modules="$out_dir/$name.modules"
  local counts direct_count indirect_count build_list_count

  module_list "$dir" >"$modules"
  counts="$(require_counts "$dir/go.mod")"
  direct_count="${counts%%$'\t'*}"
  indirect_count="${counts##*$'\t'}"
  build_list_count="$(wc -l <"$modules" | tr -d ' ')"

  printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$direct_count" "$indirect_count" "$build_list_count" "$modules"
}

write_minimal_core_report() {
  local packages="$out_dir/minimal-core-packages.txt"
  local summary="$out_dir/minimal-core-summary.tsv"

  (
    cd "$repo_root"
    GOWORK=off "$go_cmd" list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' \
      ./httpx ./binding ./middleware/maxbody ./middleware/timeout
  ) | sed '/^$/d' | sort -u >"$packages"

  local total toolkit third_party
  total="$(wc -l <"$packages" | tr -d ' ')"
  toolkit="$(grep -c '^github.com/aatuh/api-toolkit/v3' "$packages" || true)"
  third_party=$((total - toolkit))

  {
    printf 'profile\tpackage_count\ttoolkit_package_count\tthird_party_package_count\tpackage_list\n'
    printf 'minimal-core\t%s\t%s\t%s\t%s\n' "$total" "$toolkit" "$third_party" "$packages"
  } >"$summary"
}

write_diff_reports() {
  local base_ref="$1"
  local tmp_dir base_dir
  tmp_dir="$(mktemp -d)"
  base_dir="$tmp_dir/base"

  git -C "$repo_root" worktree add --detach --quiet "$base_dir" "$base_ref"
  trap 'git -C "$repo_root" worktree remove --force "$base_dir" >/dev/null 2>&1 || true; rm -rf "$tmp_dir"' RETURN

  for name in root contrib; do
    local current="$out_dir/$name.modules"
    local base_modules="$out_dir/$name.base.modules"
    local dir="."
    if [ "$name" = "contrib" ]; then
      dir="contrib"
    fi
    if [ ! -d "$base_dir/$dir" ]; then
      : >"$base_modules"
    else
      module_list "$base_dir/$dir" >"$base_modules"
    fi
    comm -13 "$base_modules" "$current" >"$out_dir/$name.added.modules"
    comm -23 "$base_modules" "$current" >"$out_dir/$name.removed.modules"
  done
}

summary_tsv="$out_dir/summary.tsv"
{
  printf 'module\tdirect_require_count\tindirect_require_count\tbuild_list_dependency_count\tmodule_list\n'
  write_module_report root "$repo_root"
  write_module_report contrib "$repo_root/contrib"
} >"$summary_tsv"

write_minimal_core_report

if [ -n "$api_base_ref" ]; then
  write_diff_reports "$api_base_ref"
fi

root_added="$out_dir/root.added.modules"
root_removed="$out_dir/root.removed.modules"
contrib_added="$out_dir/contrib.added.modules"
contrib_removed="$out_dir/contrib.removed.modules"

root_added_count=0
root_removed_count=0
contrib_added_count=0
contrib_removed_count=0
if [ -s "$root_added" ]; then root_added_count="$(wc -l <"$root_added" | tr -d ' ')"; fi
if [ -s "$root_removed" ]; then root_removed_count="$(wc -l <"$root_removed" | tr -d ' ')"; fi
if [ -s "$contrib_added" ]; then contrib_added_count="$(wc -l <"$contrib_added" | tr -d ' ')"; fi
if [ -s "$contrib_removed" ]; then contrib_removed_count="$(wc -l <"$contrib_removed" | tr -d ' ')"; fi

cat >"$out_dir/summary.md" <<EOF_SUMMARY
## Dependency footprint summary

This report is a dependency review signal. Vulnerability status remains owned by
\`make vuln\` and \`release-check-summary.json\` \`vulnerability_evidence\`.

| Module | Direct requires | Indirect requires | Build-list dependencies |
| --- | ---: | ---: | ---: |
$(awk 'NR > 1 { printf "| `%s` | %s | %s | %s |\n", $1, $2, $3, $4 }' "$summary_tsv")

Minimal core package footprint:

$(awk 'NR > 1 { printf "- profile `%s`: %s non-stdlib packages, %s toolkit packages, %s third-party packages\n", $1, $2, $3, $4 }' "$out_dir/minimal-core-summary.tsv")

Base ref: \`${api_base_ref:-not set}\`

| Module | Added modules | Removed modules |
| --- | ---: | ---: |
| root | ${root_added_count} | ${root_removed_count} |
| contrib | ${contrib_added_count} | ${contrib_removed_count} |

Artifacts:

- \`${summary_tsv}\`
- \`${out_dir}/root.modules\`
- \`${out_dir}/contrib.modules\`
- \`${out_dir}/minimal-core-summary.tsv\`
- \`${out_dir}/minimal-core-packages.txt\`
EOF_SUMMARY

cat "$out_dir/summary.md"
