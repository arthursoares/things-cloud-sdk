# Stress Testing the Write & Sync Paths

This SDK's failure mode was **cumulative history corruption**: a rare bad
identifier (roughly 1 write in 256) made Things.app crash on its next sync,
and a poisoned history cannot be repaired. The tests below attack that
class three ways — two fully local and automated, one live against a
disposable account.

## 1. Fuzzing the identifier layer (local, automated)

`base58_fuzz_test.go` throws millions of random inputs at the Base58
encode/decode/validate functions:

```bash
go test -run '^$' -fuzz=FuzzUUIDRoundTrip       -fuzztime=60s .
go test -run '^$' -fuzz=FuzzValidateUUIDIsSound -fuzztime=60s .
go test -run '^$' -fuzz=FuzzDecodeUUIDNeverPanics -fuzztime=30s .
```

- `FuzzUUIDRoundTrip` — every 16-byte UUID must survive `EncodeUUID → DecodeUUID` unchanged and encode canonically. This is the exact property the old encoder violated for leading-zero bytes; the fuzzer hits that case within a few iterations.
- `FuzzValidateUUIDIsSound` — treats `ValidateUUID` as the security boundary: anything it accepts must decode to exactly 16 bytes and be canonical. If it ever accepts a corrupting identifier, `Write()` could poison a history.
- `FuzzDecodeUUIDNeverPanics` — arbitrary caller/server strings must never crash the SDK.

The seed corpus alone catches a reverted encoder in <0.1s.

## 2. Chaos-testing the sync engine (local, automated)

`sync/chaos_test.go` (`TestChaos_MidSyncFailuresNeverCorrupt`) serves a
synthetic history through a fault-injecting fake server and, across 40
trials, kills the sync partway through at random batch boundaries (via
truncated responses and a 500), then recovers. After each recovery it
asserts:

- the cursor sits exactly at the last **committed** batch (never past the failed one),
- `change_log` has **zero** duplicate `(server_index, entity_uuid, change_type)` rows,
- the final task state is byte-identical to a clean single-shot sync.

```bash
go test -run TestChaos ./sync/
```

Reverting the atomic-cursor fix makes this fail immediately
(`cursor = 0, want 10`), confirming it tests the real invariant.

## 3. Live soak against a disposable account (manual)

Nothing local can prove Things.app *accepts* our bytes. This step does.

> **Use a throwaway test account only.** Never a real account — the soak
> writes and then trashes/purges hundreds of items.

### a. Run the soak driver

It builds `things-cli` and drives every write command through it (the
real user write path): create (tasks, projects, headings, areas, tags),
edit, move-to-today, complete, add-checklist, batch (multi-op single
commit), trash, and purge — **deliberately including identifiers built
from leading-zero-byte UUIDs**. It verifies every write landed by reading
the account back through the sync engine, then cleans up.

```bash
export THINGS_USERNAME='test-account@example.com'
export THINGS_PASSWORD='...'
export THINGS_SOAK_CONFIRM=yes-throwaway-account
go run ./cmd/soak --cycles 50
```

A `PASS` means every write was accepted and re-synced. A `FAIL` means the
account may now hold data Things.app cannot decode — stop and inspect
before opening any client on it.

### b. Watch the live app while (or after) it runs

On the machine where **Things.app is signed into the same test account**:

```bash
# Classify any pre-existing crash reports
scripts/things-crash-watch.sh --scan

# Watch for new crashes + stream the sync log during the soak
scripts/things-crash-watch.sh
```

The script watches `~/Library/Logs/DiagnosticReports` for new
`Things*.ips` reports and classifies each against the known
sync-decode crash signatures (`BSIdentifierFromBase58String`,
`BSSyncValueEncoder`, `decodeToOneRelation`, `LegacySCHistoryPerformSync`
— see [`client-side-bugs.md`](client-side-bugs.md)). A backtrace hitting
any of them is the history-poisoning class this SDK exists to prevent.

### c. If Things.app does crash — attach a debugger

```bash
# Symbolicate the newest crash report
ls -t ~/Library/Logs/DiagnosticReports/Things*.ips | head -1

# Or attach lldb to a running instance to catch it live
lldb -p "$(pgrep -x Things3 || pgrep -x Things)"
(lldb) continue
# on crash:
(lldb) thread backtrace all
(lldb) image lookup --address <faulting-address>
```

The crashing frame plus the offending item (the last thing the soak wrote
before the crash) pinpoint which wire-format rule was violated. Add a
regression test at the SDK boundary so `Write()` refuses that shape.

### d. The final human check

Even on a clean `PASS` with no crash reports, open Things.app on the test
account and confirm the created items appear and sync across a second
device. That end-to-end observation is the real acceptance test.
