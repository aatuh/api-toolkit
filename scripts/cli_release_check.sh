#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${CLI_RELEASE_OUT_DIR:-$root_dir/.ci-result/cli-release}"
version="${CLI_RELEASE_VERSION:-dev}"
commit="${CLI_RELEASE_COMMIT:-$(git -C "$root_dir" rev-parse HEAD)}"
date="${CLI_RELEASE_DATE:-$(git -C "$root_dir" show -s --format=%cI HEAD)}"

rm -rf "$out_dir"
mkdir -p "$out_dir"
targets=(linux/amd64 linux/arm64 darwin/arm64 windows/amd64)
for target in "${targets[@]}"; do
  goos="${target%/*}"; goarch="${target#*/}"
  suffix=""
  [ "$goos" = windows ] && suffix=.exe
  name="api-toolkit_${version}_${goos}_${goarch}${suffix}"
  (cd "$root_dir/cmd/api-toolkit" && GOWORK=off GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-X main.buildCommit=$commit -X main.buildDate=$date" -o "$out_dir/$name" .)
done
(cd "$out_dir" && sha256sum api-toolkit_* > checksums.txt)
grep -E '/home/|/Users/|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|AKIA[0-9A-Z]{16}' "$out_dir"/api-toolkit_* && { echo "unsafe local path or credential-shaped content in CLI artifact" >&2; exit 1; } || true
linux_bin="$(find "$out_dir" -name '*_linux_amd64' -type f -print -quit)"
version_json="$($linux_bin version --json)"
printf '%s\n' "$version_json" | grep -q '"build_commit":"' || { echo "version json lacks build commit" >&2; exit 1; }
printf '%s\n' "$version_json" | grep -q '"build_date":"' || { echo "version json lacks build date" >&2; exit 1; }
printf 'CLI release matrix verified: %s\n' "$out_dir"
