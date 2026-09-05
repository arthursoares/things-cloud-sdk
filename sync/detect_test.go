package sync

import (
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

func TestDetectTaskChanges(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("task created", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{UUID: "t1", Title: "New Task", Type: things.TaskTypeTask}
		changes := detectTaskChanges(nil, task, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskCreated); !ok {
			t.Errorf("expected TaskCreated, got %T", changes[0])
		}
	})

	t.Run("project created", func(t *testing.T) {
		t.Parallel()
		project := &things.Task{UUID: "p1", Title: "New Project", Type: things.TaskTypeProject}
		changes := detectTaskChanges(nil, project, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(ProjectCreated); !ok {
			t.Errorf("expected ProjectCreated, got %T", changes[0])
		}
	})

	t.Run("heading created", func(t *testing.T) {
		t.Parallel()
		heading := &things.Task{UUID: "h1", Title: "New Heading", Type: things.TaskTypeHeading}
		changes := detectTaskChanges(nil, heading, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(HeadingCreated); !ok {
			t.Errorf("expected HeadingCreated, got %T", changes[0])
		}
	})

	t.Run("task deleted", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{UUID: "t1", Title: "Task", Type: things.TaskTypeTask}
		changes := detectTaskChanges(task, nil, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskDeleted); !ok {
			t.Errorf("expected TaskDeleted, got %T", changes[0])
		}
	})

	t.Run("project deleted", func(t *testing.T) {
		t.Parallel()
		project := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject}
		changes := detectTaskChanges(project, nil, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(ProjectDeleted); !ok {
			t.Errorf("expected ProjectDeleted, got %T", changes[0])
		}
	})

	t.Run("heading deleted", func(t *testing.T) {
		t.Parallel()
		heading := &things.Task{UUID: "h1", Title: "Heading", Type: things.TaskTypeHeading}
		changes := detectTaskChanges(heading, nil, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(HeadingDeleted); !ok {
			t.Errorf("expected HeadingDeleted, got %T", changes[0])
		}
	})

	t.Run("task completed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusPending}
		new := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusCompleted}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskCompleted); !ok {
			t.Errorf("expected TaskCompleted, got %T", changes[0])
		}
	})

	t.Run("project completed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, Status: things.TaskStatusPending}
		new := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, Status: things.TaskStatusCompleted}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(ProjectCompleted); !ok {
			t.Errorf("expected ProjectCompleted, got %T", changes[0])
		}
	})

	t.Run("task uncompleted", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusCompleted}
		new := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusPending}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskUncompleted); !ok {
			t.Errorf("expected TaskUncompleted, got %T", changes[0])
		}
	})

	t.Run("task canceled", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusPending}
		new := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusCanceled}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskCanceled); !ok {
			t.Errorf("expected TaskCanceled, got %T", changes[0])
		}
	})

	t.Run("task title changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Old Title"}
		new := &things.Task{UUID: "t1", Title: "New Title"}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(TaskTitleChanged)
		if !ok {
			t.Fatalf("expected TaskTitleChanged, got %T", changes[0])
		}
		if tc.OldTitle != "Old Title" {
			t.Errorf("expected OldTitle 'Old Title', got %q", tc.OldTitle)
		}
	})

	t.Run("project title changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "p1", Title: "Old Project", Type: things.TaskTypeProject}
		new := &things.Task{UUID: "p1", Title: "New Project", Type: things.TaskTypeProject}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		pc, ok := changes[0].(ProjectTitleChanged)
		if !ok {
			t.Fatalf("expected ProjectTitleChanged, got %T", changes[0])
		}
		if pc.OldTitle != "Old Project" {
			t.Errorf("expected OldTitle 'Old Project', got %q", pc.OldTitle)
		}
	})

	t.Run("heading title changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "h1", Title: "Old Heading", Type: things.TaskTypeHeading}
		new := &things.Task{UUID: "h1", Title: "New Heading", Type: things.TaskTypeHeading}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		hc, ok := changes[0].(HeadingTitleChanged)
		if !ok {
			t.Fatalf("expected HeadingTitleChanged, got %T", changes[0])
		}
		if hc.OldTitle != "Old Heading" {
			t.Errorf("expected OldTitle 'Old Heading', got %q", hc.OldTitle)
		}
	})

	t.Run("task note changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Note: "Old note"}
		new := &things.Task{UUID: "t1", Title: "Task", Note: "New note"}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		nc, ok := changes[0].(TaskNoteChanged)
		if !ok {
			t.Fatalf("expected TaskNoteChanged, got %T", changes[0])
		}
		if nc.OldNote != "Old note" {
			t.Errorf("expected OldNote 'Old note', got %q", nc.OldNote)
		}
	})

	t.Run("heading note change ignored", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "h1", Title: "Heading", Type: things.TaskTypeHeading, Note: "Old note"}
		new := &things.Task{UUID: "h1", Title: "Heading", Type: things.TaskTypeHeading, Note: "New note"}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes (headings ignore notes), got %d: %v", len(changes), changes)
		}
	})

	t.Run("task trashed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", InTrash: false}
		new := &things.Task{UUID: "t1", Title: "Task", InTrash: true}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskTrashed); !ok {
			t.Errorf("expected TaskTrashed, got %T", changes[0])
		}
	})

	t.Run("project trashed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, InTrash: false}
		new := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, InTrash: true}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(ProjectTrashed); !ok {
			t.Errorf("expected ProjectTrashed, got %T", changes[0])
		}
	})

	t.Run("task restored", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", InTrash: true}
		new := &things.Task{UUID: "t1", Title: "Task", InTrash: false}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(TaskRestored); !ok {
			t.Errorf("expected TaskRestored, got %T", changes[0])
		}
	})

	t.Run("project restored", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, InTrash: true}
		new := &things.Task{UUID: "p1", Title: "Project", Type: things.TaskTypeProject, InTrash: false}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if _, ok := changes[0].(ProjectRestored); !ok {
			t.Errorf("expected ProjectRestored, got %T", changes[0])
		}
	})

	t.Run("task moved to today", func(t *testing.T) {
		t.Parallel()
		today := time.Now()
		old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleInbox}
		new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime, ScheduledDate: &today}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if _, ok := c.(TaskMovedToToday); ok {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToToday change")
		}
	})

	t.Run("task moved to inbox", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime}
		new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleInbox}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if mc, ok := c.(TaskMovedToInbox); ok {
				found = true
				if mc.From != LocationAnytime {
					t.Errorf("expected From LocationAnytime, got %v", mc.From)
				}
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToInbox change")
		}
	})

	t.Run("task moved to anytime", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleInbox}
		new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if mc, ok := c.(TaskMovedToAnytime); ok {
				found = true
				if mc.From != LocationInbox {
					t.Errorf("expected From LocationInbox, got %v", mc.From)
				}
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToAnytime change")
		}
	})

	t.Run("task moved to someday", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime}
		new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleSomeday}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if mc, ok := c.(TaskMovedToSomeday); ok {
				found = true
				if mc.From != LocationAnytime {
					t.Errorf("expected From LocationAnytime, got %v", mc.From)
				}
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToSomeday change")
		}
	})

	t.Run("task moved to upcoming", func(t *testing.T) {
		t.Parallel()
		futureDate := time.Now().AddDate(0, 0, 7) // one week in future
		old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime}
		new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleSomeday, ScheduledDate: &futureDate}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if mc, ok := c.(TaskMovedToUpcoming); ok {
				found = true
				if mc.From != LocationAnytime {
					t.Errorf("expected From LocationAnytime, got %v", mc.From)
				}
				if !mc.ScheduledFor.Equal(futureDate) {
					t.Errorf("expected ScheduledFor %v, got %v", futureDate, mc.ScheduledFor)
				}
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToUpcoming change")
		}
	})

	t.Run("task deadline changed", func(t *testing.T) {
		t.Parallel()
		oldDeadline := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		newDeadline := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
		old := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: &oldDeadline}
		new := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: &newDeadline}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		dc, ok := changes[0].(TaskDeadlineChanged)
		if !ok {
			t.Fatalf("expected TaskDeadlineChanged, got %T", changes[0])
		}
		if dc.OldDeadline == nil || !dc.OldDeadline.Equal(oldDeadline) {
			t.Errorf("expected OldDeadline %v, got %v", oldDeadline, dc.OldDeadline)
		}
	})

	t.Run("task deadline added", func(t *testing.T) {
		t.Parallel()
		newDeadline := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
		old := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: nil}
		new := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: &newDeadline}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		dc, ok := changes[0].(TaskDeadlineChanged)
		if !ok {
			t.Fatalf("expected TaskDeadlineChanged, got %T", changes[0])
		}
		if dc.OldDeadline != nil {
			t.Errorf("expected OldDeadline nil, got %v", dc.OldDeadline)
		}
	})

	t.Run("task deadline removed", func(t *testing.T) {
		t.Parallel()
		oldDeadline := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		old := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: &oldDeadline}
		new := &things.Task{UUID: "t1", Title: "Task", DeadlineDate: nil}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		dc, ok := changes[0].(TaskDeadlineChanged)
		if !ok {
			t.Fatalf("expected TaskDeadlineChanged, got %T", changes[0])
		}
		if dc.OldDeadline == nil || !dc.OldDeadline.Equal(oldDeadline) {
			t.Errorf("expected OldDeadline %v, got %v", oldDeadline, dc.OldDeadline)
		}
	})

	t.Run("multiple changes at once", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Old", Status: things.TaskStatusPending, InTrash: false}
		new := &things.Task{UUID: "t1", Title: "New", Status: things.TaskStatusCompleted, InTrash: true}
		changes := detectTaskChanges(old, new, 1, now)

		// Should have: title changed, completed, trashed
		if len(changes) != 3 {
			t.Fatalf("expected 3 changes, got %d: %v", len(changes), changes)
		}

		hasTitle, hasCompleted, hasTrashed := false, false, false
		for _, c := range changes {
			switch c.(type) {
			case TaskTitleChanged:
				hasTitle = true
			case TaskCompleted:
				hasCompleted = true
			case TaskTrashed:
				hasTrashed = true
			}
		}
		if !hasTitle {
			t.Error("expected TaskTitleChanged in changes")
		}
		if !hasCompleted {
			t.Error("expected TaskCompleted in changes")
		}
		if !hasTrashed {
			t.Error("expected TaskTrashed in changes")
		}
	})

	t.Run("tags changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", TagIDs: []string{"tag1", "tag2"}}
		new := &things.Task{UUID: "t1", Title: "Task", TagIDs: []string{"tag2", "tag3"}}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(TaskTagsChanged)
		if !ok {
			t.Fatalf("expected TaskTagsChanged, got %T", changes[0])
		}
		if len(tc.Added) != 1 || tc.Added[0] != "tag3" {
			t.Errorf("expected Added ['tag3'], got %v", tc.Added)
		}
		if len(tc.Removed) != 1 || tc.Removed[0] != "tag1" {
			t.Errorf("expected Removed ['tag1'], got %v", tc.Removed)
		}
	})

	t.Run("tags added", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", TagIDs: nil}
		new := &things.Task{UUID: "t1", Title: "Task", TagIDs: []string{"tag1", "tag2"}}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(TaskTagsChanged)
		if !ok {
			t.Fatalf("expected TaskTagsChanged, got %T", changes[0])
		}
		if len(tc.Added) != 2 {
			t.Errorf("expected 2 added tags, got %v", tc.Added)
		}
		if len(tc.Removed) != 0 {
			t.Errorf("expected 0 removed tags, got %v", tc.Removed)
		}
	})

	t.Run("tags removed", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", TagIDs: []string{"tag1", "tag2"}}
		new := &things.Task{UUID: "t1", Title: "Task", TagIDs: nil}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(TaskTagsChanged)
		if !ok {
			t.Fatalf("expected TaskTagsChanged, got %T", changes[0])
		}
		if len(tc.Added) != 0 {
			t.Errorf("expected 0 added tags, got %v", tc.Added)
		}
		if len(tc.Removed) != 2 {
			t.Errorf("expected 2 removed tags, got %v", tc.Removed)
		}
	})

	t.Run("no changes when identical", func(t *testing.T) {
		t.Parallel()
		old := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusPending}
		new := &things.Task{UUID: "t1", Title: "Task", Status: things.TaskStatusPending}
		changes := detectTaskChanges(old, new, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes, got %d: %v", len(changes), changes)
		}
	})

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		changes := detectTaskChanges(nil, nil, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes when both nil, got %d", len(changes))
		}
	})

	t.Run("change interface methods", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{UUID: "t1", Title: "Task"}
		changes := detectTaskChanges(nil, task, 42, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		c := changes[0]
		if c.ChangeType() != "TaskCreated" {
			t.Errorf("expected ChangeType 'TaskCreated', got %q", c.ChangeType())
		}
		if c.EntityType() != "Task" {
			t.Errorf("expected EntityType 'Task', got %q", c.EntityType())
		}
		if c.EntityUUID() != "t1" {
			t.Errorf("expected EntityUUID 't1', got %q", c.EntityUUID())
		}
		if c.ServerIndex() != 42 {
			t.Errorf("expected ServerIndex 42, got %d", c.ServerIndex())
		}
		if !c.Timestamp().Equal(now) {
			t.Errorf("expected Timestamp %v, got %v", now, c.Timestamp())
		}
	})
}

func TestDetectAreaChanges(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("area created", func(t *testing.T) {
		t.Parallel()
		area := &things.Area{UUID: "a1", Title: "Work"}
		changes := detectAreaChanges(nil, area, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		ac, ok := changes[0].(AreaCreated)
		if !ok {
			t.Errorf("expected AreaCreated, got %T", changes[0])
		}
		if ac.Area.UUID != "a1" {
			t.Errorf("expected Area UUID 'a1', got %q", ac.Area.UUID)
		}
	})

	t.Run("area deleted", func(t *testing.T) {
		t.Parallel()
		area := &things.Area{UUID: "a1", Title: "Work"}
		changes := detectAreaChanges(area, nil, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		ad, ok := changes[0].(AreaDeleted)
		if !ok {
			t.Errorf("expected AreaDeleted, got %T", changes[0])
		}
		if ad.Area.UUID != "a1" {
			t.Errorf("expected Area UUID 'a1', got %q", ad.Area.UUID)
		}
	})

	t.Run("area renamed", func(t *testing.T) {
		t.Parallel()
		old := &things.Area{UUID: "a1", Title: "Work"}
		new := &things.Area{UUID: "a1", Title: "Office"}
		changes := detectAreaChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		ar, ok := changes[0].(AreaRenamed)
		if !ok {
			t.Fatalf("expected AreaRenamed, got %T", changes[0])
		}
		if ar.OldTitle != "Work" {
			t.Errorf("expected OldTitle 'Work', got %q", ar.OldTitle)
		}
		if ar.Area.Title != "Office" {
			t.Errorf("expected new title 'Office', got %q", ar.Area.Title)
		}
	})

	t.Run("no changes when identical", func(t *testing.T) {
		t.Parallel()
		old := &things.Area{UUID: "a1", Title: "Work"}
		new := &things.Area{UUID: "a1", Title: "Work"}
		changes := detectAreaChanges(old, new, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes, got %d", len(changes))
		}
	})

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		changes := detectAreaChanges(nil, nil, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes when both nil, got %d", len(changes))
		}
	})

	t.Run("change interface methods", func(t *testing.T) {
		t.Parallel()
		area := &things.Area{UUID: "a1", Title: "Work"}
		changes := detectAreaChanges(nil, area, 99, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		c := changes[0]
		if c.ChangeType() != "AreaCreated" {
			t.Errorf("expected ChangeType 'AreaCreated', got %q", c.ChangeType())
		}
		if c.EntityType() != "Area" {
			t.Errorf("expected EntityType 'Area', got %q", c.EntityType())
		}
		if c.EntityUUID() != "a1" {
			t.Errorf("expected EntityUUID 'a1', got %q", c.EntityUUID())
		}
		if c.ServerIndex() != 99 {
			t.Errorf("expected ServerIndex 99, got %d", c.ServerIndex())
		}
	})
}

func TestDetectTagChanges(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("tag created", func(t *testing.T) {
		t.Parallel()
		tag := &things.Tag{UUID: "t1", Title: "Important"}
		changes := detectTagChanges(nil, tag, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(TagCreated)
		if !ok {
			t.Errorf("expected TagCreated, got %T", changes[0])
		}
		if tc.Tag.UUID != "t1" {
			t.Errorf("expected Tag UUID 't1', got %q", tc.Tag.UUID)
		}
	})

	t.Run("tag deleted", func(t *testing.T) {
		t.Parallel()
		tag := &things.Tag{UUID: "t1", Title: "Important"}
		changes := detectTagChanges(tag, nil, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		td, ok := changes[0].(TagDeleted)
		if !ok {
			t.Errorf("expected TagDeleted, got %T", changes[0])
		}
		if td.Tag.UUID != "t1" {
			t.Errorf("expected Tag UUID 't1', got %q", td.Tag.UUID)
		}
	})

	t.Run("tag renamed", func(t *testing.T) {
		t.Parallel()
		old := &things.Tag{UUID: "t1", Title: "Important"}
		new := &things.Tag{UUID: "t1", Title: "Critical"}
		changes := detectTagChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tr, ok := changes[0].(TagRenamed)
		if !ok {
			t.Fatalf("expected TagRenamed, got %T", changes[0])
		}
		if tr.OldTitle != "Important" {
			t.Errorf("expected OldTitle 'Important', got %q", tr.OldTitle)
		}
		if tr.Tag.Title != "Critical" {
			t.Errorf("expected new title 'Critical', got %q", tr.Tag.Title)
		}
	})

	t.Run("tag shortcut changed", func(t *testing.T) {
		t.Parallel()
		old := &things.Tag{UUID: "t1", Title: "Important", ShortHand: "i"}
		new := &things.Tag{UUID: "t1", Title: "Important", ShortHand: "!"}
		changes := detectTagChanges(old, new, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		sc, ok := changes[0].(TagShortcutChanged)
		if !ok {
			t.Fatalf("expected TagShortcutChanged, got %T", changes[0])
		}
		if sc.OldShortcut != "i" {
			t.Errorf("expected OldShortcut 'i', got %q", sc.OldShortcut)
		}
	})

	t.Run("multiple tag changes", func(t *testing.T) {
		t.Parallel()
		old := &things.Tag{UUID: "t1", Title: "Important", ShortHand: "i"}
		new := &things.Tag{UUID: "t1", Title: "Critical", ShortHand: "!"}
		changes := detectTagChanges(old, new, 1, now)

		if len(changes) != 2 {
			t.Fatalf("expected 2 changes, got %d", len(changes))
		}

		hasRenamed, hasShortcut := false, false
		for _, c := range changes {
			switch c.(type) {
			case TagRenamed:
				hasRenamed = true
			case TagShortcutChanged:
				hasShortcut = true
			}
		}
		if !hasRenamed {
			t.Error("expected TagRenamed in changes")
		}
		if !hasShortcut {
			t.Error("expected TagShortcutChanged in changes")
		}
	})

	t.Run("no changes when identical", func(t *testing.T) {
		t.Parallel()
		old := &things.Tag{UUID: "t1", Title: "Important", ShortHand: "i"}
		new := &things.Tag{UUID: "t1", Title: "Important", ShortHand: "i"}
		changes := detectTagChanges(old, new, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes, got %d", len(changes))
		}
	})

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		changes := detectTagChanges(nil, nil, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes when both nil, got %d", len(changes))
		}
	})

	t.Run("change interface methods", func(t *testing.T) {
		t.Parallel()
		tag := &things.Tag{UUID: "t1", Title: "Important"}
		changes := detectTagChanges(nil, tag, 77, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		c := changes[0]
		if c.ChangeType() != "TagCreated" {
			t.Errorf("expected ChangeType 'TagCreated', got %q", c.ChangeType())
		}
		if c.EntityType() != "Tag" {
			t.Errorf("expected EntityType 'Tag', got %q", c.EntityType())
		}
		if c.EntityUUID() != "t1" {
			t.Errorf("expected EntityUUID 't1', got %q", c.EntityUUID())
		}
		if c.ServerIndex() != 77 {
			t.Errorf("expected ServerIndex 77, got %d", c.ServerIndex())
		}
	})
}

func TestDetectChecklistChanges(t *testing.T) {
	t.Parallel()
	now := time.Now()
	parentTask := &things.Task{UUID: "task1", Title: "Parent Task"}

	t.Run("checklist item created", func(t *testing.T) {
		t.Parallel()
		item := &things.CheckListItem{UUID: "c1", Title: "Step 1"}
		changes := detectChecklistChanges(nil, item, parentTask, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		cc, ok := changes[0].(ChecklistItemCreated)
		if !ok {
			t.Errorf("expected ChecklistItemCreated, got %T", changes[0])
		}
		if cc.Item.UUID != "c1" {
			t.Errorf("expected Item UUID 'c1', got %q", cc.Item.UUID)
		}
		if cc.Task == nil || cc.Task.UUID != "task1" {
			t.Error("expected parent task to be set")
		}
	})

	t.Run("checklist item deleted", func(t *testing.T) {
		t.Parallel()
		item := &things.CheckListItem{UUID: "c1", Title: "Step 1"}
		changes := detectChecklistChanges(item, nil, parentTask, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		cd, ok := changes[0].(ChecklistItemDeleted)
		if !ok {
			t.Errorf("expected ChecklistItemDeleted, got %T", changes[0])
		}
		if cd.Item.UUID != "c1" {
			t.Errorf("expected Item UUID 'c1', got %q", cd.Item.UUID)
		}
	})

	t.Run("checklist item title changed", func(t *testing.T) {
		t.Parallel()
		old := &things.CheckListItem{UUID: "c1", Title: "Old Step"}
		new := &things.CheckListItem{UUID: "c1", Title: "New Step"}
		changes := detectChecklistChanges(old, new, parentTask, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		tc, ok := changes[0].(ChecklistItemTitleChanged)
		if !ok {
			t.Fatalf("expected ChecklistItemTitleChanged, got %T", changes[0])
		}
		if tc.OldTitle != "Old Step" {
			t.Errorf("expected OldTitle 'Old Step', got %q", tc.OldTitle)
		}
	})

	t.Run("checklist item completed", func(t *testing.T) {
		t.Parallel()
		old := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusPending}
		new := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusCompleted}
		changes := detectChecklistChanges(old, new, parentTask, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		cc, ok := changes[0].(ChecklistItemCompleted)
		if !ok {
			t.Errorf("expected ChecklistItemCompleted, got %T", changes[0])
		}
		if cc.Task == nil || cc.Task.UUID != "task1" {
			t.Error("expected parent task to be set")
		}
	})

	t.Run("checklist item uncompleted", func(t *testing.T) {
		t.Parallel()
		old := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusCompleted}
		new := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusPending}
		changes := detectChecklistChanges(old, new, parentTask, 1, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		cu, ok := changes[0].(ChecklistItemUncompleted)
		if !ok {
			t.Errorf("expected ChecklistItemUncompleted, got %T", changes[0])
		}
		if cu.Task == nil || cu.Task.UUID != "task1" {
			t.Error("expected parent task to be set")
		}
	})

	t.Run("multiple checklist changes", func(t *testing.T) {
		t.Parallel()
		old := &things.CheckListItem{UUID: "c1", Title: "Old Step", Status: things.TaskStatusPending}
		new := &things.CheckListItem{UUID: "c1", Title: "New Step", Status: things.TaskStatusCompleted}
		changes := detectChecklistChanges(old, new, parentTask, 1, now)

		if len(changes) != 2 {
			t.Fatalf("expected 2 changes, got %d", len(changes))
		}

		hasTitle, hasCompleted := false, false
		for _, c := range changes {
			switch c.(type) {
			case ChecklistItemTitleChanged:
				hasTitle = true
			case ChecklistItemCompleted:
				hasCompleted = true
			}
		}
		if !hasTitle {
			t.Error("expected ChecklistItemTitleChanged in changes")
		}
		if !hasCompleted {
			t.Error("expected ChecklistItemCompleted in changes")
		}
	})

	t.Run("no changes when identical", func(t *testing.T) {
		t.Parallel()
		old := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusPending}
		new := &things.CheckListItem{UUID: "c1", Title: "Step 1", Status: things.TaskStatusPending}
		changes := detectChecklistChanges(old, new, parentTask, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes, got %d", len(changes))
		}
	})

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		changes := detectChecklistChanges(nil, nil, parentTask, 1, now)

		if len(changes) != 0 {
			t.Fatalf("expected 0 changes when both nil, got %d", len(changes))
		}
	})

	t.Run("change interface methods", func(t *testing.T) {
		t.Parallel()
		item := &things.CheckListItem{UUID: "c1", Title: "Step 1"}
		changes := detectChecklistChanges(nil, item, parentTask, 55, now)

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		c := changes[0]
		if c.ChangeType() != "ChecklistItemCreated" {
			t.Errorf("expected ChangeType 'ChecklistItemCreated', got %q", c.ChangeType())
		}
		if c.EntityType() != "ChecklistItem" {
			t.Errorf("expected EntityType 'ChecklistItem', got %q", c.EntityType())
		}
		if c.EntityUUID() != "c1" {
			t.Errorf("expected EntityUUID 'c1', got %q", c.EntityUUID())
		}
		if c.ServerIndex() != 55 {
			t.Errorf("expected ServerIndex 55, got %d", c.ServerIndex())
		}
	})
}

func TestTaskLocation(t *testing.T) {
	t.Parallel()

	t.Run("nil task returns unknown", func(t *testing.T) {
		t.Parallel()
		loc := taskLocation(nil)
		if loc != LocationUnknown {
			t.Errorf("expected LocationUnknown, got %v", loc)
		}
	})

	t.Run("inbox schedule", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{Schedule: things.TaskScheduleInbox}
		loc := taskLocation(task)
		if loc != LocationInbox {
			t.Errorf("expected LocationInbox, got %v", loc)
		}
	})

	t.Run("anytime schedule without date", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{Schedule: things.TaskScheduleAnytime}
		loc := taskLocation(task)
		if loc != LocationAnytime {
			t.Errorf("expected LocationAnytime, got %v", loc)
		}
	})

	t.Run("anytime schedule with today date", func(t *testing.T) {
		t.Parallel()
		today := time.Now()
		task := &things.Task{Schedule: things.TaskScheduleAnytime, ScheduledDate: &today}
		loc := taskLocation(task)
		if loc != LocationToday {
			t.Errorf("expected LocationToday, got %v", loc)
		}
	})

	t.Run("someday schedule without date", func(t *testing.T) {
		t.Parallel()
		task := &things.Task{Schedule: things.TaskScheduleSomeday}
		loc := taskLocation(task)
		if loc != LocationSomeday {
			t.Errorf("expected LocationSomeday, got %v", loc)
		}
	})

	t.Run("someday schedule with future date", func(t *testing.T) {
		t.Parallel()
		futureDate := time.Now().AddDate(0, 0, 7)
		task := &things.Task{Schedule: things.TaskScheduleSomeday, ScheduledDate: &futureDate}
		loc := taskLocation(task)
		if loc != LocationUpcoming {
			t.Errorf("expected LocationUpcoming, got %v", loc)
		}
	})

	t.Run("someday schedule with past date", func(t *testing.T) {
		t.Parallel()
		pastDate := time.Now().AddDate(0, 0, -7)
		task := &things.Task{Schedule: things.TaskScheduleSomeday, ScheduledDate: &pastDate}
		loc := taskLocation(task)
		if loc != LocationSomeday {
			t.Errorf("expected LocationSomeday (past date), got %v", loc)
		}
	})
}

func TestTaskLocationString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		loc      TaskLocation
		expected string
	}{
		{LocationUnknown, "Unknown"},
		{LocationInbox, "Inbox"},
		{LocationToday, "Today"},
		{LocationAnytime, "Anytime"},
		{LocationSomeday, "Someday"},
		{LocationUpcoming, "Upcoming"},
		{LocationProject, "Project"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			if tc.loc.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.loc.String())
			}
		})
	}
}

func TestTimeEqual(t *testing.T) {
	t.Parallel()

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		if !timeEqual(nil, nil) {
			t.Error("expected nil == nil to be true")
		}
	})

	t.Run("first nil", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		if timeEqual(nil, &now) {
			t.Error("expected nil != time to be false")
		}
	})

	t.Run("second nil", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		if timeEqual(&now, nil) {
			t.Error("expected time != nil to be false")
		}
	})

	t.Run("equal times", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		same := now
		if !timeEqual(&now, &same) {
			t.Error("expected equal times to be true")
		}
	})

	t.Run("different times", func(t *testing.T) {
		t.Parallel()
		t1 := time.Now()
		t2 := t1.Add(time.Hour)
		if timeEqual(&t1, &t2) {
			t.Error("expected different times to be false")
		}
	})
}

func TestDiffStringSlices(t *testing.T) {
	t.Parallel()

	t.Run("empty slices", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices(nil, nil)
		if len(added) != 0 || len(removed) != 0 {
			t.Errorf("expected no changes, got added=%v removed=%v", added, removed)
		}
	})

	t.Run("items added", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices(nil, []string{"a", "b"})
		if len(added) != 2 || len(removed) != 0 {
			t.Errorf("expected 2 added, 0 removed, got added=%v removed=%v", added, removed)
		}
	})

	t.Run("items removed", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices([]string{"a", "b"}, nil)
		if len(added) != 0 || len(removed) != 2 {
			t.Errorf("expected 0 added, 2 removed, got added=%v removed=%v", added, removed)
		}
	})

	t.Run("mixed changes", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices([]string{"a", "b"}, []string{"b", "c"})
		if len(added) != 1 || added[0] != "c" {
			t.Errorf("expected added=['c'], got %v", added)
		}
		if len(removed) != 1 || removed[0] != "a" {
			t.Errorf("expected removed=['a'], got %v", removed)
		}
	})

	t.Run("no changes", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices([]string{"a", "b"}, []string{"a", "b"})
		if len(added) != 0 || len(removed) != 0 {
			t.Errorf("expected no changes, got added=%v removed=%v", added, removed)
		}
	})

	t.Run("different order same items", func(t *testing.T) {
		t.Parallel()
		added, removed := diffStringSlices([]string{"a", "b"}, []string{"b", "a"})
		if len(added) != 0 || len(removed) != 0 {
			t.Errorf("expected no changes for reordered items, got added=%v removed=%v", added, removed)
		}
	})
}

func TestIsToday(t *testing.T) {
	t.Parallel()

	t.Run("nil returns false", func(t *testing.T) {
		t.Parallel()
		if isToday(nil) {
			t.Error("expected nil to return false")
		}
	})

	t.Run("today returns true", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		if !isToday(&now) {
			t.Error("expected today to return true")
		}
	})

	t.Run("yesterday returns false", func(t *testing.T) {
		t.Parallel()
		yesterday := time.Now().AddDate(0, 0, -1)
		if isToday(&yesterday) {
			t.Error("expected yesterday to return false")
		}
	})

	t.Run("tomorrow returns false", func(t *testing.T) {
		t.Parallel()
		tomorrow := time.Now().AddDate(0, 0, 1)
		if isToday(&tomorrow) {
			t.Error("expected tomorrow to return false")
		}
	})
}
