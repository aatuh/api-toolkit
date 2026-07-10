#!/usr/bin/env python3
"""Write a Go-module license report from an SPDX SBOM."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


HEADER = ("module", "version", "license_expression", "status", "source_purls")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="write a Go module license report from an SPDX JSON SBOM"
    )
    parser.add_argument("--scope", choices=("root", "contrib"), required=True)
    parser.add_argument("--module-dir", type=Path, required=True)
    parser.add_argument("--sbom", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args()


def decode_json_stream(payload: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    values: list[dict[str, Any]] = []
    offset = 0
    while offset < len(payload):
        while offset < len(payload) and payload[offset].isspace():
            offset += 1
        if offset == len(payload):
            break
        value, offset = decoder.raw_decode(payload, offset)
        if not isinstance(value, dict):
            raise ValueError("go list emitted a non-object module record")
        values.append(value)
    return values


def module_purls(module_dir: Path) -> list[str]:
    env = os.environ.copy()
    env["GOWORK"] = "off"
    result = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=module_dir,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError("go list -m -json all failed")

    purls = set()
    for module in decode_json_stream(result.stdout):
        if module.get("Main"):
            continue
        path = module.get("Path")
        version = module.get("Version")
        if not isinstance(path, str) or not isinstance(version, str) or not path or not version:
            raise ValueError("go list emitted a dependency without a path and version")
        purls.add(f"pkg:golang/{path}@{version}")
    return sorted(purls)


def package_license_index(sbom: dict[str, Any]) -> dict[str, set[str]]:
    packages = sbom.get("packages")
    if not isinstance(packages, list):
        raise ValueError("SPDX SBOM packages must be an array")

    licenses: dict[str, set[str]] = {}
    for package in packages:
        if not isinstance(package, dict):
            continue
        expression = package.get("licenseConcluded", "NOASSERTION")
        if not isinstance(expression, str) or not expression:
            expression = "NOASSERTION"
        refs = package.get("externalRefs", [])
        if not isinstance(refs, list):
            continue
        for ref in refs:
            if not isinstance(ref, dict):
                continue
            if ref.get("referenceCategory") != "PACKAGE-MANAGER" or ref.get("referenceType") != "purl":
                continue
            purl = ref.get("referenceLocator")
            if isinstance(purl, str) and purl.startswith("pkg:golang/"):
                licenses.setdefault(purl, set()).add(expression)
    return licenses


def report_rows(module_dir: Path, sbom_path: Path) -> list[tuple[str, str, str, str, str]]:
    with sbom_path.open("r", encoding="utf-8") as handle:
        sbom = json.load(handle)
    if not isinstance(sbom, dict):
        raise ValueError("SPDX SBOM root must be an object")

    package_licenses = package_license_index(sbom)
    rows = []
    for purl in module_purls(module_dir):
        module_and_version = purl.removeprefix("pkg:golang/")
        module, version = module_and_version.rsplit("@", 1)
        expressions = sorted(package_licenses.get(purl, set()))
        if not expressions:
            rows.append((module, version, "NOASSERTION", "missing_from_sbom", purl))
            continue
        expression = " OR ".join(expressions)
        status = "detected"
        if any("NOASSERTION" in value or "LicenseRef-" in value for value in expressions):
            status = "needs_review"
        rows.append((module, version, expression, status, purl))
    return rows


def write_rows(output: Path, rows: list[tuple[str, str, str, str, str]]) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        newline="",
        dir=output.parent,
        prefix=f".{output.name}.",
        delete=False,
    ) as handle:
        writer = csv.writer(handle, delimiter="\t", lineterminator="\n")
        writer.writerow(HEADER)
        writer.writerows(rows)
        temporary = Path(handle.name)
    os.replace(temporary, output)


def main() -> int:
    args = parse_args()
    module_dir = args.module_dir.resolve()
    if not module_dir.is_dir():
        raise ValueError("--module-dir must name an existing directory")
    if not args.sbom.is_file():
        raise ValueError("--sbom must name an existing SPDX JSON file")

    rows = report_rows(module_dir, args.sbom)
    write_rows(args.output, rows)
    needs_review = sum(row[3] != "detected" for row in rows)
    print(
        f"dependency license report: scope={args.scope} modules={len(rows)} "
        f"needs_review={needs_review} output={args.output}"
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, RuntimeError, ValueError, json.JSONDecodeError) as error:
        print(f"dependency license report failed: {error}", file=sys.stderr)
        raise SystemExit(1)
