# CODE REVIEW F1

## Findings

- Fixed: WSS factories initially registered both `ws` and `wss`, but plain `ws` cannot provide the TLS peer certificate required by the connection service identity checks. Registry hooks now expose only `wss`.
- No remaining blocking code-review findings in the WSS transport slice.

## Tests

- Added `wsConn` unit coverage for read/write, multiple WebSocket messages, large messages, and concurrent read/write.
- Added WSS to the existing connection establishment test so the real listener and dialer factory path is exercised.
- `git diff --check` passes.
- `go test ./lib/connections/... -count=1 -run "TestWSConn|TestFixupWSURI|TestConnectionEstablishment|TestGetDialer"` could not run because `go` is not available in PATH in this environment.

## Residual risks

- Full hub routing/auth is outside this F1 slice; `HubURL` only adds a WSS dial target.
- Compile/test verification still needs a local Go toolchain.
