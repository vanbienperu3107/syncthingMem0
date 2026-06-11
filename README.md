# syncthingMem0

Bootstrap repository for Syncthing/Mem0 automation.

## Current State

This repository currently contains project automation and deployment scaffolding:

- CI workflow for Go repositories
- Release workflow for version tags
- Optional server deployment workflow for `strelaysrv` and `stdiscosrv`
- Supporting documentation for CI/CD and VPS deployment

At the moment, the repository does not yet contain the actual Go source code
for `cmd/syncthing`, `cmd/strelaysrv`, or `cmd/stdiscosrv`.

That means:

- CI passes in bootstrap mode when no Go module is present
- release workflow is ready, but will only build once a root `go.mod` and
  `cmd/syncthing` are added
- optional server deploy is ready, but will only run when server source is
  added to the repository

## Workflows

- CI: `.github/workflows/ci.yml`
- Release: `.github/workflows/release.yml`
- Optional server deploy: `.github/workflows/deploy-optional-servers.yml`

## Docs

- CI/CD guide: `docs/ci-cd.md`
- Optional server deploy guide: `docs/deploy-servers.md`

## Release Example

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Notes

If this repository is later populated with Syncthing-based source code, the
existing workflows are already structured to:

- test detected Go modules automatically
- build release artifacts for Linux, macOS, and Windows
- build and deploy optional relay/discovery server images to a VPS
