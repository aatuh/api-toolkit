# CLI v5 migration

Audience: existing generator users moving from the v4 contrib command to the
independently versioned CLI module.

## Install and invoke

Install a released CLI version explicitly:

```sh
go install github.com/aatuh/api-toolkit/cmd/api-toolkit/v5@<released-version>
api-toolkit version --json
```

The v4 command remains available only as a compatibility path:

```sh
go run github.com/aatuh/api-toolkit/contrib/v4/cmd/api-toolkit@v4.0.1 new service ...
```

For new projects, use the installed `api-toolkit` v5 command. Generated
Makefiles expose `API_TOOLKIT` so CI can pin an installed binary or a verified
release wrapper without changing generated source.

## Generated-project identity

Every generated project includes `.api-toolkit-generator.json`. It records the
CLI module and version, template schema, and the explicit core/contrib module
versions selected during generation. This file is generator-owned metadata:
preserve it during upgrades and review it when accepting regenerated output.

## Filesystem safety and CI modes

`api-toolkit new service` treats the directory from which it is run as its
approved output root. Relative `--dir` values must remain below that root; any
`..` component is rejected. An absolute destination is rejected unless
`--allow-absolute-dir` is supplied, and is still rejected when it is outside
the approved root. This keeps a copied CI command from unexpectedly creating a
project elsewhere on the machine.

The command does not replace an existing non-empty directory by default. Use
one explicit noninteractive mode when automation needs a different policy:

```sh
# Fail if even an empty destination already exists.
api-toolkit new service --module example.com/catalog --dir catalog --fail-if-exists

# Verify an existing generated project exactly matches the current inputs.
# This makes no change and exits non-zero when it is missing, unsafe, or stale.
api-toolkit new service --module example.com/catalog --dir catalog --check

# Replace only a recognized generator project. This refuses unknown files.
api-toolkit new service --module example.com/catalog --dir catalog --overwrite-generated
```

`--overwrite-generated` requires valid `.api-toolkit-generator.json` metadata,
rejects paths not present in the newly generated project, and replaces known
generated files only after the full replacement has been rendered in a private
staging directory. It is deliberately not a general-purpose `--force` or full
overwrite mode: preserve app-owned files outside the generated project, and
review changes to known generated files before choosing this flag.

Before either checking or replacing a project, the CLI rejects symlinks,
hard-linked regular files, named pipes, devices, sockets, and group- or
world-writable project paths. New files are created with restrictive
permissions. Initial generation publishes the completed staging directory; a
replacement keeps a same-filesystem backup and restores it if publishing the
new directory fails. These checks are local-filesystem safeguards, not a
substitute for running the CLI from a trusted account and workspace.

## Offline dependency manifest

CLI releases embed templates and default to the reviewed v4.0.1 manifest.
Generation does not fetch templates or dependencies: generated metadata records
the embedded source and network policy, `go.mod` pins versions, and default
output includes reviewed direct-module checksums in `go.sum`.

`make cli-offline-check` proves generation works with `GOPROXY=off` and
`GOSUMDB=off`. Update this manifest only in a reviewed CLI change: select
immutable published versions, refresh their checksums, update the embedded
manifest and assertions together, then run the offline check. `--allow-network`
is explicit metadata for a future workflow; this generator makes no network
calls.

## Generated-project contract and upgrades

Every new project also includes `.api-toolkit-project.json`. It records the
CLI/template identity, selected profile and module versions, providers, and the
complete generator-owned file inventory. `user_owned_files` is intentionally
empty for a fresh project: application code added later is not inferred as
generator-owned.

Use `api-toolkit project inspect --dir <project>` to read the contract and
`api-toolkit project check --dir <project>` to verify its generated inventory.
`project diff` and `project upgrade --check` report the supported policy only:
in-place upgrades never overwrite application-owned code. Generate a fresh
project, compare it with the existing application, migrate app-owned changes
deliberately, and retain the prior directory as rollback until validation
succeeds. Unsupported manifest schemas fail with this same actionable manual
migration guidance.

Until v5 core and adapter modules are published, v5 CLI templates deliberately
generate explicit v4 core and contrib requirements. The CLI module itself does
not import contrib provider libraries. Do not claim a v5 generated-project
dependency until a verified corresponding v5 release exists.

## Compatibility window

The v4 invocation is documented for existing users but receives no new
generator behavior. Update CI, local scripts, and developer setup to install a
pinned v5 CLI release before removing the v4 command from your environment.
The tag publication and release-artifact evidence are external responsibilities
tracked by EXT-005; they are not established by this local migration guide.
