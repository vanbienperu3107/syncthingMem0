# Build 2 independent brands: client and server

This repo now contains two runtime brands:

- `client` -> `cmd/syncthing`
- `server` -> `cmd/stdiscosrv` + `cmd/strelaysrv`

You can build each brand independently with dedicated scripts.

## 1) Build in bash

```bash
# Client only
./scripts/build-brands.sh client

# Server only
./scripts/build-brands.sh server

# Both
./scripts/build-brands.sh both

# Build multiple targets
GOOS_LIST=linux,windows GOARCH_LIST=amd64,arm64 ./scripts/build-brands.sh both

# Build server with all infra services (optional)
INCLUDE_INFRA=true ./scripts/build-brands.sh server
```

Output folders:
- `dist/brands/client/<goos>/<goarch>/`
- `dist/brands/server/<goos>/<goarch>/`

## 2) Build in PowerShell

```powershell
.\scripts\build-brands.ps1 -Mode client
\scripts\build-brands.ps1 -Mode server
\scripts\build-brands.ps1 -Mode both
\scripts\build-brands.ps1 -Mode both -GoOsList "linux,windows" -GoArchList "amd64,arm64"
\scripts\build-brands.ps1 -Mode server -IncludeInfra
```

### Notes
- `server` brand by default builds only relay + disco services (core server).
- `-IncludeInfra` adds: `strelaypoolsrv`, `stupgrades`, `stcrashreceiver`, `ursrv`.
- You can run client and server artifacts in separate hosts.
