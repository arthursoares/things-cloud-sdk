# CLI write validation

Based on the v0.5.0 `develop` branch. The request-handling cleanup remains a separate draft PR.

## Plan before implementation

1. Add subprocess tests against a fake Things Cloud endpoint. Invalid inputs must exit with an actionable error and send no commit. Include a batch with a valid operation followed by an invalid one to protect all-or-nothing submission.
2. Reuse the existing identifier validator for area tags and parent tags, and validate purge target IDs before constructing tombstones. Check tag IDs exactly as written; do not validate trimmed text and then send the untrimmed value.
3. Add one shared validator for the supported task schedule names, task types, and calendar dates. Apply it before task create/edit payload construction and in batch create/edit handling. Preserve existing schedule precedence and date encoding for valid inputs.
4. Validate batch create options after merging `extra`, so overrides cannot bypass validation. Validate purge IDs as well. Reject ignored `extra` options on non-create operations and unsupported keys inside create `extra`. Reject unknown batch JSON fields and trailing input to catch unsupported or misspelled fields and incomplete submissions.
5. Document the stricter validation and existing batch `extra.scheduled` support. Keep CLI parsing, payload builders, the SDK API, and date/time-zone semantics otherwise intact; no new dependencies.
6. Run new regressions before production edits, then focused wire tests, build, race tests, lint, and independent review. Open a separate draft PR against `develop`.

## Compatibility

Previously accepted malformed dates, unrecognized schedule/type names, whitespace-padded tag IDs, and invalid relationship references now fail before a commit is sent. Absent options and empty optional top-level batch strings retain their existing defaults; explicitly empty identifiers in CLI options or `extra` fail validation. Batch create retains its existing `extra` override precedence, but final values must pass validation. Non-create operations reject even an empty `extra` object, while JSON null is treated as absent. Batch scheduling uses `extra.scheduled`; this patch does not add a top-level `scheduled` field or new edit scheduling features.

## Verification

Before production edits, subprocess regressions demonstrated rejected-input cases exiting successfully and sending commits; valid cases passed on the baseline. After implementation, the CLI subprocess/wire tests, `go build ./...`, `go test -race ./...`, `make lint` (zero issues), and `git diff --check` pass. The tests use a fake cloud endpoint; no live-account writes or soak test were run.
