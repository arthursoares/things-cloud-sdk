package sync

import (
	"path/filepath"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

func TestTaskStorage(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("save and retrieve task", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		task := &things.Task{
			UUID:         "task-123",
			Title:        "Test Task",
			Note:         "Some notes",
			Status:       things.TaskStatusPending,
			Schedule:     things.TaskScheduleAnytime,
			Type:         things.TaskTypeTask,
			CreationDate: now,
			TagIDs:       []string{"tag-1", "tag-2"},
		}

		if err := syncer.saveTask(task); err != nil {
			t.Fatalf("saveTask failed: %v", err)
		}

		retrieved, err := syncer.getTask("task-123")
		if err != nil {
			t.Fatalf("getTask failed: %v", err)
		}
		if retrieved == nil {
			t.Fatal("task not found")
		}
		if retrieved.Title != "Test Task" {
			t.Errorf("Title mismatch: got %q", retrieved.Title)
		}
		if retrieved.Note != "Some notes" {
			t.Errorf("Note mismatch: got %q", retrieved.Note)
		}
		if len(retrieved.TagIDs) != 2 {
			t.Errorf("TagIDs mismatch: got %v", retrieved.TagIDs)
		}
	})

	t.Run("get non-existent task returns nil", func(t *testing.T) {
		retrieved, err := syncer.getTask("non-existent")
		if err != nil {
			t.Fatalf("getTask failed: %v", err)
		}
		if retrieved != nil {
			t.Error("expected nil for non-existent task")
		}
	})

	t.Run("soft delete task", func(t *testing.T) {
		task := &things.Task{UUID: "to-delete", Title: "Delete Me"}
		mustSaveTask(t, syncer, task)

		if err := syncer.markTaskDeleted("to-delete"); err != nil {
			t.Fatalf("markTaskDeleted failed: %v", err)
		}

		var deleted int
		if err := syncer.db.QueryRow("SELECT deleted FROM tasks WHERE uuid = 'to-delete'").Scan(&deleted); err != nil {
			t.Fatalf("read deleted flag: %v", err)
		}
		if deleted != 1 {
			t.Error("task not marked as deleted")
		}
	})

	t.Run("save task with all optional fields", func(t *testing.T) {
		now := time.Now().Truncate(time.Second).UTC()
		scheduled := now.Add(24 * time.Hour)
		deadline := now.Add(48 * time.Hour)
		completion := now.Add(time.Hour)
		modification := now.Add(30 * time.Minute)
		alarmOffset := 3600

		task := &things.Task{
			UUID:             "task-full",
			Title:            "Full Task",
			Note:             "Detailed notes here",
			Status:           things.TaskStatusCompleted,
			Schedule:         things.TaskScheduleSomeday,
			Type:             things.TaskTypeProject,
			CreationDate:     now,
			ModificationDate: &modification,
			ScheduledDate:    &scheduled,
			DeadlineDate:     &deadline,
			CompletionDate:   &completion,
			Index:            5,
			TodayIndex:       3,
			InTrash:          true,
			AreaIDs:          []string{"area-1"},
			ParentTaskIDs:    []string{"project-1"},
			ActionGroupIDs:   []string{"heading-1"},
			AlarmTimeOffset:  &alarmOffset,
			TagIDs:           []string{"tag-a", "tag-b", "tag-c"},
		}

		if err := syncer.saveTask(task); err != nil {
			t.Fatalf("saveTask failed: %v", err)
		}

		retrieved, err := syncer.getTask("task-full")
		if err != nil {
			t.Fatalf("getTask failed: %v", err)
		}
		if retrieved == nil {
			t.Fatal("task not found")
		}

		// Verify all fields
		if retrieved.Status != things.TaskStatusCompleted {
			t.Errorf("Status mismatch: got %v", retrieved.Status)
		}
		if retrieved.Schedule != things.TaskScheduleSomeday {
			t.Errorf("Schedule mismatch: got %v", retrieved.Schedule)
		}
		if retrieved.Type != things.TaskTypeProject {
			t.Errorf("Type mismatch: got %v", retrieved.Type)
		}
		if retrieved.Index != 5 {
			t.Errorf("Index mismatch: got %d", retrieved.Index)
		}
		if retrieved.TodayIndex != 3 {
			t.Errorf("TodayIndex mismatch: got %d", retrieved.TodayIndex)
		}
		if !retrieved.InTrash {
			t.Error("InTrash should be true")
		}
		if len(retrieved.AreaIDs) != 1 || retrieved.AreaIDs[0] != "area-1" {
			t.Errorf("AreaIDs mismatch: got %v", retrieved.AreaIDs)
		}
		if len(retrieved.ParentTaskIDs) != 1 || retrieved.ParentTaskIDs[0] != "project-1" {
			t.Errorf("ParentTaskIDs mismatch: got %v", retrieved.ParentTaskIDs)
		}
		if len(retrieved.ActionGroupIDs) != 1 || retrieved.ActionGroupIDs[0] != "heading-1" {
			t.Errorf("ActionGroupIDs mismatch: got %v", retrieved.ActionGroupIDs)
		}
		if retrieved.AlarmTimeOffset == nil || *retrieved.AlarmTimeOffset != 3600 {
			t.Errorf("AlarmTimeOffset mismatch: got %v", retrieved.AlarmTimeOffset)
		}
		if len(retrieved.TagIDs) != 3 {
			t.Errorf("TagIDs count mismatch: got %d", len(retrieved.TagIDs))
		}

		// Verify dates (compare Unix timestamps to avoid timezone issues)
		if retrieved.ScheduledDate == nil || retrieved.ScheduledDate.Unix() != scheduled.Unix() {
			t.Errorf("ScheduledDate mismatch")
		}
		if retrieved.DeadlineDate == nil || retrieved.DeadlineDate.Unix() != deadline.Unix() {
			t.Errorf("DeadlineDate mismatch")
		}
		if retrieved.CompletionDate == nil || retrieved.CompletionDate.Unix() != completion.Unix() {
			t.Errorf("CompletionDate mismatch")
		}
		if retrieved.ModificationDate == nil || retrieved.ModificationDate.Unix() != modification.Unix() {
			t.Errorf("ModificationDate mismatch")
		}
	})

	t.Run("update existing task", func(t *testing.T) {
		task := &things.Task{UUID: "task-update", Title: "Original Title"}
		mustSaveTask(t, syncer, task)

		task.Title = "Updated Title"
		task.Note = "Added notes"
		if err := syncer.saveTask(task); err != nil {
			t.Fatalf("saveTask update failed: %v", err)
		}

		retrieved, _ := syncer.getTask("task-update")
		if retrieved.Title != "Updated Title" {
			t.Errorf("Title not updated: got %q", retrieved.Title)
		}
		if retrieved.Note != "Added notes" {
			t.Errorf("Note not updated: got %q", retrieved.Note)
		}
	})

	t.Run("update task tags", func(t *testing.T) {
		task := &things.Task{UUID: "task-tags", Title: "Tagged Task", TagIDs: []string{"old-tag"}}
		mustSaveTask(t, syncer, task)

		task.TagIDs = []string{"new-tag-1", "new-tag-2"}
		mustSaveTask(t, syncer, task)

		retrieved, _ := syncer.getTask("task-tags")
		if len(retrieved.TagIDs) != 2 {
			t.Errorf("TagIDs not updated: got %v", retrieved.TagIDs)
		}
	})
}

func TestAreaStorage(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("save and retrieve area", func(t *testing.T) {
		area := &things.Area{UUID: "area-123", Title: "Work"}
		if err := syncer.saveArea(area); err != nil {
			t.Fatalf("saveArea failed: %v", err)
		}

		retrieved, err := syncer.getArea("area-123")
		if err != nil {
			t.Fatalf("getArea failed: %v", err)
		}
		if retrieved == nil {
			t.Fatal("area not found")
		}
		if retrieved.Title != "Work" {
			t.Errorf("Title mismatch: got %q", retrieved.Title)
		}
		if retrieved.UUID != "area-123" {
			t.Errorf("UUID mismatch: got %q", retrieved.UUID)
		}
	})

	t.Run("get non-existent area returns nil", func(t *testing.T) {
		retrieved, err := syncer.getArea("non-existent-area")
		if err != nil {
			t.Fatalf("getArea failed: %v", err)
		}
		if retrieved != nil {
			t.Error("expected nil for non-existent area")
		}
	})

	t.Run("soft delete area", func(t *testing.T) {
		area := &things.Area{UUID: "area-to-delete", Title: "Delete Me"}
		mustSaveArea(t, syncer, area)

		if err := syncer.markAreaDeleted("area-to-delete"); err != nil {
			t.Fatalf("markAreaDeleted failed: %v", err)
		}

		// Deleted area should not be returned
		retrieved, _ := syncer.getArea("area-to-delete")
		if retrieved != nil {
			t.Error("deleted area should not be returned")
		}

		// But record should still exist in DB
		var deleted int
		if err := syncer.db.QueryRow("SELECT deleted FROM areas WHERE uuid = 'area-to-delete'").Scan(&deleted); err != nil {
			t.Fatalf("read deleted flag: %v", err)
		}
		if deleted != 1 {
			t.Error("area not marked as deleted in database")
		}
	})

	t.Run("update existing area", func(t *testing.T) {
		area := &things.Area{UUID: "area-update", Title: "Original"}
		mustSaveArea(t, syncer, area)

		area.Title = "Updated"
		mustSaveArea(t, syncer, area)

		retrieved, _ := syncer.getArea("area-update")
		if retrieved.Title != "Updated" {
			t.Errorf("Title not updated: got %q", retrieved.Title)
		}
	})
}

func TestTagStorage(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("save and retrieve tag", func(t *testing.T) {
		tag := &things.Tag{UUID: "tag-123", Title: "Urgent", ShortHand: "u"}
		if err := syncer.saveTag(tag); err != nil {
			t.Fatalf("saveTag failed: %v", err)
		}

		retrieved, err := syncer.getTag("tag-123")
		if err != nil {
			t.Fatalf("getTag failed: %v", err)
		}
		if retrieved == nil {
			t.Fatal("tag not found")
		}
		if retrieved.Title != "Urgent" {
			t.Errorf("Title mismatch: got %q", retrieved.Title)
		}
		if retrieved.ShortHand != "u" {
			t.Errorf("ShortHand mismatch: got %q", retrieved.ShortHand)
		}
	})

	t.Run("get non-existent tag returns nil", func(t *testing.T) {
		retrieved, err := syncer.getTag("non-existent-tag")
		if err != nil {
			t.Fatalf("getTag failed: %v", err)
		}
		if retrieved != nil {
			t.Error("expected nil for non-existent tag")
		}
	})

	t.Run("save tag with parent", func(t *testing.T) {
		parentTag := &things.Tag{UUID: "parent-tag", Title: "Parent"}
		mustSaveTag(t, syncer, parentTag)

		childTag := &things.Tag{UUID: "child-tag", Title: "Child", ParentTagIDs: []string{"parent-tag"}}
		mustSaveTag(t, syncer, childTag)

		retrieved, _ := syncer.getTag("child-tag")
		if len(retrieved.ParentTagIDs) != 1 || retrieved.ParentTagIDs[0] != "parent-tag" {
			t.Errorf("ParentTagIDs mismatch: got %v", retrieved.ParentTagIDs)
		}
	})

	t.Run("soft delete tag", func(t *testing.T) {
		tag := &things.Tag{UUID: "tag-to-delete", Title: "Delete Me"}
		mustSaveTag(t, syncer, tag)

		if err := syncer.markTagDeleted("tag-to-delete"); err != nil {
			t.Fatalf("markTagDeleted failed: %v", err)
		}

		// Deleted tag should not be returned
		retrieved, _ := syncer.getTag("tag-to-delete")
		if retrieved != nil {
			t.Error("deleted tag should not be returned")
		}
	})

	t.Run("update existing tag", func(t *testing.T) {
		tag := &things.Tag{UUID: "tag-update", Title: "Original", ShortHand: "o"}
		mustSaveTag(t, syncer, tag)

		tag.Title = "Updated"
		tag.ShortHand = "u"
		mustSaveTag(t, syncer, tag)

		retrieved, _ := syncer.getTag("tag-update")
		if retrieved.Title != "Updated" {
			t.Errorf("Title not updated: got %q", retrieved.Title)
		}
		if retrieved.ShortHand != "u" {
			t.Errorf("ShortHand not updated: got %q", retrieved.ShortHand)
		}
	})
}

func TestChecklistItemStorage(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("save and retrieve checklist item", func(t *testing.T) {
		now := time.Now().Truncate(time.Second).UTC()
		item := &things.CheckListItem{
			UUID:         "checklist-123",
			Title:        "Buy milk",
			Status:       things.TaskStatusPending,
			Index:        0,
			CreationDate: now,
			TaskIDs:      []string{"task-parent"},
		}

		if err := syncer.saveChecklistItem(item); err != nil {
			t.Fatalf("saveChecklistItem failed: %v", err)
		}

		retrieved, err := syncer.getChecklistItem("checklist-123")
		if err != nil {
			t.Fatalf("getChecklistItem failed: %v", err)
		}
		if retrieved == nil {
			t.Fatal("checklist item not found")
		}
		if retrieved.Title != "Buy milk" {
			t.Errorf("Title mismatch: got %q", retrieved.Title)
		}
		if retrieved.Status != things.TaskStatusPending {
			t.Errorf("Status mismatch: got %v", retrieved.Status)
		}
		if len(retrieved.TaskIDs) != 1 || retrieved.TaskIDs[0] != "task-parent" {
			t.Errorf("TaskIDs mismatch: got %v", retrieved.TaskIDs)
		}
	})

	t.Run("get non-existent checklist item returns nil", func(t *testing.T) {
		retrieved, err := syncer.getChecklistItem("non-existent-checklist")
		if err != nil {
			t.Fatalf("getChecklistItem failed: %v", err)
		}
		if retrieved != nil {
			t.Error("expected nil for non-existent checklist item")
		}
	})

	t.Run("save completed checklist item", func(t *testing.T) {
		now := time.Now().Truncate(time.Second).UTC()
		completedAt := now.Add(time.Hour)

		item := &things.CheckListItem{
			UUID:           "checklist-completed",
			Title:          "Done item",
			Status:         things.TaskStatusCompleted,
			Index:          1,
			CreationDate:   now,
			CompletionDate: &completedAt,
		}

		mustSaveChecklistItem(t, syncer, item)

		retrieved, _ := syncer.getChecklistItem("checklist-completed")
		if retrieved.Status != things.TaskStatusCompleted {
			t.Errorf("Status mismatch: got %v", retrieved.Status)
		}
		if retrieved.CompletionDate == nil || retrieved.CompletionDate.Unix() != completedAt.Unix() {
			t.Error("CompletionDate mismatch")
		}
	})

	t.Run("soft delete checklist item", func(t *testing.T) {
		item := &things.CheckListItem{UUID: "checklist-to-delete", Title: "Delete Me"}
		mustSaveChecklistItem(t, syncer, item)

		if err := syncer.markChecklistItemDeleted("checklist-to-delete"); err != nil {
			t.Fatalf("markChecklistItemDeleted failed: %v", err)
		}

		// Deleted item should not be returned
		retrieved, _ := syncer.getChecklistItem("checklist-to-delete")
		if retrieved != nil {
			t.Error("deleted checklist item should not be returned")
		}
	})

	t.Run("update existing checklist item", func(t *testing.T) {
		item := &things.CheckListItem{UUID: "checklist-update", Title: "Original", Index: 0}
		mustSaveChecklistItem(t, syncer, item)

		item.Title = "Updated"
		item.Index = 5
		mustSaveChecklistItem(t, syncer, item)

		retrieved, _ := syncer.getChecklistItem("checklist-update")
		if retrieved.Title != "Updated" {
			t.Errorf("Title not updated: got %q", retrieved.Title)
		}
		if retrieved.Index != 5 {
			t.Errorf("Index not updated: got %d", retrieved.Index)
		}
	})
}

func TestSyncStateStorage(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("get empty sync state", func(t *testing.T) {
		historyID, serverIndex, err := syncer.getSyncState()
		if err != nil {
			t.Fatalf("getSyncState failed: %v", err)
		}
		if historyID != "" {
			t.Errorf("expected empty historyID, got %q", historyID)
		}
		if serverIndex != 0 {
			t.Errorf("expected 0 serverIndex, got %d", serverIndex)
		}
	})

	t.Run("save and retrieve sync state", func(t *testing.T) {
		if err := syncer.saveSyncState("history-abc", 42); err != nil {
			t.Fatalf("saveSyncState failed: %v", err)
		}

		historyID, serverIndex, err := syncer.getSyncState()
		if err != nil {
			t.Fatalf("getSyncState failed: %v", err)
		}
		if historyID != "history-abc" {
			t.Errorf("historyID mismatch: got %q", historyID)
		}
		if serverIndex != 42 {
			t.Errorf("serverIndex mismatch: got %d", serverIndex)
		}
	})

	t.Run("update sync state", func(t *testing.T) {
		if err := syncer.saveSyncState("history-1", 10); err != nil {
			t.Fatalf("saveSyncState: %v", err)
		}
		if err := syncer.saveSyncState("history-2", 100); err != nil {
			t.Fatalf("saveSyncState: %v", err)
		}

		historyID, serverIndex, _ := syncer.getSyncState()
		if historyID != "history-2" {
			t.Errorf("historyID not updated: got %q", historyID)
		}
		if serverIndex != 100 {
			t.Errorf("serverIndex not updated: got %d", serverIndex)
		}
	})
}

func TestLogChange(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	t.Run("log change entry", func(t *testing.T) {
		change := TaskCreated{taskChange: taskChange{Task: &things.Task{UUID: "test-task", Title: "Test"}}}
		if err := syncer.logChange(1, change, `{"test": true}`); err != nil {
			t.Fatalf("logChange failed: %v", err)
		}

		// Verify entry was logged
		var count int
		if err := syncer.db.QueryRow("SELECT COUNT(*) FROM change_log WHERE entity_uuid = 'test-task'").Scan(&count); err != nil {
			t.Fatalf("count change_log entries: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 log entry, got %d", count)
		}

		// Verify fields
		var changeType, entityType, entityUUID, payload string
		if err := syncer.db.QueryRow("SELECT change_type, entity_type, entity_uuid, payload FROM change_log WHERE entity_uuid = 'test-task'").
			Scan(&changeType, &entityType, &entityUUID, &payload); err != nil {
			t.Fatalf("read change_log entry: %v", err)
		}
		if changeType != "TaskCreated" {
			t.Errorf("changeType mismatch: got %q", changeType)
		}
		if entityType != "Task" {
			t.Errorf("entityType mismatch: got %q", entityType)
		}
		if payload != `{"test": true}` {
			t.Errorf("payload mismatch: got %q", payload)
		}
	})
}
