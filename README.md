# syncthingMem0

Planned Syncthing/Mem0 fork for proxy-friendly continuous file synchronization.

## Overview

This project is intended to evolve into a Syncthing-based fork that syncs data
in real time between a VPS hub and multiple personal devices, while still
working in networks that only allow HTTPS traffic on port 443.

The target design is different from upstream Syncthing:

- transport moves to HTTPS/WebSocket on port 443
- authentication moves to bearer tokens instead of TLS client certificates
- scanning becomes incremental instead of periodic full-folder rescans
- transfer moves toward rsync-style delta exchange
- conflict handling moves toward last-writer-wins with ancestor tracking

## Architecture Summary

At a high level, the planned system looks like this:

```text
                +--------------------------------------+
                | VPS Hub                              |
                | HTTPS/WSS :443                       |
                | JWT auth                             |
                | Sync engine                          |
                | Reconciler                           |
                | Incremental scanner                  |
                | rsync delta engine                   |
                | Metadata store                       |
                +-----------------+--------------------+
                                  |
               +------------------+------------------+
               |                  |                  |
             WSS                WSS                WSS
               |                  |                  |
         +-----------+      +-----------+      +-----------+
         | Client A  |      | Client B  |      | Client C  |
         +-----------+      +-----------+      +-----------+
```

The intended traffic model is simple:

1. A client authenticates to the VPS hub with a bearer token.
2. The client connects over WSS on port 443.
3. The hub coordinates metadata, reconciliation, and file transfer.
4. Changes are scanned incrementally and transmitted as compact deltas when possible.

## Planned Differences From Upstream

| Area | Upstream Syncthing | Planned fork |
|---|---|---|
| Transport | TCP/TLS plus relay ports | WSS over HTTPS on port 443 |
| Authentication | TLS client certificates | JWT bearer tokens |
| Discovery/relay model | Native discovery and relay services | Hub-oriented connection model |
| Scanning | Periodic full scan plus fsnotify | Incremental scan with dirty paths and cache |
| Transfer | Fixed-size block exchange | rsync-style delta transfer |
| Conflict handling | Conflict copy files | Last-writer-wins plus ancestor tracking |

## Core Components

### Hub

The hub is planned as the central coordination point. It accepts HTTPS and WSS
connections, verifies bearer tokens, keeps synchronization state, and drives
reconciliation and transfer decisions.

### Client

Each client connects to the hub using a device token. Clients are expected to
watch local file changes, maintain local state, and exchange data with the hub
over WebSocket transport.

### Authentication Layer

The planned authentication model replaces certificate-based identity with JWT
tokens signed by a shared server secret. This is meant to simplify device
registration and make the system friendlier to proxy-heavy environments.

### Incremental Scanner

Instead of walking the full folder tree on every scan interval, the planned
scanner keeps baseline state, tracks dirty paths, and reuses cached file digests
when metadata has not changed.

### Delta Transfer Engine

The planned transfer model is inspired by rsync: signatures, rolling hashes,
strong verification, and small patch payloads when only parts of a file change.

### Reconciler

Conflict resolution is planned around last-writer-wins semantics with ancestor
tracking so the system can distinguish one-sided change, two-sided change, and
delete-versus-modify cases more accurately.

## Planned Implementation Order

1. WSS transport
2. Bearer-token authentication
3. Incremental scanner
4. rsync delta transfer
5. Last-writer-wins plus ancestor tracking
6. Integration testing and release hardening

## Repository Status

This repository currently contains bootstrap automation and deployment
scaffolding for the planned fork, but not the fork source code itself.

What is already here:

- CI workflow for Go repositories
- release workflow for version tags
- optional deployment workflow for server-style components
- CI/CD and deployment documentation

What is not here yet:

- root `go.mod`
- `cmd/syncthing`
- hub implementation
- WSS transport code
- bearer-token auth code
- scanner, delta, and reconciler implementation

Because of that, the current automation behaves as bootstrap infrastructure:

- CI passes in bootstrap mode when no Go module is present
- release workflow becomes active once the main Go project is added
- optional deployment only becomes active when matching server code exists

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

This README is aligned with the architecture summary and the current repository
state at the same time: the target system is defined, but the implementation is
still waiting to be imported or built in this repository.
