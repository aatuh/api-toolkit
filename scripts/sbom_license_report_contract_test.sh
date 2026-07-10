#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/sbom_license_report.py"
tmp="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

module_dir="$tmp/module"
bin_dir="$tmp/bin"
mkdir -p "$module_dir" "$bin_dir"

cat >"$bin_dir/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -euo pipefail

if [ "$*" != "list -m -json all" ]; then
  exit 2
fi

printf '%s\n' \
  '{"Path":"example.com/project","Main":true}' \
  '{"Path":"example.com/bar","Version":"v2.0.0"}' \
  '{"Path":"example.com/foo","Version":"v1.0.0"}' \
  '{"Path":"example.com/missing","Version":"v3.0.0"}'
FAKE_GO
chmod +x "$bin_dir/go"

cat >"$tmp/sbom.json" <<'JSON'
{
  "packages": [
    {
      "licenseConcluded": "MIT",
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:golang/example.com/foo@v1.0.0"
        }
      ]
    },
    {
      "licenseConcluded": "BSD-3-Clause",
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:golang/example.com/foo@v1.0.0"
        }
      ]
    },
    {
      "licenseConcluded": "NOASSERTION",
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:golang/example.com/bar@v2.0.0"
        }
      ]
    },
    {
      "licenseConcluded": "Apache-2.0",
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:npm/unrelated@1.0.0"
        }
      ]
    }
  ]
}
JSON

PATH="$bin_dir:$PATH" python3 "$script" \
  --scope root \
  --module-dir "$module_dir" \
  --sbom "$tmp/sbom.json" \
  --output "$tmp/report.tsv" >/dev/null

cat >"$tmp/expected.tsv" <<'TSV'
module	version	license_expression	status	source_purls
example.com/bar	v2.0.0	NOASSERTION	needs_review	pkg:golang/example.com/bar@v2.0.0
example.com/foo	v1.0.0	BSD-3-Clause OR MIT	detected	pkg:golang/example.com/foo@v1.0.0
example.com/missing	v3.0.0	NOASSERTION	missing_from_sbom	pkg:golang/example.com/missing@v3.0.0
TSV

diff -u "$tmp/expected.tsv" "$tmp/report.tsv"

if PATH="$bin_dir:$PATH" python3 "$script" \
  --scope root \
  --module-dir "$module_dir" \
  --sbom "$tmp/missing.json" \
  --output "$tmp/absent.tsv" >/dev/null 2>&1; then
  echo "expected missing SBOM input to fail" >&2
  exit 1
fi
test ! -e "$tmp/absent.tsv"

echo "SBOM license report contract tests passed"
