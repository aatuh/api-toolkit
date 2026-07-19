#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/api_additions_check.sh"

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

init_repo() {
  local dir="$1"
  mkdir -p "$dir/docs"
  git -C "$dir" init -q
  git -C "$dir" config user.email "api-additions-contract@example.invalid"
  git -C "$dir" config user.name "api additions contract"
  printf '# API inventory\n' >"$dir/docs/api-inventory.md"
}

run_script_in_dir() {
  local dir="$1"
  shift
  (
    cd "$dir"
    env "$@" "$script"
  )
}

make_fake_go() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  cat >"$bin_dir/go" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$GO_CALLS"
FAKE
  chmod +x "$bin_dir/go"
}

fake_bin="$tmp/bin"
make_fake_go "$fake_bin"

major_dir="$tmp/major-transition"
init_repo "$major_dir"
printf 'module example.com/toolkit/v3\n\ngo 1.25.0\n' >"$major_dir/go.mod"
git -C "$major_dir" add docs/api-inventory.md go.mod
git -C "$major_dir" commit -qm base
git -C "$major_dir" tag v3-base
printf 'module example.com/toolkit/v4\n\ngo 1.25.0\n' >"$major_dir/go.mod"
git -C "$major_dir" add go.mod
git -C "$major_dir" commit -qm major-transition

major_calls="$tmp/major.calls"
: >"$major_calls"
major_output="$(run_script_in_dir "$major_dir" PATH="$fake_bin:$PATH" GO_CALLS="$major_calls" API_ADDITIONS_BASE_REF=v3-base)"
case "$major_output" in
  *"API module path changed from example.com/toolkit/v3 to example.com/toolkit/v4; skipping API additions check"*) ;;
  *) printf 'major transition did not explain the API additions skip:\n%s\n' "$major_output" >&2; exit 1 ;;
esac
if [ -s "$major_calls" ]; then
  printf 'major transition unexpectedly ran the API additions tool\n' >&2
  exit 1
fi

same_major_dir="$tmp/same-major"
init_repo "$same_major_dir"
printf 'module example.com/toolkit/v4\n\ngo 1.25.0\n' >"$same_major_dir/go.mod"
git -C "$same_major_dir" add docs/api-inventory.md go.mod
git -C "$same_major_dir" commit -qm base
git -C "$same_major_dir" tag v4-base
touch "$same_major_dir/README.md"
git -C "$same_major_dir" add README.md
git -C "$same_major_dir" commit -qm current

same_major_calls="$tmp/same-major.calls"
: >"$same_major_calls"
run_script_in_dir "$same_major_dir" PATH="$fake_bin:$PATH" GO_CALLS="$same_major_calls" API_ADDITIONS_BASE_REF=v4-base >/dev/null
if [ ! -s "$same_major_calls" ]; then
  printf 'same-major comparison did not run the API additions tool\n' >&2
  exit 1
fi
