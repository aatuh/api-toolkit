#!/usr/bin/env bash
# Report-only public API drift check for selected contrib packages.
set -euo pipefail

base_ref="${CONTRIB_API_BASE_REF:-${API_BASE_REF:-}}"
if [ -z "$base_ref" ]; then
  echo "Set CONTRIB_API_BASE_REF or API_BASE_REF to generate a contrib API drift report." >&2
  exit 2
fi

if ! git rev-parse --verify "$base_ref" >/dev/null 2>&1; then
  echo "Base ref $base_ref not found; skipping report-only contrib API drift check."
  exit 0
fi

if ! command -v apidiff >/dev/null 2>&1; then
  echo "apidiff not found. Install with: make tools" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
manifest="${CONTRIB_API_DRIFT_MANIFEST:-docs/contrib-api-drift-packages.txt}"
manifest_path="$repo_root/$manifest"
if [ ! -f "$manifest_path" ]; then
  echo "Contrib API drift manifest $manifest is missing." >&2
  exit 2
fi

packages=()
while IFS= read -r raw_line || [ -n "$raw_line" ]; do
  pkg="$(printf '%s' "$raw_line" | sed 's/[[:space:]]*#.*$//' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [ -z "$pkg" ]; then
    continue
  fi
  case "$pkg" in
    github.com/aatuh/api-toolkit/contrib/v2/*) ;;
    *)
      echo "Invalid contrib API drift package in $manifest: $pkg" >&2
      exit 2
      ;;
  esac
  packages+=("$pkg")
done < "$manifest_path"

if [ "${#packages[@]}" -eq 0 ]; then
  echo "Contrib API drift manifest $manifest has no packages." >&2
  exit 2
fi

worktree="$(mktemp -d)"
tmpdir="$(mktemp -d)"
cleanup() {
  git worktree remove --force "$worktree" >/dev/null 2>&1 || true
  rm -rf "$worktree" "$tmpdir"
}
trap cleanup EXIT

git worktree add "$worktree" "$base_ref" --quiet
if [ -f "$worktree/contrib/go.mod" ]; then
  (cd "$worktree/contrib" && go mod tidy)
fi

echo "Contrib API drift report (report-only)"
echo "Baseline: $base_ref"
echo "Package manifest: $manifest"
echo "Policy: contrib remains outside the stable v2 API promise; review drift for release notes and migration guidance."

drift_count=0
skip_count=0
compatible_drift_count=0
incompatible_drift_count=0
for pkg in "${packages[@]}"; do
  rel="${pkg#github.com/aatuh/api-toolkit/contrib/v2}"
  rel="${rel#/}"
  old_path="$worktree/contrib/$rel"
  new_path="$repo_root/contrib/$rel"
  export_name="$(printf '%s' "$pkg" | tr '/:' '__')"
  old_export="$tmpdir/$export_name.old"
  new_export="$tmpdir/$export_name.new"

  if [ ! -d "$old_path" ] || [ ! -d "$new_path" ]; then
    echo "SKIP $pkg (missing in baseline or working tree)"
    skip_count=$((skip_count + 1))
    continue
  fi
  if ! (cd "$worktree/contrib" && apidiff -w "$old_export" "$pkg") >/dev/null 2>&1; then
    echo "SKIP $pkg (could not export baseline package)"
    skip_count=$((skip_count + 1))
    continue
  fi
  if ! (cd "$repo_root/contrib" && apidiff -w "$new_export" "$pkg") >/dev/null 2>&1; then
    echo "SKIP $pkg (could not export working tree package)"
    skip_count=$((skip_count + 1))
    continue
  fi

  diff_output="$(apidiff "$old_export" "$new_export" 2>&1 || true)"
  if [ -z "$diff_output" ]; then
    echo "OK   $pkg"
    continue
  fi
  drift_count=$((drift_count + 1))
  if printf '%s\n' "$diff_output" | grep -qi '^Incompatible changes:'; then
    incompatible_drift_count=$((incompatible_drift_count + 1))
  else
    compatible_drift_count=$((compatible_drift_count + 1))
  fi
  echo "DRIFT $pkg"
  printf '%s\n' "$diff_output"
done

echo "Report complete: drift_packages=$drift_count skipped_packages=$skip_count compatible_drift_packages=$compatible_drift_count incompatible_drift_packages=$incompatible_drift_count"
exit 0
