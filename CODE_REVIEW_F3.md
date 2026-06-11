# CODE REVIEW F3

## Findings

- No open blocking findings after self-review.
- Fixed during review: incremental cache is now only passed to the hasher when `Incremental.Enabled` is true.
- Fixed during review: digest cache overwrite no longer evicts an unrelated entry when the cache is already full.

## Tests

- Added snapshot unit coverage for lookup, subtree reuse, dirty subtree replacement, and dirty/ancestor matching.
- Added digest cache unit coverage for hit/miss, clone safety, TTL expiry, eviction, and prefix removal.
- Added minimal scanner integration coverage for the incremental cache-hit path in `TestWalkIncrementalCacheHit`.
- `git diff --check`: pass.
- `go test -count=1 ./lib/scanner`: not run because `go` is not available in PATH (`GO_NOT_FOUND`).

## Residual Risks

- Go formatting and compile/test validation still need to run in an environment with the Go toolchain.
- Snapshot is in-memory only; restart falls back to full scan/cache rebuild as intended.
- Current integration reuses cached block lists for matching size/mtime; platform-specific file identity is not included yet.
