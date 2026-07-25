package sync

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// TestChangeGetters constructs every Change implementation and verifies its
// ChangeType/EntityType/EntityUUID getters. These are trivial accessors, so the
// value is guarding against copy-paste mistakes in the string literals.
func TestChangeGetters(t *testing.T) {
	t.Parallel()

	task := &things.Task{UUID: "task-1"}
	project := &things.Task{UUID: "proj-1", Type: things.TaskTypeProject}
	heading := &things.Task{UUID: "head-1", Type: things.TaskTypeHeading}
	area := &things.Area{UUID: "area-1"}
	tag := &things.Tag{UUID: "tag-1"}
	item := &things.CheckListItem{UUID: "cli-1"}

	tc := taskChange{Task: task}
	pc := projectChange{Project: project}
	hc := headingChange{Heading: heading}
	ac := areaChange{Area: area}
	tgc := tagChange{Tag: tag}
	cic := checklistItemChange{Item: item}

	cases := []struct {
		change     Change
		changeType string
		entityType string
		entityUUID string
	}{
		{TaskCreated{taskChange: tc}, "TaskCreated", "Task", "task-1"},
		{TaskDeleted{taskChange: tc}, "TaskDeleted", "Task", "task-1"},
		{TaskCompleted{taskChange: tc}, "TaskCompleted", "Task", "task-1"},
		{TaskUncompleted{taskChange: tc}, "TaskUncompleted", "Task", "task-1"},
		{TaskCanceled{taskChange: tc}, "TaskCanceled", "Task", "task-1"},
		{TaskTitleChanged{taskChange: tc}, "TaskTitleChanged", "Task", "task-1"},
		{TaskNoteChanged{taskChange: tc}, "TaskNoteChanged", "Task", "task-1"},
		{TaskMovedToInbox{taskChange: tc}, "TaskMovedToInbox", "Task", "task-1"},
		{TaskMovedToToday{taskChange: tc}, "TaskMovedToToday", "Task", "task-1"},
		{TaskMovedToAnytime{taskChange: tc}, "TaskMovedToAnytime", "Task", "task-1"},
		{TaskMovedToSomeday{taskChange: tc}, "TaskMovedToSomeday", "Task", "task-1"},
		{TaskMovedToUpcoming{taskChange: tc}, "TaskMovedToUpcoming", "Task", "task-1"},
		{TaskDeadlineChanged{taskChange: tc}, "TaskDeadlineChanged", "Task", "task-1"},
		{TaskAssignedToProject{taskChange: tc}, "TaskAssignedToProject", "Task", "task-1"},
		{TaskAssignedToArea{taskChange: tc}, "TaskAssignedToArea", "Task", "task-1"},
		{TaskTrashed{taskChange: tc}, "TaskTrashed", "Task", "task-1"},
		{TaskRestored{taskChange: tc}, "TaskRestored", "Task", "task-1"},
		{TaskTagsChanged{taskChange: tc}, "TaskTagsChanged", "Task", "task-1"},

		{ProjectCreated{projectChange: pc}, "ProjectCreated", "Project", "proj-1"},
		{ProjectDeleted{projectChange: pc}, "ProjectDeleted", "Project", "proj-1"},
		{ProjectCompleted{projectChange: pc}, "ProjectCompleted", "Project", "proj-1"},
		{ProjectTitleChanged{projectChange: pc}, "ProjectTitleChanged", "Project", "proj-1"},
		{ProjectTrashed{projectChange: pc}, "ProjectTrashed", "Project", "proj-1"},
		{ProjectRestored{projectChange: pc}, "ProjectRestored", "Project", "proj-1"},

		{HeadingCreated{headingChange: hc}, "HeadingCreated", "Heading", "head-1"},
		{HeadingDeleted{headingChange: hc}, "HeadingDeleted", "Heading", "head-1"},
		{HeadingTitleChanged{headingChange: hc}, "HeadingTitleChanged", "Heading", "head-1"},

		{AreaCreated{areaChange: ac}, "AreaCreated", "Area", "area-1"},
		{AreaDeleted{areaChange: ac}, "AreaDeleted", "Area", "area-1"},
		{AreaRenamed{areaChange: ac}, "AreaRenamed", "Area", "area-1"},

		{TagCreated{tagChange: tgc}, "TagCreated", "Tag", "tag-1"},
		{TagDeleted{tagChange: tgc}, "TagDeleted", "Tag", "tag-1"},
		{TagRenamed{tagChange: tgc}, "TagRenamed", "Tag", "tag-1"},
		{TagShortcutChanged{tagChange: tgc}, "TagShortcutChanged", "Tag", "tag-1"},

		{ChecklistItemCreated{checklistItemChange: cic}, "ChecklistItemCreated", "ChecklistItem", "cli-1"},
		{ChecklistItemDeleted{checklistItemChange: cic}, "ChecklistItemDeleted", "ChecklistItem", "cli-1"},
		{ChecklistItemCompleted{checklistItemChange: cic}, "ChecklistItemCompleted", "ChecklistItem", "cli-1"},
		{ChecklistItemUncompleted{checklistItemChange: cic}, "ChecklistItemUncompleted", "ChecklistItem", "cli-1"},
		{ChecklistItemTitleChanged{checklistItemChange: cic}, "ChecklistItemTitleChanged", "ChecklistItem", "cli-1"},

		{UnknownChange{entityType: "Weird", entityUUID: "u-1"}, "UnknownChange", "Weird", "u-1"},
	}

	for _, c := range cases {
		t.Run(c.changeType, func(t *testing.T) {
			t.Parallel()
			if got := c.change.ChangeType(); got != c.changeType {
				t.Errorf("ChangeType() = %q, want %q", got, c.changeType)
			}
			if got := c.change.EntityType(); got != c.entityType {
				t.Errorf("EntityType() = %q, want %q", got, c.entityType)
			}
			if got := c.change.EntityUUID(); got != c.entityUUID {
				t.Errorf("EntityUUID() = %q, want %q", got, c.entityUUID)
			}
		})
	}
}

// TestChangeEntityUUIDNilEntity checks the nil-guard branches in the embedded
// EntityUUID accessors.
func TestChangeEntityUUIDNilEntity(t *testing.T) {
	t.Parallel()

	if got := (TaskCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil task EntityUUID() = %q, want empty", got)
	}
	if got := (ProjectCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil project EntityUUID() = %q, want empty", got)
	}
	if got := (HeadingCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil heading EntityUUID() = %q, want empty", got)
	}
	if got := (AreaCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil area EntityUUID() = %q, want empty", got)
	}
	if got := (TagCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil tag EntityUUID() = %q, want empty", got)
	}
	if got := (ChecklistItemCreated{}).EntityUUID(); got != "" {
		t.Errorf("nil checklist item EntityUUID() = %q, want empty", got)
	}
}

// TestLoggedChangeGetters covers the LoggedChange accessors, including Payload.
func TestLoggedChangeGetters(t *testing.T) {
	t.Parallel()
	lc := LoggedChange{
		changeType: "TaskCreated",
		entityType: "Task",
		entityUUID: "task-1",
		payload:    `{"tt":"hi"}`,
	}
	if lc.ChangeType() != "TaskCreated" {
		t.Errorf("ChangeType() = %q", lc.ChangeType())
	}
	if lc.EntityType() != "Task" {
		t.Errorf("EntityType() = %q", lc.EntityType())
	}
	if lc.EntityUUID() != "task-1" {
		t.Errorf("EntityUUID() = %q", lc.EntityUUID())
	}
	if lc.Payload() != `{"tt":"hi"}` {
		t.Errorf("Payload() = %q", lc.Payload())
	}
}

func TestStateAccessors(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	syncer.saveArea(&things.Area{UUID: "area-1", Title: "Work"})
	syncer.saveTag(&things.Tag{UUID: "tag-1", Title: "Urgent", ShortHand: "u"})
	syncer.saveTag(&things.Tag{UUID: "tag-2", Title: "Later", ShortHand: "l"})

	state := syncer.State()

	t.Run("Area", func(t *testing.T) {
		area, err := state.Area("area-1")
		if err != nil {
			t.Fatalf("Area failed: %v", err)
		}
		if area == nil || area.Title != "Work" {
			t.Errorf("expected area 'Work', got %v", area)
		}
	})

	t.Run("Tag", func(t *testing.T) {
		tag, err := state.Tag("tag-1")
		if err != nil {
			t.Fatalf("Tag failed: %v", err)
		}
		if tag == nil || tag.Title != "Urgent" {
			t.Errorf("expected tag 'Urgent', got %v", tag)
		}
	})

	t.Run("AllTags", func(t *testing.T) {
		tags, err := state.AllTags()
		if err != nil {
			t.Fatalf("AllTags failed: %v", err)
		}
		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(tags))
		}
	})
}

func TestStateProjectAreaAndChecklistQueries(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	syncer.saveTask(&things.Task{UUID: "proj-1", Title: "Project", Type: things.TaskTypeProject})
	syncer.saveTask(&things.Task{UUID: "in-project", Title: "In Project", Type: things.TaskTypeTask, ParentTaskIDs: []string{"proj-1"}, Status: things.TaskStatusPending})
	syncer.saveTask(&things.Task{UUID: "in-area", Title: "In Area", Type: things.TaskTypeTask, AreaIDs: []string{"area-1"}, Status: things.TaskStatusPending})
	syncer.saveChecklistItem(&things.CheckListItem{UUID: "cli-1", Title: "Step 1", Index: 0, TaskIDs: []string{"in-project"}})
	syncer.saveChecklistItem(&things.CheckListItem{UUID: "cli-2", Title: "Step 2", Index: 1, TaskIDs: []string{"in-project"}})

	state := syncer.State()

	t.Run("TasksInProject", func(t *testing.T) {
		tasks, err := state.TasksInProject("proj-1", QueryOpts{})
		if err != nil {
			t.Fatalf("TasksInProject failed: %v", err)
		}
		if len(tasks) != 1 || tasks[0].UUID != "in-project" {
			t.Errorf("expected [in-project], got %v", tasks)
		}
	})

	t.Run("TasksInArea", func(t *testing.T) {
		tasks, err := state.TasksInArea("area-1", QueryOpts{})
		if err != nil {
			t.Fatalf("TasksInArea failed: %v", err)
		}
		if len(tasks) != 1 || tasks[0].UUID != "in-area" {
			t.Errorf("expected [in-area], got %v", tasks)
		}
	})

	t.Run("ChecklistItems", func(t *testing.T) {
		items, err := state.ChecklistItems("in-project")
		if err != nil {
			t.Fatalf("ChecklistItems failed: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 checklist items, got %d", len(items))
		}
		if items[0].UUID != "cli-1" || items[1].UUID != "cli-2" {
			t.Errorf("unexpected order: %q %q", items[0].UUID, items[1].UUID)
		}
		if len(items[0].TaskIDs) != 1 || items[0].TaskIDs[0] != "in-project" {
			t.Errorf("expected TaskIDs to be set to parent, got %v", items[0].TaskIDs)
		}
	})
}

// TestSearchTasksEscapesLikeWildcards verifies that LIKE metacharacters in the
// query are escaped rather than treated as wildcards.
func TestSearchTasksEscapesLikeWildcards(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	opts := QueryOpts{}
	syncer.saveTask(&things.Task{UUID: "pct", Title: "50% off", Schedule: things.TaskScheduleAnytime, Status: things.TaskStatusPending})
	syncer.saveTask(&things.Task{UUID: "other", Title: "nothing special", Schedule: things.TaskScheduleAnytime, Status: things.TaskStatusPending})

	state := syncer.State()

	// A bare "%" must not match every task; it should be treated literally.
	tasks, err := state.SearchTasks("%", opts)
	if err != nil {
		t.Fatalf("SearchTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].UUID != "pct" {
		t.Errorf("expected only the literal '%%' match, got %v", tasks)
	}

	// Blank query returns an empty (non-nil) slice.
	empty, err := state.SearchTasks("   ", opts)
	if err != nil {
		t.Fatalf("SearchTasks failed: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("expected empty non-nil slice, got %v", empty)
	}
}

// TestProcessTagChecklistTombstone drives processItems through the tag,
// checklist, and tombstone code paths.
func TestProcessTagChecklistTombstone(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("tag create then delete", func(t *testing.T) {
		title := "Urgent"
		shorthand := "u"
		p, _ := json.Marshal(things.TagActionItemPayload{Title: &title, ShortHand: &shorthand})
		changes, err := syncer.processItems([]things.Item{{
			UUID: "tag-x", Kind: things.ItemKindTag, Action: things.ItemActionCreated, P: p,
		}}, 0)
		if err != nil {
			t.Fatalf("processItems (tag create) failed: %v", err)
		}
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TagCreated); !ok {
			t.Errorf("expected TagCreated, got %T", changes[0])
		}

		delChanges, err := syncer.processItems([]things.Item{{
			UUID: "tag-x", Kind: things.ItemKindTag, Action: things.ItemActionDeleted, P: json.RawMessage(`{}`),
		}}, 1)
		if err != nil {
			t.Fatalf("processItems (tag delete) failed: %v", err)
		}
		if len(delChanges) != 1 {
			t.Fatalf("expected 1 delete change, got %d", len(delChanges))
		}
		if _, ok := delChanges[0].(TagDeleted); !ok {
			t.Errorf("expected TagDeleted, got %T", delChanges[0])
		}
	})

	t.Run("checklist item create and delete", func(t *testing.T) {
		syncer.saveTask(&things.Task{UUID: "parent-task", Title: "Parent"})
		title := "A step"
		p, _ := json.Marshal(things.CheckListActionItemPayload{Title: &title, TaskIDs: &[]string{"parent-task"}})
		changes, err := syncer.processItems([]things.Item{{
			UUID: "cli-x", Kind: things.ItemKindChecklistItem, Action: things.ItemActionCreated, P: p,
		}}, 2)
		if err != nil {
			t.Fatalf("processItems (checklist create) failed: %v", err)
		}
		created, ok := changes[0].(ChecklistItemCreated)
		if !ok {
			t.Fatalf("expected ChecklistItemCreated, got %T", changes[0])
		}
		if created.Task == nil || created.Task.UUID != "parent-task" {
			t.Error("expected parent task to be resolved on the change")
		}

		delChanges, err := syncer.processItems([]things.Item{{
			UUID: "cli-x", Kind: things.ItemKindChecklistItem, Action: things.ItemActionDeleted, P: json.RawMessage(`{}`),
		}}, 3)
		if err != nil {
			t.Fatalf("processItems (checklist delete) failed: %v", err)
		}
		if _, ok := delChanges[0].(ChecklistItemDeleted); !ok {
			t.Errorf("expected ChecklistItemDeleted, got %T", delChanges[0])
		}
	})

	t.Run("tombstone deletes a task", func(t *testing.T) {
		syncer.saveTask(&things.Task{UUID: "doomed", Title: "Doomed Task"})
		p, _ := json.Marshal(things.TombstoneActionItemPayload{DeletedObjectID: "doomed"})
		changes, err := syncer.processItems([]things.Item{{
			UUID: "tomb-1", Kind: things.ItemKindTombstone, Action: things.ItemActionCreated, P: p,
		}}, 4)
		if err != nil {
			t.Fatalf("processItems (tombstone) failed: %v", err)
		}
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskDeleted); !ok {
			t.Errorf("expected TaskDeleted, got %T", changes[0])
		}
		task, _ := syncer.State().Task("doomed")
		if task != nil {
			t.Error("expected task to be soft-deleted after tombstone")
		}
	})

	t.Run("tombstone for unknown object is a no-op", func(t *testing.T) {
		p, _ := json.Marshal(things.TombstoneActionItemPayload{DeletedObjectID: "never-existed"})
		changes, err := syncer.processItems([]things.Item{{
			UUID: "tomb-2", Kind: things.ItemKindTombstone, Action: things.ItemActionCreated, P: p,
		}}, 5)
		if err != nil {
			t.Fatalf("processItems (unknown tombstone) failed: %v", err)
		}
		if len(changes) != 0 {
			t.Errorf("expected no changes for unknown tombstone, got %d", len(changes))
		}
	})
}

// TestProcessNotePayloads covers parseNotePayload for plain-string, full-text,
// and delta note representations.
func TestProcessNotePayloads(t *testing.T) {
	t.Parallel()

	t.Run("plain string note", func(t *testing.T) {
		t.Parallel()
		got := parseNotePayload("old", json.RawMessage(`"a plain note"`))
		if got != "a plain note" {
			t.Errorf("expected plain string note, got %q", got)
		}
	})

	t.Run("full text note", func(t *testing.T) {
		t.Parallel()
		raw, _ := json.Marshal(things.Note{Type: things.NoteTypeFullText, Value: "full replacement"})
		got := parseNotePayload("old", raw)
		if got != "full replacement" {
			t.Errorf("expected full-text replacement, got %q", got)
		}
	})

	t.Run("delta note applies patches", func(t *testing.T) {
		t.Parallel()
		raw, _ := json.Marshal(things.Note{
			Type:    things.NoteTypeDelta,
			Patches: []things.NotePatch{{Position: 0, Length: 0, Replacement: "Hi "}},
		})
		got := parseNotePayload("there", raw)
		if got != "Hi there" {
			t.Errorf("expected patched note 'Hi there', got %q", got)
		}
	})

	t.Run("invalid note keeps current value", func(t *testing.T) {
		t.Parallel()
		got := parseNotePayload("unchanged", json.RawMessage(`12345`))
		if got != "unchanged" {
			t.Errorf("expected current note preserved, got %q", got)
		}
	})

	t.Run("note delivered through processItems", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "test.db")
		syncer, err := Open(dbPath, nil)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer syncer.Close()

		title := "Task with note"
		tp := things.TaskTypeTask
		note := json.RawMessage(`"initial note"`)
		p, _ := json.Marshal(things.TaskActionItemPayload{Title: &title, Type: &tp, Note: note})
		if _, err := syncer.processItems([]things.Item{{
			UUID: "note-task", Kind: things.ItemKindTask, Action: things.ItemActionCreated, P: p,
		}}, 0); err != nil {
			t.Fatalf("processItems failed: %v", err)
		}

		task, _ := syncer.State().Task("note-task")
		if task == nil || task.Note != "initial note" {
			t.Errorf("expected note 'initial note', got %v", task)
		}
	})
}

// TestChangesSinceAndForEntity exercises the timestamp- and entity-scoped
// change-log queries.
func TestChangesSinceAndForEntity(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	title := "Logged task"
	tp := things.TaskTypeTask
	p, _ := json.Marshal(things.TaskActionItemPayload{Title: &title, Type: &tp})
	if _, err := syncer.processItems([]things.Item{{
		UUID: "logged-1", Kind: things.ItemKindTask, Action: things.ItemActionCreated, P: p,
	}}, 0); err != nil {
		t.Fatalf("processItems failed: %v", err)
	}

	t.Run("ChangesSince returns recent entries", func(t *testing.T) {
		changes, err := syncer.ChangesSince(time.Unix(0, 0))
		if err != nil {
			t.Fatalf("ChangesSince failed: %v", err)
		}
		if len(changes) != 1 {
			t.Fatalf("expected 1 change since epoch, got %d", len(changes))
		}
		if changes[0].EntityUUID() != "logged-1" {
			t.Errorf("expected entity logged-1, got %q", changes[0].EntityUUID())
		}
	})

	t.Run("ChangesSince excludes future window", func(t *testing.T) {
		changes, err := syncer.ChangesSince(time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("ChangesSince failed: %v", err)
		}
		if len(changes) != 0 {
			t.Errorf("expected 0 changes in future window, got %d", len(changes))
		}
	})

	t.Run("ChangesForEntity filters by UUID", func(t *testing.T) {
		changes, err := syncer.ChangesForEntity("logged-1")
		if err != nil {
			t.Fatalf("ChangesForEntity failed: %v", err)
		}
		if len(changes) != 1 {
			t.Fatalf("expected 1 change for logged-1, got %d", len(changes))
		}

		none, err := syncer.ChangesForEntity("no-such-entity")
		if err != nil {
			t.Fatalf("ChangesForEntity failed: %v", err)
		}
		if len(none) != 0 {
			t.Errorf("expected 0 changes for unknown entity, got %d", len(none))
		}
	})
}
