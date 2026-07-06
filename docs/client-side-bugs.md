# Client-Side Bugs — Things App Crash Investigation

## Context

After updating the SDK and building `things-cli`, tasks created by the CLI caused the Things macOS app to crash or behave erratically upon sync. The root causes were identified by comparing `things-cli` output against a Proxyman HAR capture of the real Things 3.15 client (`assets/Things_02-10-2026-10-01-09.har`).

## Bug 1: Schedule Field (`st`) Values Were Swapped (CRITICAL)

### What `st` actually means

The `st` JSON field maps to the `start` column (row 11) in Things' internal SQLite database. It is NOT a "schedule type" — it represents the **start state** of a task. The Things UI determines which view to show the task in based on the combination of `st` and the `sr`/`tir` date fields:

| `st` | Start state | `sr`/`tir` | Things UI view |
|------|-------------|------------|----------------|
| 0    | Not started | null       | **Inbox**      |
| 1    | Started     | today's date | **Today**    |
| 1    | Started     | null       | **Anytime**    |
| 2    | Deferred    | future date | **Upcoming**  |
| 2    | Deferred    | null       | **Someday**    |

### What things-cli was sending (WRONG)

```go
case "today":
    st = 2    // WRONG — 2 means "deferred" (someday)
case "someday", "anytime":
    st = 1    // WRONG for someday — 1 means "started" (anytime)
```

This produced an **invalid combination**: `st=2` (deferred) paired with `sr`/`tir` set to today's date. The Things app does not expect a "deferred" task to have today's date — this state has no valid UI representation and caused the client to crash.

### HAR evidence

The real Things app creates a "Today" task with:
```json
{
  "st": 1,
  "sr": 1770681600,
  "tir": 1770681600
}
```
Where `1770681600` is 2026-02-10 00:00:00 UTC (the day of capture).

All 5 instances of `st=1` with dates in the HAR used today's date. All 5 instances of `st=2` used future dates or null (someday).

### Fix

```go
case "today":
    st = 1  // "started" — combined with today's sr/tir = Today view
case "anytime":
    st = 1  // "started" — without dates = Anytime view
case "someday":
    st = 2  // "deferred" — without dates = Someday view
case "inbox":
    st = 0  // "not started" — Inbox view
```

### Also affected: `listTasks` today filter

The `things-cli today` command filtered tasks by `task.Schedule != 2`, which matched "someday" tasks, not "today" tasks. Fixed to check `task.Schedule == 1` AND `task.ScheduledDate == today`.

## Bug 2: SDK `TaskSchedule` Constants Were Misleading

### The problem

The SDK constants named value 0 as "Today":

```go
TaskScheduleToday   TaskSchedule = 0  // Actually Inbox!
TaskScheduleAnytime TaskSchedule = 1  // Correct
TaskScheduleSomeday TaskSchedule = 2  // Correct
```

This naming was incorrect. The value `0` maps to "not started" = **Inbox**, not Today. Today tasks use `st=1` (same as Anytime) differentiated by having `sr`/`tir` dates set.

### Fix

Renamed to reflect actual semantics:

```go
TaskScheduleInbox   TaskSchedule = 0  // Not started
TaskScheduleAnytime TaskSchedule = 1  // Started (Today when sr/tir=today, Anytime when null)
TaskScheduleSomeday TaskSchedule = 2  // Deferred

// Deprecated alias for backward compatibility
TaskScheduleToday = TaskScheduleInbox
```

## Bug 3: Timestamp Precision Loss

### The problem

`Timestamp.MarshalJSON()` serialized as integer seconds:

```go
func (t *Timestamp) MarshalJSON() ([]byte, error) {
    var tt = time.Time(*t).Unix()  // truncates to integer
    return json.Marshal(tt)        // outputs: 1496009117
}
```

The real Things API uses **fractional seconds** (e.g., `1770713623.4716659` for `cd`/`md` fields). The integer truncation could cause ordering issues when Things compares modification timestamps for conflict resolution.

Similarly, `UnmarshalJSON` was discarding sub-second precision:
```go
*t = Timestamp(time.Unix(int64(d), 0).UTC())  // nanoseconds always 0
```

### Fix

Both methods now preserve nanosecond precision:

```go
// Marshal: fractional epoch output
func (t *Timestamp) MarshalJSON() ([]byte, error) {
    tt := time.Time(*t)
    ts := float64(tt.UnixNano()) / 1e9
    return json.Marshal(ts)
}

// Unmarshal: preserve fractional part
sec := int64(d)
nsec := int64((d - float64(sec)) * 1e9)
*t = Timestamp(time.Unix(sec, nsec).UTC())
```

## Bug 4: State Aggregation Crashes (Server-Side Processing)

Two additional bugs in the SDK's state aggregation layer caused panics when processing the event stream:

### 4a: Nil pointer in `hasArea()` (state/memory/memory.go)

When a parent task was deleted (via tombstone or `ItemActionDeleted`), child tasks still referenced the parent UUID in `ParentTaskIDs`. The recursive `hasArea()` lookup followed `state.Tasks[parentID]` which returned `nil`, then accessed `.AreaIDs` — panic.

**Fix:** Added `if task == nil { return false }` nil guard.

### 4b: Slice out-of-bounds in `ApplyPatches()` (notes.go)

When a note's delta patch had `Position` exceeding the current string length (possible when patches arrive out of order during sync), the slice operation `runes[:p.Position]` panicked.

**Fix:** Clamped `Position` and `end` to `len(runes)` before slicing.

## Verification Method

All bugs were identified by:
1. Capturing the real Things 3.15 client traffic via Proxyman (91 requests)
2. Extracting all POST commit payloads from the HAR file
3. Comparing field values (especially `st`, `sr`, `tir`) between the HAR capture and `things-cli` debug output
4. Cross-referencing against the Things SQLite schema comments in `types.go`

## Bug 5: Things.app Crashes on Cloud-Synced Tasks — RESOLVED

### Root Cause: UUIDs Were Not Base58-Encoded

The crash was **not** caused by notes, `md` timestamps, field ordering, or the two-commit pattern. **The root cause was invalid UUID encoding.**

Things.app's `BSSyncValueEncoder.decode()` in `Base.framework` calls `BSIdentifierFromBase58String()` to parse each UUID key from sync data. The SDK was generating UUIDs using a fake Base62 mapping (modulo of raw bytes against an alphanumeric alphabet), which produced strings that **looked** plausible but were **not valid Base58**.

When Things.app tried to decode these UUIDs:
1. `BSIdentifierFromBase58String()` returned an invalid result
2. The decode loop's array bounds check failed
3. `EXC_BREAKPOINT` (Swift `brk #1`) at Base.framework offset `0xA6194`

### How We Found It

1. **Crash report analysis** (`~/Library/Logs/DiagnosticReports/Things3-2026-02-10-124613.ips`) — the crash was in `Base.BSSyncValueEncoder.decode()`, NOT `insertTaskWithUUID:usingTombstones:` as initially assumed
2. **Hopper disassembly** of `Base.framework` — traced the crash to the `BSIdentifierFromBase58String()` call and its bounds-check trap
3. **Register state** at crash — `x23 = 0xFFFFFFFF` (error sentinel from failed Base58 decode), `x22` non-zero (ruling out the nil-operation path)
4. **HAR capture comparison** — real Things UUIDs are 21-22 character Base58 strings (e.g., `Q9sihFX2SsvGaz6vv4J2Hf`), not standard UUID format

### The Fix

Replace the broken UUID generation:

```go
// BEFORE (broken — fake Base62, not valid Base58):
chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
result := make([]byte, 22)
for i := 0; i < 22; i++ {
    result[i] = chars[int(bytes[i%16])%len(chars)]
}

// AFTER (correct — proper Base58 encoding with Bitcoin alphabet):
const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
n := new(big.Int).SetBytes(uuid[:])
base := big.NewInt(58)
for n.Sign() > 0 {
    n.DivMod(n, base, mod)
    encoded = append(encoded, alphabet[mod.Int64()])
}
// reverse for big-endian
```

Key differences:
- Base58 alphabet excludes `0`, `O`, `I`, `l` (avoids visual ambiguity)
- Proper base conversion via `math/big` division, not modulo mapping
- Produces 21-22 character strings matching Things' native format

### Verification

Tested on account **things33** (2026-02-10):
- Task creation (no note) — syncs and displays correctly
- Task creation with note (single commit) — syncs and displays correctly
- Task editing, completion, trashing — all work
- Project with headings and subtasks — all work
- Tag and area creation — all work

The "note bug" was a red herring. Notes in single-commit creates work fine — every test that "failed on notes" actually failed because the UUID was invalid.

### Why Corrupted Accounts Cascade-Failed

Once an item with an invalid UUID is written to the cloud history, **every subsequent sync attempt crashes**. The server stores UUID keys as opaque strings, so it accepts anything. But the client must decode every UUID in the history during sync. One bad UUID poisons the entire history, which is why things9-23 all became permanently unusable.

## Bug 6: Tasks Under Projects/Headings/Areas Default to Inbox

### The Problem

When creating a task with `--project`, `--heading`, or `--area`, the CLI defaulted to `st=0` (inbox). This caused tasks to appear in the Inbox instead of under their parent project/heading/area in Things.app.

### Root Cause

Tasks placed into a project, under a heading, or within an area have already been "triaged" — they shouldn't land in Inbox. The real Things app creates these tasks with `st=1` (anytime/started).

### Fix

Auto-set `st=1` when `--project`, `--heading`, or `--area` is provided, unless `--when` is explicitly set:

```go
// --project
if v, ok := opts["project"]; ok && v != "" {
    pr = []string{v}
    if _, hasWhen := opts["when"]; !hasWhen {
        st = 1
    }
}
// Same pattern for --heading and --area
```

This follows the same principle as the heading fix (Bug 5): structural elements inside projects and areas are never "inbox" items.

## Bug 7: Incremental Sync Returns 500 Errors (2026-02-11)

### The Problem

The persistent sync engine (`sync` package) was getting 500 errors on incremental syncs when fetching items from Things Cloud. The error occurred on the second or third batch of items, not the first.

### Root Cause

The server returns 500 when `start-index > current-item-index` (out of bounds request). The SDK was calculating the next page's start index incorrectly.

Items in the `/items` response are nested maps:
```json
{
  "current-item-index": 201,
  "items": [
    {"uuid1": {...}, "uuid2": {...}},  // 1 server item, 2 entities
    {"uuid3": {...}},                   // 1 server item, 1 entity
    ...
  ]
}
```

During parsing, these nested maps get *expanded* — a single server item with multiple entity keys becomes multiple `Item` structs. The SDK was using `len(expandedItems)` to calculate the next start index, but the server's `current-item-index` advances by the *outer array* count.

**Example**: Request `start-index=177` returns 24 server items that expand to 32 entities, with `current-item-index=201`.
- Wrong: `177 + 32 = 209` → server returns 500 (209 > 201)
- Right: use server's `current-item-index` directly → 201

### The Fix

Two changes to `sync/sync.go`:

1. **Pre-check**: Call `GET /history/{id}` to get `latest-server-index` before fetching items. Skip if stored cursor >= server index (nothing new to fetch).

2. **Pagination**: Use `s.history.LatestServerIndex` (populated from response's `current-item-index`) instead of calculating `startIndex + len(items)`.

```go
// Before (wrong):
startIndex = startIndex + len(items)  // items is expanded count

// After (correct):
startIndex = s.history.LatestServerIndex  // server's actual index
```

### Additional Discovery

The `/items` endpoint doesn't require authentication — you can curl it directly. Things.app also caches the history ID locally and skips the `/account/{email}` call on incremental syncs. The SDK now matches this pattern.

### Verification

Tested on account **things36** (2026-02-11):
- Fresh sync from index 0 — works
- Incremental sync — works (no 500)
- Multiple consecutive syncs — works
- After external changes in Things.app — detects and fetches delta correctly

## Bug 8: Edit Command Doesn't Set st=1 When Adding Project/Area/Heading (2026-02-12)

### The Problem

The `things-cli edit` command's `--project`, `--area`, and `--heading` flags only set their respective fields without updating `st` (schedule). This caused tasks to remain in Inbox (`st=0`) instead of moving to Anytime (`st=1`) when organizing via edit.

### Root Cause

The edit path was incomplete compared to the create path. When creating a task with `--project`/`--area`/`--heading`, the CLI auto-sets `st=1`. But when editing an existing task to add these, only the parent reference was being updated.

### Evidence

HAR capture (`assets/task_inbox-to-project.har`) shows Things.app sends both fields when moving a task from Inbox to a project:

```json
{"pr":["BVU8qZ9dNjrdxLvDHPvfDS"],"st":1,"ix":4712,"md":...}
```

### The Fix

Updated `cmdEdit()` to set `st=1` (Anytime) when `--project`, `--area`, or `--heading` is provided, unless `--when` is explicitly set:

```go
if v, ok := opts["area"]; ok && v != "" {
    u.Area(v)
    if _, hasWhen := opts["when"]; !hasWhen {
        u.Schedule(1, 0, 0) // Anytime
    }
}
// Same pattern for --project and --heading
```

### Verification

Tasks moved from Inbox to project/area/heading via `things-cli edit` now correctly appear under their parent instead of remaining in Inbox.

## Bug 9: Base58 Encoder Dropped Leading Zero Bytes (2026-07-06)

### The Problem

Even after the Bug 5 fix switched UUID generation to Base58, accounts still intermittently accumulated items that crashed Things.app. The failure was probabilistic: everything worked for dozens of creates, then one item poisoned the history.

### Root Cause

The Base58 encoders in `things-cli` and `example/` (duplicated code) converted the UUID through `math/big`, which normalizes away leading zero bytes. Canonical Base58 represents each leading `0x00` byte as a leading `1` character; without that padding, a UUID whose first byte is zero encodes one character short and no longer decodes to 16 bytes. Byte 0 of a v4 UUID is fully random, so **~1 in 256 generated identifiers was malformed** — feeding the exact decoder (`BSIdentifierFromBase58String`, see Bug 5) whose array-bounds failure crashes the app.

### Evidence

- All identifiers captured from real Things.app traffic round-trip through a canonical encoder unchanged (`VJ1edXTP9q3PmFDUuy8EQh`, `FQxaqvLBkbR5q2Q5oRoknc`, `BVU8qZ9dNjrdxLvDHPvfDS`) — Things uses standard Bitcoin-style Base58 with leading-`1` padding.
- Empirical: a UUID starting `0x00` encoded by the old code round-trips to only 15 bytes.

### The Fix (v0.3.0)

Moved a single canonical implementation into the SDK (`base58.go`): `EncodeUUID` emits one `1` per leading zero byte; `DecodeUUID` rejects anything that doesn't decode to exactly 16 bytes; `NewUUID()` is the one true generator. Defense in depth:

- `History.Write()` validates every item UUID (and rejects duplicate UUIDs per commit) before anything reaches the server
- `things-cli` validates all user-supplied identifiers (`--uuid`, `--project`, `--area`, `--heading`, `--tags`, positional targets)
- Related hole closed: `create --type heading --when inbox` could still emit `tp=2, st=0` (Bug 6's crash payload) because the `--when` switch ran after the type switch; projects and headings now force `st=1` after all schedule processing

### Verification

TDD with real-traffic round-trip vectors; forced leading-zero-byte cases in the round-trip suite; 2000-draw canonical-validity test on the generator.

## Files Changed

| File | Changes |
|------|---------|
| `cmd/things-cli/main.go` | Fixed `st` values, fixed `generateUUID()` to use Base58, added `create-area` command, auto-set `st=1` for --project/--heading/--area (create and edit - Bug 8), added `batch` command for multiple ops in one HTTP request |
| `syncutil/syncutil.go` | New package with shared utilities for sync-based CLI tools |
| `example/main.go` | Base58 UUID encoding, removed `ModificationDate` from creates |
| `types.go` | Renamed `TaskScheduleToday` → `TaskScheduleInbox`, fixed `Timestamp` marshal/unmarshal |
| `types_test.go` | Updated marshal test for fractional output |
| `itemaction_string.go` | Updated stringer for renamed constant |
| `state/memory/memory.go` | Nil guard in `hasArea()` |
| `notes.go` | Bounds clamping in `ApplyPatches()` |
| `notes_test.go` | Regression tests for edge cases |
| `state/memory/memory_test.go` | Regression tests for tombstone and orphan cases |
| `sync/sync.go` | Pre-check server index, fix pagination to use server's `current-item-index`, added `getServerIndex()` |
| `histories.go` | Added `HistoryWithID()` helper for cached history ID |
