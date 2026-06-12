# Windows Installer Build & Deploy

Each Windows release includes:

- `client|server-<target>-windows-amd64-*.zip` (archive)
- `client|server-<target>-windows-amd64.exe` (NSIS installer)
- `client|server-<target>-windows-amd64.msi` (WiX MSI installer)

The installers:

- Copy the binary to `Program Files\<Product Name>`
- Create config folders:
  - Client: `%PROGRAMDATA%\Syncthing-Client`
  - Discovery Server: `%PROGRAMDATA%\Syncthing-Discovery`
  - Relay Server: `%PROGRAMDATA%\Syncthing-Relay`
- Create default sync folder for client:
  - `%PROGRAMDATA%\Syncthing-Client\Sync`
- Create matching Windows services (auto-start):
  - `SyncthingClient`
  - `SyncthingDiscovery`
  - `SyncthingRelay`

## Install quickly

1. Run `.exe` or `.msi` as Administrator.
2. Verify service status:
   - `sc query SyncthingClient` (or `SyncthingDiscovery`, `SyncthingRelay`)
3. Open Syncthing UI if needed (default URL per your config).

## Uninstall

Open `Add or Remove Programs` or run uninstall from Windows app list.
