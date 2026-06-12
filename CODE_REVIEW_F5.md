# CODE REVIEW F5

## Findings

- No obvious out-of-scope edits found. Changes are limited to folder DB reconcile/global selection, ancestor metadata storage/schema, config flag plumbing, and focused tests.
- LWW behavior is guarded by `UseLWWReconciler` and defaults off. Existing vector ordering remains active when the flag is disabled.
- Ancestor metadata is committed only after local and at least one remote have the same global version, so failed or partial pulls should not advance the ancestor.

## Tests

- Added SQLite tests for LWW reconcile behavior, flag-off compatibility, one-sided changes, ancestor store CRUD, and ancestor schema creation.
- Added config tests for default false, JSON true, and XML true paths.
- `git diff --check` run.
- `go test` could not run because the Go toolchain is not available in PATH on this machine.

## Residual Risks

- Full compile and gofmt verification are blocked until Go is installed or added to PATH.
- Integration behavior across multiple live devices still needs a real sync test once the toolchain/runtime is available.
