# Task7 disposable-account verification — 2026-09-06

## Result and scope

Ten explicit Task7 commits succeeded in an isolated disposable Things Cloud
account using Things 3.23.3 (32303501), schema 301. Each commit was read back
with the exact submitted property dictionary. Native app synchronization and
read-only SQLite queries confirmed the resulting state. SDK reads confirmed
the tested values, with the Unicode replay correction described below.

This establishes a tested subset, not a complete Task7 write specification.
The production `History.Write` Task7 guard remains unchanged; normal SDK/CLI
writes remain Task6. No account credentials or private history captures are
included in this repository.

## Method

- Confirmed the disposable and original accounts had distinct history IDs.
  The original account was used only for this read-only separation check.
- Created a named control task in Things, edited its Unicode note, and set it
  to Today. Captured those Task7 events from cloud history. These are persisted
  native events, not an interception of the native outgoing HTTP request.
- Used a private test-only HTTP writer to submit explicit `e: "Task7"` events
  for one owned test UUID. It checked account separation and the current head,
  recorded each write intent, and refused automatic retries or duplicate stages.
- Reused captured native creation, note, and Today shapes. Future scheduling,
  date clearing, completion, reopening, and trash/restoration used the existing
  SDK property conventions with an explicit Task7 envelope.
- Compared cloud readback, candidate SDK output, and Things' local database
  opened read-only. Inspected creation/notes in the UI and the restored task
  after a normal app quit and relaunch. Intermediate lifecycle verification
  used the local database; it did not independently inspect every view.

## Passed cases

| Operation | Verified fields or behavior |
| --- | --- |
| Create ordinary task | Native 34-property shape; empty note; task opens in Things |
| Initial Unicode note | Captured insertion into empty note |
| Interior Unicode edit | Captured byte-offset delta after Greek text and emoji |
| Today | Native `st=1`, `sr`, `tir`, `ix`, `md` shape |
| Future schedule and deadline | `st=2`; September 10 start; September 11 deadline |
| Clear dates | Explicit null `sr`, `tir`, `dd`; `st=1` |
| Complete | `ss=3` and non-null `sp` |
| Reopen | `ss=0`, null `sp` |
| Recoverable trash | `tr=true` |
| Restore | `tr=false`; task remains open with its correct note after restart |

Native creation used the same 34 property names as the current Task6 create
payload. That observation alone does not establish equal semantics for every
field or justify changing the production envelope.

## Unicode defect and correction

Starting text: `Native Task7 note α 🚀\nSecond line.`

The native app emitted this delta:

```json
{"_t":"tx","t":2,"ps":[{"r":"Update","p":26,"l":5,"ch":3672733299}]}
```

Things produced `Native Task7 note α 🚀\nUpdated line.`. Offset 26 is the UTF-8
byte position of `Second`; its Unicode scalar position is 22 and its UTF-16
position is 23. The old rune-index decoder instead produced `SecoUpdatene.`.
Replaying the identical delta through an explicit Task7 write reproduced the
correct native result and the incorrect SDK result.

`ApplyPatches` now uses byte offsets and bounds lengths before addition to avoid
integer overflow. Replay generation 3 invalidates older CLI/SQLite derived
state. Regression tests cover the native delta, malformed numeric ranges, and
caught-up version-2 cache/database recovery with preserved SQLite audit rows.
The actual disposable-account version-2 CLI cache was also repaired by the
new binary and matched the app after restart.

## Remaining verification gaps

- Non-null `rp`/`rr`, modern recurrence and repeat-instance operations.
- Direct Task7 project/heading creation, relationship moves, tags, checklists,
  reminders, ordering under concurrency, and bulk writes.
- Permanent deletion and wire action `t=2` were not exercised in this write test.
- Conflict/retry behavior, offline concurrent edits, and multi-device convergence.
- Raw native HTTP headers/body equivalence; server persistence may normalize
  requests. No TLS interception or certificate changes were used.
- The general malformed-patch/checksum contract is not established by this
  successful Unicode example. A malformed byte range splitting a multibyte
  character can still yield invalid UTF-8; checked replay with transactional
  error propagation would require a separate change.

These limits must stay explicit in any proposal to enable Task7 writes.
