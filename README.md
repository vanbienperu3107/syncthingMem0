# syncthingMem0

Planned Syncthing/Mem0 fork for proxy-friendly continuous file synchronization.

## Project Goal

This project follows the architecture described in the internal
"Architecture Summary" for a Syncthing-based fork that can:

- sync in real time between a VPS hub and multiple personal devices
- work through proxy-restricted networks that only allow HTTPS on port 443
- replace certificate-based pairing with bearer-token authentication
- reduce scan and transfer cost for large folders

The target design is a hub-based synchronization model over HTTPS/WebSocket,
not the default Syncthing transport and relay model.

## Planned Architecture

Compared with upstream Syncthing, the planned fork introduces five main changes:

1. WSS transport over port 443 instead of the default TCP/TLS plus relay ports
2. Bearer-token authentication with JWT instead of TLS client certificates
3. Incremental scanning with dirty paths and digest cache instead of periodic
   full-folder scans
4. rsync-style delta transfer instead of fixed-block transfer
5. Last-writer-wins with ancestor tracking instead of conflict copy files

At a high level, the system is intended to look like this:

- a VPS hub accepts HTTPS/WebSocket connections on port 443
- each client device authenticates with a bearer token
- synchronization, reconciliation, and metadata storage are centralized at the hub

## Repository Status

This repository currently contains bootstrap automation and deployment
scaffolding for that planned fork:

- CI workflow for Go repositories
- release workflow for version tags
- optional deployment workflow for relay and discovery style services
- supporting docs for CI/CD and VPS deployment

The actual Go source code for the planned fork has not been imported into this
repository yet.

That means:

- CI currently passes in bootstrap mode when no Go module is present
- release automation is ready, but it will only build once a root `go.mod`
  and `cmd/syncthing` are added
- optional server deploy is ready, but it will only run once matching server
  source exists in the repository

## Workflows

- CI: `.github/workflows/ci.yml`
- Release: `.github/workflows/release.yml`
- Optional server deploy: `.github/workflows/deploy-optional-servers.yml`

## Docs In This Repo

- CI/CD guide: `docs/ci-cd.md`
- Optional server deploy guide: `docs/deploy-servers.md`

## Release Example

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Notes

The current README reflects the intended architecture from the project
documentation, while also staying accurate about the present repository state:
the implementation plan is defined, but the fork source itself is not yet here.
