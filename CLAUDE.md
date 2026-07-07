# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go SDK for the Things 3 cloud API (Cultured Code). This is a reverse-engineered, unofficial SDK — there is no official API documentation. The client mimics `ThingsMac/32209501` and sends a base64-encoded `things-client-info` device metadata header.

## Build & Test Commands

```bash
go build -v ./...          # Build all packages
go test -v ./...           # Run all tests
go test -v -run TestName   # Run a single test
go test -v ./state/memory  # Run tests for a specific package
go test -v ./sync          # Run sync engine tests
go generate                # Regenerate stringer methods (itemaction_string.go)
```

## Architecture

All source code lives at the package root (`package thingscloud`, module `github.com/arthursoares/things-cloud-sdk`), with three sub-packages:
- `state/memory` — In-memory state aggregation (for testing/simple use)
- `sync` — Persistent SQLite-backed sync engine with semantic change detection
- `syncutil` — Shared utilities for sync-based CLI tools

### Core Design: Event-Sourced Sync

The SDK models all changes as immutable **Items** (events). A **History** is a sync stream identified by a UUID. The client pushes/pulls Items through Histories to stay in sync with the Things Cloud server.

- **`client.go`** — HTTP client with `ClientInfo` header, configurable `Debug` logging, 60s timeout, and typed `HTTPError` for non-OK responses; base endpoint `https://cloud.culturedcode.com`
- **`base58.go`** — Canonical Base58 identifier handling: `NewUUID()`, `EncodeUUID`/`DecodeUUID`, `ValidateUUID`. Always generate identifiers through this API.
- **`histories.go`** — History CRUD and sync operations (list, create, delete, read/write items with ancestor indices). `Write()` accepts multiple items for batching and rejects invalid or duplicate UUIDs before POSTing.
- **`items.go`** — Item construction: every mutation (create/modify/delete) on a Task, Area, Tag, CheckListItem, or Tombstone produces an Item
- **`types.go`** — Domain types: `Task` (with `TaskType` enum: Task/Project/Heading), `Area`, `Tag`, `CheckListItem`, `Tombstone`, plus custom JSON types (`Timestamp`, `Boolean`)
- **`notes.go`** — Structured `Note` type with full-text and delta patch support (`ApplyPatches`)
- **`repeat.go`** — Recurring task date calculation (daily/weekly/monthly/yearly, with end conditions)
- **`helpers.go`** — Pointer helpers (`String()`, `Status()`, `Schedule()`, `Time()`, `TaskTypePtr()`)
- **`account.go`** — AccountService for signup, confirmation, password change, deletion
- **`app_instance.go`** — Device registration for APNS push notifications

### Sync Engine (`sync/`)

The `sync` package provides a persistent sync engine that:
- Stores state in SQLite with WAL mode for performance
- Detects semantic changes (40+ change types: TaskCreated, TaskCompleted, TaskMovedToToday, etc.)
- Tracks change history for behavioral analysis (reschedule patterns, task age)
- Uses transaction batching for fast initial syncs (4000+ items in ~2.5 seconds)

Key types:
- **`Syncer`** — Main sync controller with `Sync()`, `State()`, `ChangesSince()` methods
- **`State`** — Query interface: `TasksInInbox()`, `TasksInToday()`, `AllTasks()`, etc.
- **`Change`** — Interface for semantic changes with `ChangeType()`, `EntityUUID()`, `Timestamp()`

### State Aggregation (`state/memory`)

`state/memory` provides an in-memory store that aggregates Items into a queryable hierarchy: Areas → Tasks → Subtasks → CheckListItems. Key queries: `Projects()`, `Headings()`, `TasksByHeading()`, `TasksByArea()`, `Subtasks()`, `CheckListItemsByTask()`. It is **not thread-safe**.

Use `state/memory` for testing or simple scripts. Use `sync` for production apps needing persistence and change tracking.

### Shared Utilities (`syncutil/`)

The `syncutil` package provides shared utilities for sync-based CLI tools:
- `FilterChanges()`, `FilterChangesPrefix()` — Filter changes by type
- `DaysSinceCreated()`, `CountMoves()`, `TaskAge()` — Task analytics
- `BuildDailySummary()` — Daily activity stats (completed, created, moved)

### CLI Tools (`cmd/`)

See `cmd/README.md` for detailed documentation. Key tools:
- **`things-cli`** — Full CRUD operations (create, edit, complete, trash tasks)
  - Supports `batch` command for multiple operations in one HTTP request
- **`thingsync`** — JSON-based sync with workflow views (today, inbox, review, patterns)
- **`synctest`** — Human-readable sync output for testing

### Test Infrastructure

Tests use `httptest.Server` with pre-recorded JSON responses in the `tapes/` directory. Tests use `t.Parallel()`.

### Code Generation

`types.go` contains `//go:generate stringer -type ItemAction,TaskStatus,TaskSchedule`. Run `go generate` after modifying these enum types. Do not hand-edit `itemaction_string.go`.

### Wire Format Rules (Crash-Critical)

Violating these corrupts the sync history and crashes Things.app during sync — and a poisoned history cannot be repaired, only abandoned:

- **UUIDs must be Base58-encoded** (Bitcoin alphabet: `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz` — no `0`, `O`, `I`, `l`). Things.app decodes with `BSIdentifierFromBase58String()`; standard UUID strings crash it.
- **`md` (modification date) must be `null` on creates.** Set a timestamp only on updates.
- **Headings (`tp=2`) must have `st=1`** (anytime), never `st=0` (inbox). Tasks under projects/headings/areas should also default to `st=1`.
- **Kind strings**: `Task6`, `Tag4`, `ChecklistItem3`, `Area3`, `Tombstone2`. Using `Area2` is silently ignored by Things.app.

See `docs/client-side-bugs.md` for the full crash investigations behind each rule.

### Schedule Field (`st`) Mapping

The `st` JSON field maps to the `start` column in Things' SQLite DB. It represents a task's start state, **not** which UI view it belongs to. The view is determined by `st` + `sr`/`tir` dates:

| `st` | Constant | + `sr`/`tir` | Things view |
|------|----------|-------------|-------------|
| 0 | `TaskScheduleInbox` | null | Inbox |
| 1 | `TaskScheduleAnytime` | today's date | Today |
| 1 | `TaskScheduleAnytime` | null | Anytime |
| 2 | `TaskScheduleSomeday` | future date | Upcoming |
| 2 | `TaskScheduleSomeday` | null | Someday |

Don't confuse `st` (schedule) with `ss` (status): `ss` is completion state — `0` = pending, `2` = canceled, `3` = completed (with `sp` set to the completion timestamp).

See `docs/client-side-bugs.md` for the full investigation.

## Environment Variables

The example app and real usage require:
- `THINGS_USERNAME` — Things account email
- `THINGS_PASSWORD` — Things account password
