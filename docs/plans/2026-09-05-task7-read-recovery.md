# Task7 read support and recovery

## Scope and decisions

Implement issue #29 from the v0.5.0 develop baseline. The user approved the proposed approach: retain the active SQLite audit log and rebuild derived state through staged replay. Issue #26's write scheduling is a separate branch/PR. Existing Task6 writes remain unchanged.

Protocol evidence comes from Things 3.23.3 (build 32303501), read-only cloud inspection, and a consistent local database snapshot. The app normalizes Task6 to Task7 without modifying the operation's property dictionary. `sr` is startDate; `tir` is a distinct todayIndexReferenceDate. Modern repeater fields remain incompletely verified and are not a new write contract.

## Plan before production edits

1. Add failing fixtures for Task7 create/partial update, mixed Task6/Task7 identity, notes, relationships, nullable fields, deletion, and future unsupported task kinds.
2. Add an explicit Task7 read kind and a shared read-only payload helper for null-versus-omitted fields. Preflight unsupported task kinds before a memory batch mutates. SQLite must roll back rejected batches.
   Add a serialized-envelope guard in `History.Write` so the new Task7 read kind and future task kinds cannot reach the network, including through custom envelope implementations. Retain the existing write kinds and full Task6 wire regression coverage.
3. Remove SQLite's incorrect `tir` override of `sr`, with a regression distinguishing the two dates.
4. Introduce a persisted replay generation independent of structural schema. Check it before caught-up returns. Fresh databases use the current generation; old populated state requires recovery.
5. Take a consistent, permission-restricted SQLite backup. Replay cloud history from zero into empty staging state. Preserve old change-log rows/IDs/timestamps; suppress historical replay events below the original next-batch cursor. Install rebuilt entities, junctions, cursor, generation, and only genuinely new log entries in one transaction after checking that the original database has not advanced concurrently.
6. Keep the old database unchanged on backup, fetch, decode, progress, or installation failure. Test successful retry, note-delta single application, old-log retention, cutoff boundaries, idempotence, and concurrent-state protection.
7. Bump the CLI cache generation, back up obsolete cache bytes, and replace the cache atomically only after successful complete replay. Preserve usable old data on failure and report unsupported task kinds without private payloads.
8. Run build, focused regressions, race tests, lint, and independent review. Re-run the read-only comparison using the actual implementation with disposable local state, documenting remaining gaps. Publish a draft PR and update issue #29.

No dependencies, cloud mutations, or destructive migration of the user's live Things.app database are needed. Private captures and credentials stay outside the repository. Recovery of the user's installed SDK cache/database is tested on copies first.

## Verification record

- New backend/cache/write-guard regressions reproduced the missing Task7 state, stale caught-up cache, unsafe replacement, and unverified write paths before the fixes.
- Independent review found malformed-note acceptance and repeated full-backup accumulation. Note values are now checked before mutation; every SQLite attempt makes a fresh validated snapshot and removes only its own full-content duplicate. A restored database with the same cursor but different audit content retains its own backup.
- Real-capture upgrade using the actual implementation restored seven missing tasks and all three inbox tasks. The 265 scheduled-date mismatches disappeared; all 5,352 existing audit rows remained byte-identical, recovery returned no historical notifications, and the next sync returned no changes.
- Read-only live CLI validation rebuilt a disposable version-1 cache: old reader inbox 0, new reader inbox 3, Things.app inbox 3. A repeated read stayed at 3 and the original cache had an exact 0600 backup.
- Three orphan Task6 modification-only records remain separate from the Task7 repair. Modern repeater semantics and actual Task7 project/heading wire variants remain incompletely observed; those variant tests are synthetic.
- The CLI uses optimistic cache-change detection plus atomic replacement, not a cross-process lock. Concurrent complete snapshots may replace one another and catch up on a later read. Separate cache paths avoid shared-writer contention.
- No live cloud writes have been made. The separate scheduling PR and combined stack require a controlled disposable-account/Things.app check before treating the write behavior as app-verified. Process-kill/power-loss injection has not been performed.
