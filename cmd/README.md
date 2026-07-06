# CLI Tools

All tools require environment variables:
```bash
export THINGS_USERNAME="your@email.com"
export THINGS_PASSWORD="yourpassword"
```

Or create a `.env` file and source it: `source .env`

## Production Tools

### things-cli

Full-featured CLI for CRUD operations on Things Cloud.

```bash
# Read operations (uses an incremental local state cache)
things-cli list [--today] [--inbox] [--area NAME] [--project NAME]
things-cli show <uuid>
things-cli areas
things-cli projects
things-cli tags

# Write operations (fast - no state loading)
things-cli create "Task title" [options]
things-cli edit <uuid> [--title ...] [--note ...] [--when ...]
things-cli complete <uuid>
things-cli trash <uuid>
things-cli purge <uuid>
things-cli move-to-today <uuid>

# Batch operations (all in one HTTP request - much faster!)
echo '[
  {"cmd": "create", "title": "Task 1"},
  {"cmd": "create", "title": "Task 2"},
  {"cmd": "complete", "uuid": "abc123"},
  {"cmd": "move-to-project", "uuid": "def456", "project": "proj-uuid"}
]' | things-cli batch

# Batch commands: create, complete, trash, purge, move-to-today,
#                 move-to-project, move-to-area, edit

# Optional read-state cache location:
#   THINGS_CLI_CACHE=/path/to/things-cli-state.json

# Create options:
#   --note "text"           Add a note
#   --when today|anytime|someday|inbox
#   --deadline YYYY-MM-DD
#   --scheduled YYYY-MM-DD
#   --project UUID          Add to project
#   --heading UUID          Add under heading
#   --area UUID             Add to area
#   --tags UUID,UUID,...    Add tags
#   --type task|project|heading
#   --checklist "Item 1,Item 2,..."
```

### thingsync

JSON-based sync with workflow views. Persists state to `~/.things-workflow/sync.db`.

```bash
# Default: full sync with JSON output
thingsync

# Human-readable output
thingsync --human

# Workflow views (JSON output)
thingsync --today      # Morning review: today's tasks + alerts
thingsync --inbox      # Triage view: inbox items with staleness
thingsync --review     # Evening review: completed vs remaining
thingsync --patterns   # Behavioral analysis: reschedule patterns

# Custom database location
thingsync --db /path/to/sync.db
```

Output includes:
- Sync metadata (index before/after, change count)
- Rich changes with context (project, area, heading, tags)
- Daily summary (completed, created, moved)
- Alerts (stale inbox, reschedule patterns, deadlines)

### synctest

Human-readable sync output for testing. Persists to temp directory.

```bash
synctest
```

Shows:
- New changes with titles
- Today's activity summary
- Inbox status with item ages
- Today view with reschedule warnings
- Accountability check for problematic tasks

---

## Debug & Development Tools

These tools are for SDK development and debugging. Most have hardcoded UUIDs for specific investigations.

### debug

Count items by kind (Task6, Area3, Tag4, etc.)

```bash
debug
# Output:
# Item kinds:
#   Task6: 1234
#   Area3: 12
#   Tag4: 45
```

### recent

Show the last ~100 items from history.

```bash
recent
```

### trace

Trace all changes for a specific UUID through history. Has hardcoded target UUID.

```bash
trace
# Shows all item versions for target UUID with full payload
```

### list

Simple task listing using state/memory.

```bash
list
# Output: all tasks with trash/status info
```

### fullstate

Dump complete state: areas, tags, all tasks.

```bash
fullstate
```

### statedebug

Debug state aggregation - shows Task6 item counts vs final state.

```bash
statedebug
```

### findtask

Find a specific task and trace its state changes. Has hardcoded target UUID.

```bash
findtask
```

### rawitem

Show raw item JSON for a specific UUID. Has hardcoded target UUID.

```bash
rawitem
```

### rawtask

Show first 20 Task6 items with titles.

```bash
rawtask
```

### debugupdate

Debug state.Update() behavior for a specific task. Has hardcoded target UUID.

```bash
debugupdate
```

---

## When to Use Which Tool

| Use Case | Tool |
|----------|------|
| Create/edit/complete tasks | `things-cli` |
| Automated workflows, JSON output | `thingsync` |
| Quick human-readable sync test | `synctest` |
| Debug item kinds in history | `debug` |
| See recent activity | `recent` |
| Investigate specific item history | `trace` |
| Debug state aggregation | `statedebug`, `findtask` |

## Building

```bash
# Build all tools
go build -v ./cmd/...

# Build specific tool
go build -v ./cmd/things-cli

# Run without building
go run ./cmd/synctest
```
