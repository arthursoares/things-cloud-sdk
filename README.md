# things cloud sdk

[Things](https://culturedcode.com/things/) comes with a cloud based API, which can
be used to synchronize data between devices.
This is a golang SDK to interact with that API, opening the API so that you
can enhance your Things experience on iOS and Mac.

[![Go](https://github.com/arthursoares/things-cloud-sdk/actions/workflows/go.yml/badge.svg)](https://github.com/arthursoares/things-cloud-sdk/actions/workflows/go.yml)

## Getting Started

### Installation

```bash
go get github.com/arthursoares/things-cloud-sdk
```

### Quick Start

**1. Set up your credentials:**

```bash
export THINGS_USERNAME='your@email.com'
export THINGS_PASSWORD='yourpassword'
```

**2. Create a simple Go program:**

```go
package main

import (
    "fmt"
    "os"
    things "github.com/arthursoares/things-cloud-sdk"
)

func main() {
    client := things.New(
        things.APIEndpoint,
        os.Getenv("THINGS_USERNAME"),
        os.Getenv("THINGS_PASSWORD"),
    )

    // Verify credentials
    resp, err := client.Verify()
    if err != nil {
        panic(err)
    }
    fmt.Printf("✓ Connected: %s\n", resp.Email)

    // Get or create a history
    histories, _ := client.Histories()
    var historyID string
    if len(histories) > 0 {
        historyID = histories[0].ID
    } else {
        hist, _ := client.CreateHistory()
        historyID = hist.ID
    }

    // Create a task
    task := things.Task{
        UUID:  things.GenerateUUID(),
        Title: things.String("My first task from the SDK!"),
    }

    items := []things.Item{things.NewCreateTaskItem(task)}
    client.Write(historyID, items, -1)
    fmt.Printf("✓ Created task: %s\n", *task.Title)
}
```

**3. Run it:**

```bash
go run main.go
```

### CLI Quick Start

Install and use the command-line tool:

```bash
# Install
go install github.com/arthursoares/things-cloud-sdk/cmd/things-cli@latest

# Create a task
things-cli create "Buy groceries" --when today

# List today's tasks
things-cli list --today

# Complete a task
things-cli complete <task-uuid>
```

## Features

- **Verify Credentials** — validate account access
- **Account Management** — signup, confirmation, password change, deletion
- **History Management** — list, create, delete, sync histories
- **Item Read/Write** — full event-sourced CRUD for tasks, areas, tags, checklist items (supports batching multiple items in one request)
- **Task Types** — tasks, projects, and headings (action groups within projects)
- **Structured Notes** — full-text and delta patch support for task notes
- **Recurring Tasks** — neverending, end on date, end after N times
- **Tombstone Deletion** — explicit deletion records via `Tombstone2` entities
- **Device Registration** — register app instances for APNS push notifications
- **Alarm/Reminders** — alarm time offset support on tasks
- **State Aggregation** — in-memory state built from history items, with queries for projects, headings, subtasks, areas, tags, and checklist items
- **Persistent Sync Engine** — SQLite-backed incremental sync with semantic change detection

## CLI

`things-cli` is a command-line tool for interacting with Things Cloud directly.

### Setup

```bash
export THINGS_USERNAME='your@email.com'
export THINGS_PASSWORD='yourpassword'
go build -o things-cli ./cmd/things-cli/
```

### Commands

```bash
# Read
things-cli list [--today] [--inbox] [--anytime] [--someday] [--upcoming] [--search QUERY] [--area NAME] [--project NAME]
things-cli today
things-cli inbox
things-cli anytime
things-cli someday
things-cli upcoming
things-cli search <query>
things-cli show <uuid>
things-cli areas
things-cli projects
things-cli tags

# Optional read-state cache location
export THINGS_CLI_CACHE=/path/to/things-cli-state.json

# Create
things-cli create "Title" [--note ...] [--when today|anytime|someday|inbox] \
  [--deadline YYYY-MM-DD] [--scheduled YYYY-MM-DD] \
  [--project UUID] [--heading UUID] [--area UUID] \
  [--tags UUID,...] [--type task|project|heading]
things-cli create-area "Name"
things-cli create-tag "Name" [--shorthand KEY] [--parent UUID]

# Modify
things-cli edit <uuid> [--title ...] [--note ...] [--when ...] [--deadline ...]
things-cli complete <uuid>
things-cli trash <uuid>
things-cli purge <uuid>
things-cli move-to-today <uuid>

# Batch (all operations in one HTTP request - much faster!)
echo '[{"cmd":"complete","uuid":"abc"},{"cmd":"trash","uuid":"def"}]' | things-cli batch
```

### Examples

```bash
# Create a project with tasks
things-cli create "My Project" --type project --when anytime
# → {"status":"created","uuid":"BXmAcvS6yK1eDhW31MuZrL","title":"My Project"}

things-cli create "First Task" --project BXmAcvS6yK1eDhW31MuZrL --when today --note "Details here"

# Create an area and assign tasks
things-cli create-area "Work"
things-cli create "Review PR" --area <area-uuid> --when today --deadline 2026-02-15

# Batch operations (50 ops in ~2 sec instead of ~2-3 min)
echo '[
  {"cmd": "create", "title": "Task 1"},
  {"cmd": "create", "title": "Task 2"},
  {"cmd": "move-to-project", "uuid": "abc123", "project": "proj-uuid"},
  {"cmd": "complete", "uuid": "def456"}
]' | things-cli batch
```

## Advanced SDK Usage

### Working with Histories and Items

```go
package main

import (
    "fmt"
    "os"
    things "github.com/arthursoares/things-cloud-sdk"
)

func main() {
    client := things.New(
        things.APIEndpoint,
        os.Getenv("THINGS_USERNAME"),
        os.Getenv("THINGS_PASSWORD"),
    )

    // Create a history
    history, _ := client.CreateHistory()

    // Create a project with tasks
    project := things.Task{
        UUID:     things.GenerateUUID(),
        Title:    things.String("My Project"),
        TaskType: things.TaskTypePtr(things.TaskTypeProject),
        Status:   things.Status(things.TaskStatusPending),
        Schedule: things.Schedule(things.TaskScheduleAnytime),
    }

    task1 := things.Task{
        UUID:      things.GenerateUUID(),
        Title:     things.String("First task"),
        ProjectID: things.String(project.UUID),
        Schedule:  things.Schedule(things.TaskScheduleAnytime),
    }

    task2 := things.Task{
        UUID:      things.GenerateUUID(),
        Title:     things.String("Second task"),
        ProjectID: things.String(project.UUID),
        Schedule:  things.Schedule(things.TaskScheduleAnytime),
    }

    // Write all items in one batch
    items := []things.Item{
        things.NewCreateTaskItem(project),
        things.NewCreateTaskItem(task1),
        things.NewCreateTaskItem(task2),
    }

    client.Write(history.ID, items, -1)
    fmt.Println("✓ Created project with 2 tasks")
}
```

See the `example/` directory for more complete examples including history sync, task creation, and state aggregation.

## Persistent Sync Engine

The `sync` package provides a SQLite-backed sync engine that tracks "what changed since last sync" — perfect for building agents, automations, or dashboards that react to Things changes.

```go
package main

import (
    "fmt"
    "os"
    things "github.com/arthursoares/things-cloud-sdk"
    "github.com/arthursoares/things-cloud-sdk/sync"
)

func main() {
    client := things.New(
        things.APIEndpoint,
        os.Getenv("THINGS_USERNAME"),
        os.Getenv("THINGS_PASSWORD"),
    )

    // Open persistent sync database
    syncer, _ := sync.Open("things.db", client)
    defer syncer.Close()

    // Fetch changes since last sync
    changes, _ := syncer.Sync()

    for _, c := range changes {
        switch v := c.(type) {
        case sync.TaskCreated:
            fmt.Printf("New task: %s\n", v.Task.Title)
        case sync.TaskCompleted:
            fmt.Printf("Completed: %s\n", v.Task.Title)
        case sync.TaskMovedToToday:
            fmt.Printf("Moved to Today: %s\n", v.Task.Title)
        }
    }

    // Query current state
    state := syncer.State()
    inbox, _ := state.TasksInInbox(sync.QueryOpts{})
    projects, _ := state.AllProjects(sync.QueryOpts{})
}
```

### Semantic Change Types

The sync engine detects 40+ semantic change types:

| Category | Changes |
|----------|---------|
| **Task Lifecycle** | `TaskCreated`, `TaskCompleted`, `TaskUncompleted`, `TaskTrashed`, `TaskDeleted` |
| **Task Movement** | `TaskMovedToInbox`, `TaskMovedToToday`, `TaskMovedToAnytime`, `TaskMovedToSomeday`, `TaskMovedToUpcoming` |
| **Task Organization** | `TaskMovedToProject`, `TaskMovedToArea`, `TaskMovedUnderHeading`, `TaskTagsChanged` |
| **Task Details** | `TaskTitleChanged`, `TaskNoteChanged`, `TaskDeadlineSet`, `TaskDeadlineRemoved` |
| **Projects** | `ProjectCreated`, `ProjectCompleted`, `ProjectTrashed`, `ProjectDeleted` |
| **Areas & Tags** | `AreaCreated`, `AreaDeleted`, `TagCreated`, `TagDeleted` |
| **Checklists** | `ChecklistItemCreated`, `ChecklistItemCompleted`, `ChecklistItemDeleted` |

### State Queries

```go
state := syncer.State()

// Query by location
inbox, _ := state.TasksInInbox(sync.QueryOpts{})
today, _ := state.TasksInToday(sync.QueryOpts{})
anytime, _ := state.TasksInAnytime(sync.QueryOpts{})
someday, _ := state.TasksInSomeday(sync.QueryOpts{})
upcoming, _ := state.TasksInUpcoming(sync.QueryOpts{})

// Query by container
tasks, _ := state.TasksInProject(projectUUID, sync.QueryOpts{})
tasks, _ := state.TasksInArea(areaUUID, sync.QueryOpts{})
tasks, _ := state.TasksUnderHeading(headingUUID, sync.QueryOpts{})
headings, _ := state.HeadingsInProject(projectUUID, sync.QueryOpts{})

// Query by tag or text
tasks, _ := state.TasksWithTag(tagUUID, sync.QueryOpts{})
tasks, _ := state.SearchTasks("invoice", sync.QueryOpts{})

// List all
projects, _ := state.AllProjects(sync.QueryOpts{})
headings, _ := state.AllHeadings(sync.QueryOpts{})
areas, _ := state.AllAreas(sync.QueryOpts{})
tags, _ := state.AllTags(sync.QueryOpts{})
```

### Change Log Queries

```go
// Changes in last hour
changes, _ := syncer.ChangesSince(time.Now().Add(-1 * time.Hour))

// Changes for a specific task
changes, _ := syncer.ChangesForEntity(taskUUID)

// Changes since server index
changes, _ := syncer.ChangesSinceIndex(150)
```

## Wire Format Notes

Key findings from reverse engineering the Things Cloud sync protocol:

- **UUIDs must be Base58-encoded** (Bitcoin alphabet: `123456789ABCDEFGH...`). Standard UUID strings or other encodings will crash Things.app during sync.
- **`md` (modification date) must be `null` on creates.** Set timestamps only on updates.
- **Schedule field (`st`)**: `0` = Inbox, `1` = Anytime/Today (with `sr`/`tir` dates = Today), `2` = Someday/Upcoming (with dates = Upcoming).
- **Status field (`ss`)**: `0` = Pending, `2` = Canceled, `3` = Completed. Don't confuse with `st` (schedule)!
- **Headings (`tp=2`) must have `st=1`** (anytime). `st=0` (inbox) crashes Things.app.
- **Tasks in projects, headings, or areas** should default to `st=1` (anytime) — they've been triaged out of inbox.
- **Kind strings**: `Task6`, `Tag4`, `ChecklistItem3`, `Area3`, `Tombstone2`

See `docs/client-side-bugs.md` for the full investigation and crash analysis.

## Architecture

The SDK models all changes as immutable Items (events). A History is a sync stream identified by a UUID. The client pushes/pulls Items through Histories, inspired by [operational transformations and Git's internals](https://www.swift.org/blog/how-swifts-server-support-powers-things-cloud/).

## TODO

- [ ] Repeat after completion
- [x] Persistent state storage (see `sync` package)

## Note

As there is no official API documentation available all requests need to be reverse engineered,
which takes some time. Feel free to contribute and improve & extend this implementation.
