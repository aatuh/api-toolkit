#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
script="$repo_root/scripts/dead_code_todo_check.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$tmp_dir/docs" "$tmp_dir/pkg/stable" "$tmp_dir/pkg/experimental"

cat >"$tmp_dir/docs/package-classification.tsv" <<'TSV'
import_path	api_status	test_status	notes
example.com/root/pkg/stable	stable	direct-tests	stable fixture package
example.com/root/pkg/experimental	experimental	direct-tests	experimental fixture package
TSV

cat >"$tmp_dir/docs/todo-exceptions.tsv" <<'TSV'
path	marker	issue	classification	reason
TSV

cat >"$tmp_dir/pkg/stable/stable.go" <<'GO'
package stable

func Stable() {
	// TODO: missing issue
}
GO

cat >"$tmp_dir/pkg/experimental/experimental.go" <<'GO'
package experimental

func Experimental() {
	// TODO: experimental packages are outside the stable TODO gate
}
GO

if TODO_GATE_REPO_ROOT="$tmp_dir" \
  TODO_GATE_CLASSIFICATION="$tmp_dir/docs/package-classification.tsv" \
  TODO_GATE_EXCEPTIONS="$tmp_dir/docs/todo-exceptions.tsv" \
  TODO_GATE_ROOT_MODULE="example.com/root" \
  "$script" >"$tmp_dir/missing-issue.out" 2>&1; then
  echo "expected missing-issue TODO fixture to fail" >&2
  exit 1
fi
if ! grep -q "missing linked issue" "$tmp_dir/missing-issue.out"; then
  cat "$tmp_dir/missing-issue.out" >&2
  echo "missing-issue fixture did not explain the linked-issue requirement" >&2
  exit 1
fi

cat >"$tmp_dir/pkg/stable/stable.go" <<'GO'
package stable

func Stable() {
	// TODO(#123): linked but not classified
}
GO

if TODO_GATE_REPO_ROOT="$tmp_dir" \
  TODO_GATE_CLASSIFICATION="$tmp_dir/docs/package-classification.tsv" \
  TODO_GATE_EXCEPTIONS="$tmp_dir/docs/todo-exceptions.tsv" \
  TODO_GATE_ROOT_MODULE="example.com/root" \
  "$script" >"$tmp_dir/missing-exception.out" 2>&1; then
  echo "expected unclassified TODO fixture to fail" >&2
  exit 1
fi
if ! grep -q "missing non-blocking exception row" "$tmp_dir/missing-exception.out"; then
  cat "$tmp_dir/missing-exception.out" >&2
  echo "unclassified fixture did not explain the exception requirement" >&2
  exit 1
fi

cat >"$tmp_dir/docs/todo-exceptions.tsv" <<'TSV'
path	marker	issue	classification	reason
pkg/stable/stable.go	TODO	#123	non-blocking	fixture proves classified stable TODOs may pass
TSV

TODO_GATE_REPO_ROOT="$tmp_dir" \
  TODO_GATE_CLASSIFICATION="$tmp_dir/docs/package-classification.tsv" \
  TODO_GATE_EXCEPTIONS="$tmp_dir/docs/todo-exceptions.tsv" \
  TODO_GATE_ROOT_MODULE="example.com/root" \
  "$script" >"$tmp_dir/pass.out" 2>&1

if ! grep -q "stable package TODO/FIXME gate passed" "$tmp_dir/pass.out"; then
  cat "$tmp_dir/pass.out" >&2
  echo "passing fixture did not report success" >&2
  exit 1
fi

echo "dead code TODO gate contract tests passed"
