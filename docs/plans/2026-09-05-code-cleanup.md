# Code cleanup after v0.5.0

Baseline: `0c748970026507bbbf75936e38ffbc8ce5ed23dd`. `develop` has been rebased onto the released `main` commit.

## First change: request construction and redundant code

1. Add regression tests for invalid account/history identifiers in `Client.Verify`, `History.Items`, and `History.Write`. Require errors instead of panics, no HTTP requests, and unchanged history cursors.
2. Lock the valid commit request headers and query parameters with a regression test before removing duplicate assignments.
3. Move request-construction error checks before any request access. Keep existing payloads, response handling, status errors, and authentication behavior.
4. Remove write headers already supplied by `Client.do`. Replace the test-only `asHTTPError` wrapper with direct `errors.As` calls.
5. Delete the duplicate task-title assignment and commented-out import in memory replay, covered by the existing title-update and replay tests. Delete the unused `bufio` import and placeholder reference in the soak tool; these have no runtime behavior.
6. Run focused tests, then build, race-enabled tests, lint, and diff checks. Get an independent review before opening a draft PR against `develop`.

No dependencies or new production abstractions are needed. Changes to the persistent sync schema, CLI architecture, public error types, or debug logging policy belong in separate work.

## Audit findings and follow-up

The following findings are based on code inspection, not live-account testing. They remain outside this first patch. Address correctness before larger structural refactors.

### Next priority: CLI write validation

- `cmd/things-cli/main.go`, `cmdCreateArea` and `cmdCreateTag`: area tag references and parent-tag identifiers bypass `validateIdentifierOpts`. `History.Write` validates only envelope identifiers. Reuse and extend the existing validator, test invalid relationship IDs with a fake server, and assert no commit is sent. Also check that tag whitespace handling produces the same values that validation accepts.
- `parseDate`, task create/edit, and batch handling silently omit invalid dates or ignore unknown schedule names. Reject invalid options before a write, with tests covering both individual and batch commands. This changes previously permissive CLI behavior and should be documented.

### Sync correctness

- `sync/detect.go`, `detectTaskChanges`: project/area assignment changes are never compared, even though `TaskAssignedToProject` and `TaskAssignedToArea` are public types consumed by `thingsync`. Define assignment, reassignment, and removal event payloads before implementation; test all three.
- `sync/store.go`, `logChange`, and `sync/sync.go`, `ChangesSince`: storage and query cursors use integer Unix seconds with a strict greater-than comparison. A later event in the same second as a cursor is omitted. Choose a precise cursor/storage contract and a migration strategy; test a cursor between two same-second changes.
- `sync/process.go`, `processTombstone`: all four entity lookups discard errors. A lookup failure can be treated as absence, letting a batch advance past an unapplied deletion. Propagate failures and verify transaction rollback and unchanged sync position under an injected lookup error.
- `state/memory/memory.go`, `hasArea`: recursive parent traversal has no cycle detection. Add a visited set and tests for self-cycles, multi-node cycles, and a cycle with another parent reaching an area.

### Decisions needed before broader refactoring

- Memory replay deliberately skips malformed payloads despite returning `error`. Decide whether replay remains best-effort with diagnostics or becomes strict, including partial-state semantics. Do not simply change `continue` to `return` without defining what happens to already-applied items.
- CLI date writes encode a local calendar date at UTC midnight, but `listTasks` compares against the current UTC day. Around local midnight this can disagree with `--when today`. Specify date-only wire semantics, then test positive and negative time zones with a fixed clock; do not apply a blanket UTC-to-local conversion.
- SQLite legacy identifier remapping still needs a database migration/rebuild decision, as documented in v0.5.0.
- Checklist modification dates are applied in memory by the sync processor but are absent from the persistence schema. Area tags and area/tag ordering are also only partially modeled. Group schema changes into a separately tested migration.
- `thingsync` discards query errors in multiple output builders, which can report incomplete data as a successful result. Refactor those functions to return errors, with closed-database tests, as a separate CLI change.

### Lower-priority simplification candidates

- `cmd/soak`, `runCLI`: every caller ignores its JSON result, and JSON decoding errors are already discarded. Remove the unused return and decoding after locking subprocess success/failure reporting with an offline test.
- Several debug commands have personal hardcoded task IDs. Confirm continued maintainer use before deleting these entry points; avoid turning them into a new CLI framework merely to retain them.
- Consolidate or synchronize CLI command documentation, and define deterministic ordering for map-backed area/tag listings if callers need stable output.
- SDK HTTP status errors mix strings and `HTTPError`; debug output also dumps full requests/responses. Handle error compatibility and logging/redaction policy in a separate patch.

## Verification of the first patch

Before editing production code, the new regression test reproduced nil-request panics in all three affected methods. The valid commit metadata test passed on the baseline. Existing memory replay/title tests passed before removing the duplicate assignment.

After cleanup, `go build ./...`, `go test -race ./...`, `make lint` (zero issues), and `git diff --check` all pass. No live-account or soak test was run.
