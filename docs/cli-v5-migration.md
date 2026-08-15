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
