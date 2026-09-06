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
  {"cmd": "complete", "uuid": "BXmAcvS6yK1eDhW31MuZrL"},
  {"cmd": "move-to-project", "uuid": "VJ1edXTP9q3PmFDUuy8EQh", "project": "FQxaqvLBkbR5q2Q5oRoknc"}
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

#### Write validation

Replace the sample UUIDs above with IDs returned by `create` or `list` for your account.

Invalid identifiers, schedule names, task types, and dates cause a nonzero exit before any commit is sent. Identifiers must be canonical Base58 exactly as supplied, including `create-area --tags`, `create-tag --parent`, and purge targets. Do not include spaces around comma-separated tag IDs.

`--when` accepts `today`, `anytime`, `someday`, or `inbox`; `--type` accepts `task`, `project`, or `heading`. Dates must be real calendar dates in `YYYY-MM-DD` format. Invalid or missing option values are rejected instead of silently producing default or partial writes.

A batch is submitted only after every operation passes validation. Its input must contain one JSON array, with no unknown top-level operation fields or trailing input. Empty optional top-level batch strings retain their existing meaning of "not supplied." Explicitly empty identifier options in individual commands or `extra` are rejected; omit an optional identifier instead.

Batch `create` also accepts an `extra` object containing `note`, `when`, `deadline`, `scheduled`, `project`, `area`, `heading`, `tags`, or `type`. These values override the corresponding create options and are validated after merging. `extra.tags` is comma-separated text; top-level `tags` is a JSON array. Other operations reject an `extra` object, including `{}`; `extra: null` is treated as absent. For example:

```json
[{"cmd":"create","title":"Plan launch","extra":{"scheduled":"2026-10-15"}}]
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
