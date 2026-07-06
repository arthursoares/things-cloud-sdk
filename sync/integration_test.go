package sync

import (
	"encoding/json"
	"path/filepath"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func TestIntegration(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("process task creation", func(t *testing.T) {
		payload := things.TaskActionItemPayload{}
		title := "Buy groceries"
		payload.Title = &title
		tp := things.TaskTypeTask
		payload.Type = &tp

		payloadBytes, _ := json.Marshal(payload)
		item := things.Item{
			UUID:   "task-001",
			Kind:   things.ItemKindTask,
			Action: things.ItemActionCreated,
			P:      payloadBytes,
		}

		changes, err := syncer.processItems([]things.Item{item}, 0)
		if err != nil {
			t.Fatalf("processItems failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		created, ok := changes[0].(TaskCreated)
		if !ok {
			t.Fatalf("expected TaskCreated, got %T", changes[0])
		}
		if created.Task.Title != "Buy groceries" {
			t.Errorf("expected title 'Buy groceries', got %q", created.Task.Title)
		}

		// Verify task was persisted
		state := syncer.State()
		task, err := state.Task("task-001")
		if err != nil {
			t.Fatalf("Task lookup failed: %v", err)
		}
		if task == nil {
			t.Fatal("task not persisted")
		}
	})

	t.Run("process task completion", func(t *testing.T) {
		payload := things.TaskActionItemPayload{}
		status := things.TaskStatusCompleted
		payload.Status = &status

		payloadBytes, _ := json.Marshal(payload)
		item := things.Item{
			UUID:   "task-001",
			Kind:   things.ItemKindTask,
			Action: things.ItemActionModified,
			P:      payloadBytes,
		}

		changes, err := syncer.processItems([]things.Item{item}, 1)
		if err != nil {
			t.Fatalf("processItems failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		_, ok := changes[0].(TaskCompleted)
		if !ok {
			t.Fatalf("expected TaskCompleted, got %T", changes[0])
		}
	})

	t.Run("process area creation", func(t *testing.T) {
		payload := things.AreaActionItemPayload{}
		title := "Work"
		payload.Title = &title

		payloadBytes, _ := json.Marshal(payload)
		item := things.Item{
			UUID:   "area-001",
			Kind:   things.ItemKindArea3,
			Action: things.ItemActionCreated,
			P:      payloadBytes,
		}

		changes, err := syncer.processItems([]things.Item{item}, 2)
		if err != nil {
			t.Fatalf("processItems failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		_, ok := changes[0].(AreaCreated)
		if !ok {
			t.Fatalf("expected AreaCreated, got %T", changes[0])
		}

		state := syncer.State()
		areas, _ := state.AllAreas()
		if len(areas) != 1 {
			t.Errorf("expected 1 area, got %d", len(areas))
		}
	})

	t.Run("query change log", func(t *testing.T) {
		// ChangesSinceIndex(0) returns changes with server_index > 0
		// So we should get 2 changes (indexes 1 and 2)
		changes, err := syncer.ChangesSinceIndex(0)
		if err != nil {
			t.Fatalf("ChangesSinceIndex failed: %v", err)
		}

		if len(changes) != 2 {
			t.Errorf("expected 2 changes (indexes 1 and 2), got %d", len(changes))
		}

		// Test ChangesSinceIndex(-1) to get all changes
		allChanges, err := syncer.ChangesSinceIndex(-1)
		if err != nil {
			t.Fatalf("ChangesSinceIndex failed: %v", err)
		}

		if len(allChanges) != 3 {
			t.Errorf("expected 3 total changes, got %d", len(allChanges))
		}
		if allChanges[0].ChangeType() != "TaskCreated" {
			t.Errorf("expected first logged change type TaskCreated, got %s", allChanges[0].ChangeType())
		}
		if _, ok := allChanges[0].(LoggedChange); !ok {
			t.Errorf("expected persisted change to be LoggedChange, got %T", allChanges[0])
		}
	})
}

func TestProcessItemsUsesItemServerIndex(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	payload := things.TaskActionItemPayload{}
	title := "Indexed task"
	payload.Title = &title
	tp := things.TaskTypeTask
	payload.Type = &tp
	payloadBytes, _ := json.Marshal(payload)

	items := []things.Item{
		{
			UUID:           "task-a",
			Kind:           things.ItemKindTask,
			Action:         things.ItemActionCreated,
			P:              payloadBytes,
			ServerIndex:    7,
			HasServerIndex: true,
		},
		{
			UUID:           "task-b",
			Kind:           things.ItemKindTask,
			Action:         things.ItemActionCreated,
			P:              payloadBytes,
			ServerIndex:    7,
			HasServerIndex: true,
		},
		{
			UUID:           "task-c",
			Kind:           things.ItemKindTask,
			Action:         things.ItemActionCreated,
			P:              payloadBytes,
			ServerIndex:    8,
			HasServerIndex: true,
		},
	}

	changes, err := syncer.processItems(items, 100)
	if err != nil {
		t.Fatalf("processItems failed: %v", err)
	}
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	rows, err := syncer.db.Query(`SELECT entity_uuid, server_index FROM change_log`)
	if err != nil {
		t.Fatalf("querying change_log failed: %v", err)
	}
	defer rows.Close()

	indexByUUID := make(map[string]int)
	for rows.Next() {
		var uuid string
		var serverIndex int
		if err := rows.Scan(&uuid, &serverIndex); err != nil {
			t.Fatalf("scanning change_log failed: %v", err)
		}
		indexByUUID[uuid] = serverIndex
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading change_log failed: %v", err)
	}

	if indexByUUID["task-a"] != 7 {
		t.Errorf("task-a server_index = %d, want 7", indexByUUID["task-a"])
	}
	if indexByUUID["task-b"] != 7 {
		t.Errorf("task-b server_index = %d, want 7", indexByUUID["task-b"])
	}
	if indexByUUID["task-c"] != 8 {
		t.Errorf("task-c server_index = %d, want 8", indexByUUID["task-c"])
	}
}

func TestStateQueries(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	// Create test data directly
	syncer.saveTask(&things.Task{UUID: "inbox-1", Title: "Inbox Task", Schedule: things.TaskScheduleInbox, Status: things.TaskStatusPending})
	syncer.saveTask(&things.Task{UUID: "anytime-1", Title: "Anytime Task", Schedule: things.TaskScheduleAnytime, Status: things.TaskStatusPending})
	syncer.saveTask(&things.Task{UUID: "completed-1", Title: "Completed Task", Schedule: things.TaskScheduleAnytime, Status: things.TaskStatusCompleted})
	syncer.saveTask(&things.Task{UUID: "trashed-1", Title: "Trashed Task", InTrash: true})
	syncer.saveTask(&things.Task{UUID: "project-1", Title: "Test Project", Type: things.TaskTypeProject})

	state := syncer.State()

	t.Run("TasksInInbox excludes completed by default", func(t *testing.T) {
		tasks, err := state.TasksInInbox(QueryOpts{})
		if err != nil {
			t.Fatalf("TasksInInbox failed: %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 inbox task, got %d", len(tasks))
		}
	})

	t.Run("AllTasks excludes trashed by default", func(t *testing.T) {
		tasks, err := state.AllTasks(QueryOpts{})
		if err != nil {
			t.Fatalf("AllTasks failed: %v", err)
		}
		for _, task := range tasks {
			if task.InTrash {
				t.Error("trashed task should be excluded")
			}
		}
	})

	t.Run("AllTasks includes trashed when requested", func(t *testing.T) {
		tasks, err := state.AllTasks(QueryOpts{IncludeTrashed: true})
		if err != nil {
			t.Fatalf("AllTasks failed: %v", err)
		}
		found := false
		for _, task := range tasks {
			if task.UUID == "trashed-1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("trashed task should be included")
		}
	})

	t.Run("AllProjects returns only projects", func(t *testing.T) {
		projects, err := state.AllProjects(QueryOpts{})
		if err != nil {
			t.Fatalf("AllProjects failed: %v", err)
		}
		if len(projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(projects))
		}
		if projects[0].Type != things.TaskTypeProject {
			t.Error("returned task is not a project")
		}
	})
}
