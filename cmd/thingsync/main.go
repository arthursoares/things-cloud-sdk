package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	"github.com/arthursoares/things-cloud-sdk/sync"
	"github.com/arthursoares/things-cloud-sdk/syncutil"
)

// Output structures for JSON

type SyncInfo struct {
	LastIndex    int       `json:"lastIndex"`
	NewIndex     int       `json:"newIndex"`
	ChangesCount int       `json:"changesCount"`
	SyncedAt     time.Time `json:"syncedAt"`
}

type EntityRef struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

type TaskContext struct {
	Heading *EntityRef `json:"heading,omitempty"`
	Project *EntityRef `json:"project,omitempty"`
	Area    *EntityRef `json:"area,omitempty"`
}

type LocationInfo struct {
	Location string     `json:"location"` // inbox, today, anytime, upcoming, someday, project
	Project  *EntityRef `json:"project,omitempty"`
	Area     *EntityRef `json:"area,omitempty"`
	Heading  *EntityRef `json:"heading,omitempty"`
	Date     string     `json:"date,omitempty"` // for upcoming
}

// RichChange is the enhanced change output with full context
type RichChange struct {
	Type        string        `json:"type"`
	UUID        string        `json:"uuid"`
	Title       string        `json:"title,omitempty"`
	Context     *TaskContext  `json:"context,omitempty"`
	From        *LocationInfo `json:"from,omitempty"`
	To          *LocationInfo `json:"to,omitempty"`
	Where       string        `json:"where,omitempty"` // for creates: inbox, today, etc.
	Tags        []EntityRef   `json:"tags,omitempty"`
	Date        string        `json:"date,omitempty"` // for scheduled
	CompletedAt *time.Time    `json:"completedAt,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
}

type TagInfo struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

type ProjectInfo struct {
	UUID  string `json:"uuid,omitempty"`
	Title string `json:"title,omitempty"`
}

type AreaInfo struct {
	UUID  string `json:"uuid,omitempty"`
	Title string `json:"title,omitempty"`
}

type TaskInfo struct {
	UUID          string       `json:"uuid"`
	Title         string       `json:"title"`
	Note          string       `json:"note,omitempty"`
	Tags          []TagInfo    `json:"tags,omitempty"`
	Project       *ProjectInfo `json:"project,omitempty"`
	Area          *AreaInfo    `json:"area,omitempty"`
	Heading       *EntityRef   `json:"heading,omitempty"`
	When          string       `json:"when"` // inbox, today, anytime, someday, upcoming
	AgeDays       int          `json:"ageDays,omitempty"`
	MoveCount     int          `json:"moveCount,omitempty"`
	Deadline      string       `json:"deadline,omitempty"`
	ScheduledDate string       `json:"scheduledDate,omitempty"`
	CreatedAt     time.Time    `json:"createdAt,omitempty"`
}

type Alert struct {
	Type   string `json:"type"` // stale, inbox_overflow, reschedule_pattern
	UUID   string `json:"uuid,omitempty"`
	Title  string `json:"title,omitempty"`
	Reason string `json:"reason"`
}

type StateOutput struct {
	Inbox  []TaskInfo `json:"inbox"`
	Today  []TaskInfo `json:"today"`
	Alerts []Alert    `json:"alerts"`
}

type Output struct {
	Sync    SyncInfo              `json:"sync"`
	Changes []RichChange          `json:"changes"`
	Summary syncutil.DailySummary `json:"dailySummary"`
	State   StateOutput           `json:"state"`
}

func main() {
	// Flags
	dbPath := flag.String("db", "", "Path to SQLite database (default: ~/.things-workflow/sync.db)")
	humanReadable := flag.Bool("human", false, "Human-readable output instead of JSON")

	// Workflow commands
	cmdToday := flag.Bool("today", false, "Show today view for morning review")
	cmdInbox := flag.Bool("inbox", false, "Show inbox for triage")
	cmdReview := flag.Bool("review", false, "Show evening review")
	cmdPatterns := flag.Bool("patterns", false, "Show behavioral patterns")

	flag.Parse()

	// Credentials from env
	username := os.Getenv("THINGS_USERNAME")
	password := os.Getenv("THINGS_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("THINGS_USERNAME and THINGS_PASSWORD must be set")
	}

	// Database path
	if *dbPath == "" {
		home, _ := os.UserHomeDir()
		*dbPath = filepath.Join(home, ".things-workflow", "sync.db")
	}
	os.MkdirAll(filepath.Dir(*dbPath), 0755)

	// Create client and syncer
	client := things.New(things.APIEndpoint, username, password)
	syncer, err := sync.Open(*dbPath, client)
	if err != nil {
		log.Fatalf("Failed to open syncer: %v", err)
	}
	defer syncer.Close()

	lastIndex := syncer.LastSyncedIndex()

	// Sync
	changes, err := syncer.Sync()
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	newIndex := syncer.LastSyncedIndex()

	// Handle workflow commands
	if *cmdToday {
		printTodayView(syncer)
		return
	}
	if *cmdInbox {
		printInboxView(syncer)
		return
	}
	if *cmdReview {
		printReviewView(syncer)
		return
	}
	if *cmdPatterns {
		printPatternsView(syncer)
		return
	}

	// Build output
	output := Output{
		Sync: SyncInfo{
			LastIndex:    lastIndex,
			NewIndex:     newIndex,
			ChangesCount: len(changes),
			SyncedAt:     time.Now(),
		},
		Changes: buildRichChanges(changes, syncer),
		Summary: syncutil.BuildDailySummary(syncer),
		State:   buildState(syncer),
	}

	if *humanReadable {
		printHuman(output)
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
	}
}

// resolveEntityRef looks up an entity by UUID and returns a reference with title
func resolveEntityRef(uuid string, syncer *sync.Syncer) *EntityRef {
	if uuid == "" {
		return nil
	}
	state := syncer.State()

	// Try task (could be project or heading)
	if task, err := state.Task(uuid); err == nil && task != nil {
		return &EntityRef{UUID: uuid, Title: task.Title}
	}
	// Try area
	if area, err := state.Area(uuid); err == nil && area != nil {
		return &EntityRef{UUID: uuid, Title: area.Title}
	}
	// Try tag
	if tag, err := state.Tag(uuid); err == nil && tag != nil {
		return &EntityRef{UUID: uuid, Title: tag.Title}
	}

	return &EntityRef{UUID: uuid, Title: "(unknown)"}
}

// getTaskContext builds the full context for a task
func getTaskContext(task *things.Task, syncer *sync.Syncer) *TaskContext {
	if task == nil {
		return nil
	}

	ctx := &TaskContext{}
	state := syncer.State()

	// Heading (action group)
	if len(task.ActionGroupIDs) > 0 {
		ctx.Heading = resolveEntityRef(task.ActionGroupIDs[0], syncer)
	}

	// Project (parent task that is a project)
	if len(task.ParentTaskIDs) > 0 {
		for _, parentID := range task.ParentTaskIDs {
			if parent, err := state.Task(parentID); err == nil && parent != nil {
				if parent.Type == things.TaskTypeProject {
					ctx.Project = &EntityRef{UUID: parent.UUID, Title: parent.Title}
					break
				}
			}
		}
	}

	// Area
	if len(task.AreaIDs) > 0 {
		if area, err := state.Area(task.AreaIDs[0]); err == nil && area != nil {
			ctx.Area = &EntityRef{UUID: area.UUID, Title: area.Title}
		}
	}

	// Check if context is empty
	if ctx.Heading == nil && ctx.Project == nil && ctx.Area == nil {
		return nil
	}

	return ctx
}

// getTaskLocation determines where a task is (inbox, today, etc.)
func getTaskLocation(task *things.Task, syncer *sync.Syncer) string {
	if task == nil {
		return "unknown"
	}

	// Check trash first
	if task.InTrash {
		return "trash"
	}

	// Check status
	if task.Status == things.TaskStatusCompleted {
		return "completed"
	}

	// Based on schedule/start state
	switch task.Schedule {
	case things.TaskScheduleInbox:
		return "inbox"
	case things.TaskScheduleAnytime:
		// Check if it has a today index (actually in today vs anytime)
		if task.TodayIndex != 0 {
			return "today"
		}
		return "anytime"
	case things.TaskScheduleSomeday:
		// Check if scheduled (upcoming) vs someday
		if task.ScheduledDate != nil && !task.ScheduledDate.IsZero() {
			return "upcoming"
		}
		return "someday"
	}

	return "unknown"
}

// buildRichChanges converts sync changes to rich format with context
func buildRichChanges(changes []sync.Change, syncer *sync.Syncer) []RichChange {
	var result []RichChange

	for _, c := range changes {
		rich := RichChange{
			UUID:      c.EntityUUID(),
			Timestamp: c.Timestamp(),
		}

		switch v := c.(type) {
		case sync.TaskCreated:
			rich.Type = "task_created"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Where = getTaskLocation(v.Task, syncer)
				rich.Context = getTaskContext(v.Task, syncer)
				rich.Tags = getTaskTags(v.Task, syncer)
			}

		case sync.TaskCompleted:
			rich.Type = "task_completed"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				if v.Task.CompletionDate != nil {
					rich.CompletedAt = v.Task.CompletionDate
				}
			}

		case sync.TaskTitleChanged:
			rich.Type = "task_updated"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
			}

		case sync.TaskMovedToToday:
			rich.Type = "task_today"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				rich.To = &LocationInfo{Location: "today"}
			}

		case sync.TaskMovedToInbox:
			rich.Type = "task_moved"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.To = &LocationInfo{Location: "inbox"}
			}

		case sync.TaskMovedToAnytime:
			rich.Type = "task_moved"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				rich.To = &LocationInfo{Location: "anytime"}
			}

		case sync.TaskMovedToSomeday:
			rich.Type = "task_deferred"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				rich.To = &LocationInfo{Location: "someday"}
			}

		case sync.TaskMovedToUpcoming:
			rich.Type = "task_scheduled"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				rich.To = &LocationInfo{Location: "upcoming"}
				if v.Task.ScheduledDate != nil {
					rich.Date = v.Task.ScheduledDate.Format("2006-01-02")
					rich.To.Date = rich.Date
				}
			}

		case sync.TaskTrashed:
			rich.Type = "task_trashed"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
			}

		case sync.TaskTagsChanged:
			rich.Type = "task_tagged"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Tags = getTaskTags(v.Task, syncer)
			}

		case sync.TaskAssignedToProject:
			rich.Type = "task_moved"
			if v.Task != nil {
				rich.Title = v.Task.Title
				rich.Context = getTaskContext(v.Task, syncer)
				if rich.Context != nil && rich.Context.Project != nil {
					rich.To = &LocationInfo{
						Location: "project",
						Project:  rich.Context.Project,
					}
				}
			}

		case sync.ProjectCreated:
			rich.Type = "project_created"
			if v.Project != nil {
				rich.Title = v.Project.Title
			}

		case sync.ProjectCompleted:
			rich.Type = "project_completed"
			if v.Project != nil {
				rich.Title = v.Project.Title
			}

		case sync.HeadingCreated:
			rich.Type = "heading_created"
			if v.Heading != nil {
				rich.Title = v.Heading.Title
				// Try to get parent project
				if len(v.Heading.ParentTaskIDs) > 0 {
					rich.Context = &TaskContext{
						Project: resolveEntityRef(v.Heading.ParentTaskIDs[0], syncer),
					}
				}
			}

		case sync.HeadingTitleChanged:
			rich.Type = "heading_updated"
			if v.Heading != nil {
				rich.Title = v.Heading.Title
				if len(v.Heading.ParentTaskIDs) > 0 {
					rich.Context = &TaskContext{
						Project: resolveEntityRef(v.Heading.ParentTaskIDs[0], syncer),
					}
				}
			}

		case sync.AreaCreated:
			rich.Type = "area_created"
			if v.Area != nil {
				rich.Title = v.Area.Title
			}

		case sync.TagCreated:
			rich.Type = "tag_created"
			if v.Tag != nil {
				rich.Title = v.Tag.Title
			}

		default:
			// Generic fallback
			rich.Type = c.ChangeType()
		}

		result = append(result, rich)
	}

	return result
}

// getTaskTags returns tag references for a task
func getTaskTags(task *things.Task, syncer *sync.Syncer) []EntityRef {
	if task == nil || len(task.TagIDs) == 0 {
		return nil
	}

	var tags []EntityRef
	state := syncer.State()

	for _, tagID := range task.TagIDs {
		if tag, err := state.Tag(tagID); err == nil && tag != nil {
			tags = append(tags, EntityRef{UUID: tag.UUID, Title: tag.Title})
		}
	}

	return tags
}

func buildState(syncer *sync.Syncer) StateOutput {
	state := syncer.State()
	output := StateOutput{
		Inbox:  []TaskInfo{},
		Today:  []TaskInfo{},
		Alerts: []Alert{},
	}

	// Inbox
	inbox, _ := state.TasksInInbox(sync.QueryOpts{})
	for _, t := range inbox {
		info := taskToInfo(t, syncer, "inbox")
		output.Inbox = append(output.Inbox, info)

		// Check for alerts
		if info.AgeDays >= 7 {
			output.Alerts = append(output.Alerts, Alert{
				Type:   "stale",
				UUID:   t.UUID,
				Title:  t.Title,
				Reason: fmt.Sprintf("in inbox for %d days", info.AgeDays),
			})
		}
	}

	// Inbox overflow alert
	if len(inbox) > 10 {
		output.Alerts = append(output.Alerts, Alert{
			Type:   "inbox_overflow",
			Reason: fmt.Sprintf("inbox has %d items", len(inbox)),
		})
	}

	// Today
	today, _ := state.TasksInToday(sync.QueryOpts{})
	for _, t := range today {
		info := taskToInfo(t, syncer, "today")
		output.Today = append(output.Today, info)

		// Check for reschedule patterns
		if info.MoveCount >= 3 {
			output.Alerts = append(output.Alerts, Alert{
				Type:   "reschedule_pattern",
				UUID:   t.UUID,
				Title:  t.Title,
				Reason: fmt.Sprintf("rescheduled %d times", info.MoveCount),
			})
		}
	}

	return output
}

func taskToInfo(t *things.Task, syncer *sync.Syncer, when string) TaskInfo {
	info := TaskInfo{
		UUID:  t.UUID,
		Title: t.Title,
		Note:  t.Note,
		When:  when,
	}

	state := syncer.State()

	// Age
	if !t.CreationDate.IsZero() {
		info.AgeDays = int(time.Since(t.CreationDate).Hours() / 24)
		info.CreatedAt = t.CreationDate
	}

	// Move count from history
	changes, _ := syncer.ChangesForEntity(t.UUID)
	for _, c := range changes {
		if strings.HasPrefix(c.ChangeType(), "TaskMovedTo") {
			info.MoveCount++
		}
	}

	// Deadline
	if t.DeadlineDate != nil && !t.DeadlineDate.IsZero() {
		info.Deadline = t.DeadlineDate.Format("2006-01-02")
	}

	// Scheduled date
	if t.ScheduledDate != nil && !t.ScheduledDate.IsZero() {
		info.ScheduledDate = t.ScheduledDate.Format("2006-01-02")
	}

	// Tags
	for _, tagID := range t.TagIDs {
		if tag, err := state.Tag(tagID); err == nil && tag != nil {
			info.Tags = append(info.Tags, TagInfo{
				UUID:  tag.UUID,
				Title: tag.Title,
			})
		}
	}

	// Heading
	if len(t.ActionGroupIDs) > 0 {
		if heading, err := state.Task(t.ActionGroupIDs[0]); err == nil && heading != nil {
			info.Heading = &EntityRef{UUID: heading.UUID, Title: heading.Title}
		}
	}

	// Area
	if len(t.AreaIDs) > 0 {
		if area, err := state.Area(t.AreaIDs[0]); err == nil && area != nil {
			info.Area = &AreaInfo{
				UUID:  area.UUID,
				Title: area.Title,
			}
		}
	}

	// Project
	if len(t.ParentTaskIDs) > 0 {
		for _, parentID := range t.ParentTaskIDs {
			if parent, err := state.Task(parentID); err == nil && parent != nil {
				if parent.Type == things.TaskTypeProject {
					info.Project = &ProjectInfo{
						UUID:  parent.UUID,
						Title: parent.Title,
					}
					break
				}
			}
		}
	}

	return info
}

func printTaskHuman(t TaskInfo) {
	// Build tag prefix
	var tagStr string
	if len(t.Tags) > 0 {
		var tagNames []string
		for _, tag := range t.Tags {
			tagNames = append(tagNames, tag.Title)
		}
		tagStr = "[" + strings.Join(tagNames, ", ") + "] "
	}

	// Build context suffix
	var ctx []string
	if t.AgeDays > 1 {
		ctx = append(ctx, fmt.Sprintf("%dd old", t.AgeDays))
	}
	if t.MoveCount > 0 {
		ctx = append(ctx, fmt.Sprintf("moved %dx", t.MoveCount))
	}
	if t.Heading != nil {
		ctx = append(ctx, fmt.Sprintf("under '%s'", t.Heading.Title))
	}
	if t.Project != nil {
		ctx = append(ctx, fmt.Sprintf("in '%s'", t.Project.Title))
	}
	if t.Deadline != "" {
		ctx = append(ctx, fmt.Sprintf("due %s", t.Deadline))
	}

	// Print
	if len(ctx) > 0 {
		fmt.Printf("  - %s%s (%s)\n", tagStr, t.Title, strings.Join(ctx, ", "))
	} else {
		fmt.Printf("  - %s%s\n", tagStr, t.Title)
	}
}

func printChangeHuman(c RichChange) {
	var desc string

	switch c.Type {
	case "task_created":
		desc = fmt.Sprintf("✚ Created: %s", c.Title)
		if c.Where != "" && c.Where != "unknown" {
			desc += fmt.Sprintf(" → %s", c.Where)
		}
	case "task_completed":
		desc = fmt.Sprintf("✓ Completed: %s", c.Title)
	case "task_today":
		desc = fmt.Sprintf("→ Today: %s", c.Title)
	case "task_deferred":
		desc = fmt.Sprintf("⏸ Deferred: %s", c.Title)
	case "task_scheduled":
		desc = fmt.Sprintf("📅 Scheduled: %s", c.Title)
		if c.Date != "" {
			desc += fmt.Sprintf(" for %s", c.Date)
		}
	case "task_moved":
		desc = fmt.Sprintf("↔ Moved: %s", c.Title)
		if c.To != nil {
			desc += fmt.Sprintf(" → %s", c.To.Location)
			if c.To.Project != nil {
				desc += fmt.Sprintf(" (%s)", c.To.Project.Title)
			}
		}
	case "task_trashed":
		desc = fmt.Sprintf("🗑 Trashed: %s", c.Title)
	case "task_tagged":
		desc = fmt.Sprintf("🏷 Tagged: %s", c.Title)
	case "task_updated":
		desc = fmt.Sprintf("✎ Updated: %s", c.Title)
	case "project_created":
		desc = fmt.Sprintf("📁 Project created: %s", c.Title)
	case "project_completed":
		desc = fmt.Sprintf("📁✓ Project completed: %s", c.Title)
	case "heading_created":
		desc = fmt.Sprintf("§ Heading created: %s", c.Title)
		if c.Context != nil && c.Context.Project != nil {
			desc += fmt.Sprintf(" in '%s'", c.Context.Project.Title)
		}
	case "area_created":
		desc = fmt.Sprintf("◉ Area created: %s", c.Title)
	case "tag_created":
		desc = fmt.Sprintf("🏷 Tag created: %s", c.Title)
	default:
		desc = fmt.Sprintf("%s: %s", c.Type, c.Title)
	}

	// Add context if available
	if c.Context != nil && c.Type != "heading_created" {
		var ctxParts []string
		if c.Context.Heading != nil {
			ctxParts = append(ctxParts, fmt.Sprintf("under '%s'", c.Context.Heading.Title))
		}
		if c.Context.Project != nil {
			ctxParts = append(ctxParts, fmt.Sprintf("in '%s'", c.Context.Project.Title))
		}
		if len(ctxParts) > 0 {
			desc += fmt.Sprintf(" (%s)", strings.Join(ctxParts, ", "))
		}
	}

	fmt.Printf("  %s\n", desc)
}

func printHuman(output Output) {
	fmt.Printf("Synced: %d → %d (%d changes)\n",
		output.Sync.LastIndex, output.Sync.NewIndex, output.Sync.ChangesCount)

	// Show changes if any
	if len(output.Changes) > 0 {
		fmt.Println("\n--- Changes ---")
		for _, c := range output.Changes {
			printChangeHuman(c)
		}
	}

	if output.Summary.Completed > 0 || output.Summary.Created > 0 || output.Summary.MovedToToday > 0 {
		fmt.Println("\n--- Today's Activity ---")
		fmt.Printf("  ✓ Completed: %d\n", output.Summary.Completed)
		fmt.Printf("  + Created: %d\n", output.Summary.Created)
		fmt.Printf("  → Moved to Today: %d\n", output.Summary.MovedToToday)
		fmt.Printf("  ↔ Rescheduled: %d\n", output.Summary.Rescheduled)
	}

	fmt.Printf("\n--- Inbox (%d) ---\n", len(output.State.Inbox))
	for _, t := range output.State.Inbox {
		printTaskHuman(t)
	}

	fmt.Printf("\n--- Today (%d) ---\n", len(output.State.Today))
	for _, t := range output.State.Today {
		printTaskHuman(t)
	}

	if len(output.State.Alerts) > 0 {
		fmt.Println("\n--- Alerts ---")
		for _, a := range output.State.Alerts {
			fmt.Printf("  ⚠️  %s: %s\n", a.Type, a.Reason)
		}
	}
}

// ============================================
// Workflow Views
// ============================================

type TodayView struct {
	Tasks      []TaskInfo `json:"tasks"`
	Overloaded bool       `json:"overloaded"`
	Alerts     []Alert    `json:"alerts"`
	Summary    string     `json:"summary"`
}

type InboxView struct {
	Items      []TaskInfo `json:"items"`
	Count      int        `json:"count"`
	OldestDays int        `json:"oldestDays"`
	Alerts     []Alert    `json:"alerts"`
}

type ReviewView struct {
	CompletedToday []TaskInfo            `json:"completedToday"`
	StillInToday   []TaskInfo            `json:"stillInToday"`
	MovedCount     int                   `json:"movedCount"`
	Summary        syncutil.DailySummary `json:"summary"`
}

type PatternView struct {
	Rescheduled    []TaskInfo `json:"rescheduled"`    // Tasks moved 3+ times
	StaleInbox     []TaskInfo `json:"staleInbox"`     // Inbox items >7 days
	NeglectedAreas []string   `json:"neglectedAreas"` // Areas with no activity
}

func printTodayView(syncer *sync.Syncer) {
	state := syncer.State()
	today, _ := state.TasksInToday(sync.QueryOpts{})

	view := TodayView{
		Tasks:      []TaskInfo{},
		Overloaded: len(today) > 5,
		Alerts:     []Alert{},
	}

	for _, t := range today {
		info := taskToInfo(t, syncer, "today")
		view.Tasks = append(view.Tasks, info)

		// Check for reschedule patterns
		if info.MoveCount >= 3 {
			view.Alerts = append(view.Alerts, Alert{
				Type:   "reschedule_pattern",
				UUID:   t.UUID,
				Title:  t.Title,
				Reason: fmt.Sprintf("rescheduled %d times - what's blocking this?", info.MoveCount),
			})
		}

		// Check for deadlines
		if info.Deadline != "" {
			deadline, _ := time.Parse("2006-01-02", info.Deadline)
			if deadline.Before(time.Now().Add(24 * time.Hour)) {
				view.Alerts = append(view.Alerts, Alert{
					Type:   "deadline",
					UUID:   t.UUID,
					Title:  t.Title,
					Reason: fmt.Sprintf("due %s", info.Deadline),
				})
			}
		}
	}

	if view.Overloaded {
		view.Alerts = append(view.Alerts, Alert{
			Type:   "overloaded",
			Reason: fmt.Sprintf("Today has %d tasks - consider deferring some", len(today)),
		})
	}

	view.Summary = fmt.Sprintf("%d tasks for today", len(today))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(view)
}

func printInboxView(syncer *sync.Syncer) {
	state := syncer.State()
	inbox, _ := state.TasksInInbox(sync.QueryOpts{})

	view := InboxView{
		Items:  []TaskInfo{},
		Count:  len(inbox),
		Alerts: []Alert{},
	}

	for _, t := range inbox {
		info := taskToInfo(t, syncer, "inbox")
		view.Items = append(view.Items, info)

		if info.AgeDays > view.OldestDays {
			view.OldestDays = info.AgeDays
		}

		if info.AgeDays >= 7 {
			view.Alerts = append(view.Alerts, Alert{
				Type:   "stale",
				UUID:   t.UUID,
				Title:  t.Title,
				Reason: fmt.Sprintf("sitting in inbox for %d days", info.AgeDays),
			})
		}
	}

	if len(inbox) > 10 {
		view.Alerts = append(view.Alerts, Alert{
			Type:   "inbox_overflow",
			Reason: fmt.Sprintf("inbox has %d items - needs triage", len(inbox)),
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(view)
}

func printReviewView(syncer *sync.Syncer) {
	state := syncer.State()
	todayStart := syncutil.StartOfToday()

	// Get completed tasks today
	changes, _ := syncer.ChangesSince(todayStart)
	completedUUIDs := make(map[string]bool)
	for _, c := range changes {
		if c.ChangeType() == "TaskCompleted" {
			completedUUIDs[c.EntityUUID()] = true
		}
	}

	view := ReviewView{
		CompletedToday: []TaskInfo{},
		StillInToday:   []TaskInfo{},
		Summary:        syncutil.BuildDailySummary(syncer),
	}

	// Get completed tasks
	for uuid := range completedUUIDs {
		if task, err := state.Task(uuid); err == nil && task != nil {
			info := taskToInfo(task, syncer, "completed")
			view.CompletedToday = append(view.CompletedToday, info)
		}
	}

	// Get remaining today tasks
	today, _ := state.TasksInToday(sync.QueryOpts{})
	for _, t := range today {
		info := taskToInfo(t, syncer, "today")
		view.StillInToday = append(view.StillInToday, info)
	}

	// Count tasks moved today
	for _, c := range changes {
		if strings.HasPrefix(c.ChangeType(), "TaskMovedTo") {
			view.MovedCount++
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(view)
}

func printPatternsView(syncer *sync.Syncer) {
	state := syncer.State()

	view := PatternView{
		Rescheduled:    []TaskInfo{},
		StaleInbox:     []TaskInfo{},
		NeglectedAreas: []string{},
	}

	// Find rescheduled tasks (anywhere, not just today)
	today, _ := state.TasksInToday(sync.QueryOpts{})
	for _, t := range today {
		info := taskToInfo(t, syncer, "today")
		if info.MoveCount >= 3 {
			view.Rescheduled = append(view.Rescheduled, info)
		}
	}

	// Also check all tasks for reschedule patterns
	allTasks, _ := state.AllTasks(sync.QueryOpts{})
	seenUUIDs := make(map[string]bool)
	for _, t := range today {
		seenUUIDs[t.UUID] = true
	}
	for _, t := range allTasks {
		if seenUUIDs[t.UUID] {
			continue // Already checked in today
		}
		info := taskToInfo(t, syncer, getTaskLocation(t, syncer))
		if info.MoveCount >= 3 {
			view.Rescheduled = append(view.Rescheduled, info)
		}
	}

	// Stale inbox items
	inbox, _ := state.TasksInInbox(sync.QueryOpts{})
	for _, t := range inbox {
		info := taskToInfo(t, syncer, "inbox")
		if info.AgeDays >= 7 {
			view.StaleInbox = append(view.StaleInbox, info)
		}
	}

	// TODO: Track area activity for neglected areas

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(view)
}
