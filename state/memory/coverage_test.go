package memory

import (
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// buildState returns a State populated with a small hierarchy used across the
// query-method tests below. Records are inserted directly into the maps to keep
// the fixtures explicit and independent of the Update() code path.
func buildState() *State {
	s := NewState()

	s.Areas["area-work"] = &things.Area{UUID: "area-work", Title: "Work"}
	s.Areas["area-home"] = &things.Area{UUID: "area-home", Title: "Home"}

	s.Tasks["proj-1"] = &things.Task{UUID: "proj-1", Title: "Launch", Type: things.TaskTypeProject, AreaIDs: []string{"area-work"}}
	s.Tasks["task-in-area"] = &things.Task{UUID: "task-in-area", Title: "In Area", Type: things.TaskTypeTask, AreaIDs: []string{"area-work"}, Index: 1}
	s.Tasks["task-done"] = &things.Task{UUID: "task-done", Title: "Done", Type: things.TaskTypeTask, AreaIDs: []string{"area-work"}, Status: things.TaskStatusCompleted, Index: 2}
	s.Tasks["task-trashed"] = &things.Task{UUID: "task-trashed", Title: "Trashed", Type: things.TaskTypeTask, AreaIDs: []string{"area-work"}, InTrash: true, Index: 3}

	// Subtasks under proj-1
	s.Tasks["sub-a"] = &things.Task{UUID: "sub-a", Title: "Sub A", Type: things.TaskTypeTask, ParentTaskIDs: []string{"proj-1"}, Index: 2}
	s.Tasks["sub-b"] = &things.Task{UUID: "sub-b", Title: "Sub B", Type: things.TaskTypeTask, ParentTaskIDs: []string{"proj-1"}, Index: 1}

	return s
}

func containsTask(tasks []*things.Task, uuid string) bool {
	for _, t := range tasks {
		if t.UUID == uuid {
			return true
		}
	}
	return false
}

func TestProjects(t *testing.T) {
	t.Parallel()
	s := buildState()
	projects := s.Projects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].UUID != "proj-1" {
		t.Errorf("expected proj-1, got %q", projects[0].UUID)
	}
}

func TestProjectByName(t *testing.T) {
	t.Parallel()
	s := buildState()

	if p := s.ProjectByName("Launch"); p == nil || p.UUID != "proj-1" {
		t.Errorf("expected to find project 'Launch', got %v", p)
	}
	if p := s.ProjectByName("Nonexistent"); p != nil {
		t.Errorf("expected nil for unknown project, got %v", p)
	}
	// A non-project task with a matching title must not be returned.
	if p := s.ProjectByName("In Area"); p != nil {
		t.Errorf("expected nil when title matches a non-project task, got %v", p)
	}
}

func TestAreaByName(t *testing.T) {
	t.Parallel()
	s := buildState()

	if a := s.AreaByName("Work"); a == nil || a.UUID != "area-work" {
		t.Errorf("expected to find area 'Work', got %v", a)
	}
	if a := s.AreaByName("Missing"); a != nil {
		t.Errorf("expected nil for unknown area, got %v", a)
	}
}

func TestSubtasks(t *testing.T) {
	t.Parallel()
	s := buildState()
	root := s.Tasks["proj-1"]

	subs := s.Subtasks(root, ListOption{})
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subs))
	}
	// Sorted by Index ascending: sub-b (1) before sub-a (2).
	if subs[0].UUID != "sub-b" || subs[1].UUID != "sub-a" {
		t.Errorf("expected sub-b then sub-a, got %q then %q", subs[0].UUID, subs[1].UUID)
	}
}

func TestSubtasksExcludesCompletedAndTrashed(t *testing.T) {
	t.Parallel()
	s := NewState()
	root := &things.Task{UUID: "root", Title: "Root"}
	s.Tasks["root"] = root
	s.Tasks["child-open"] = &things.Task{UUID: "child-open", ParentTaskIDs: []string{"root"}, Index: 1}
	s.Tasks["child-done"] = &things.Task{UUID: "child-done", ParentTaskIDs: []string{"root"}, Status: things.TaskStatusCompleted, Index: 2}
	s.Tasks["child-trash"] = &things.Task{UUID: "child-trash", ParentTaskIDs: []string{"root"}, InTrash: true, Index: 3}

	subs := s.Subtasks(root, ListOption{ExcludeCompleted: true, ExcludeInTrash: true})
	if len(subs) != 1 || subs[0].UUID != "child-open" {
		t.Errorf("expected only child-open, got %v", subs)
	}
}

func TestTasksByArea(t *testing.T) {
	t.Parallel()
	s := buildState()
	area := s.Areas["area-work"]

	all := s.TasksByArea(area, ListOption{})
	// proj-1, task-in-area, task-done, task-trashed all reference area-work.
	if len(all) != 4 {
		t.Fatalf("expected 4 tasks in area, got %d", len(all))
	}

	filtered := s.TasksByArea(area, ListOption{ExcludeCompleted: true, ExcludeInTrash: true})
	if containsTask(filtered, "task-done") {
		t.Error("completed task should be excluded")
	}
	if containsTask(filtered, "task-trashed") {
		t.Error("trashed task should be excluded")
	}
	if !containsTask(filtered, "task-in-area") {
		t.Error("open task should be included")
	}

	// An area with no tasks returns an empty slice.
	empty := s.TasksByArea(s.Areas["area-home"], ListOption{})
	if len(empty) != 0 {
		t.Errorf("expected 0 tasks for area-home, got %d", len(empty))
	}
}

func TestCheckListItemsByTask(t *testing.T) {
	t.Parallel()
	s := NewState()
	task := &things.Task{UUID: "task-1", Title: "Has checklist"}
	s.Tasks["task-1"] = task

	s.CheckListItems["c1"] = &things.CheckListItem{UUID: "c1", Title: "Step 2", TaskIDs: []string{"task-1"}, Index: 2}
	s.CheckListItems["c2"] = &things.CheckListItem{UUID: "c2", Title: "Step 1", TaskIDs: []string{"task-1"}, Index: 1}
	s.CheckListItems["c3"] = &things.CheckListItem{UUID: "c3", Title: "Done step", TaskIDs: []string{"task-1"}, Status: things.TaskStatusCompleted, Index: 3}
	s.CheckListItems["c4"] = &things.CheckListItem{UUID: "c4", Title: "Other", TaskIDs: []string{"other-task"}, Index: 0}

	all := s.CheckListItemsByTask(task, ListOption{})
	if len(all) != 3 {
		t.Fatalf("expected 3 checklist items, got %d", len(all))
	}
	// Sorted by Index: c2 (1), c1 (2), c3 (3).
	if all[0].UUID != "c2" || all[1].UUID != "c1" || all[2].UUID != "c3" {
		t.Errorf("unexpected order: %q %q %q", all[0].UUID, all[1].UUID, all[2].UUID)
	}

	open := s.CheckListItemsByTask(task, ListOption{ExcludeCompleted: true})
	for _, item := range open {
		if item.UUID == "c3" {
			t.Error("completed checklist item should be excluded")
		}
	}
	if len(open) != 2 {
		t.Errorf("expected 2 open items, got %d", len(open))
	}
}

func TestSubTags(t *testing.T) {
	t.Parallel()
	s := NewState()
	root := &things.Tag{UUID: "root-tag", Title: "Priority"}
	s.Tags["root-tag"] = root
	s.Tags["child-b"] = &things.Tag{UUID: "child-b", Title: "Low", ShortHand: "b", ParentTagIDs: []string{"root-tag"}}
	s.Tags["child-a"] = &things.Tag{UUID: "child-a", Title: "High", ShortHand: "a", ParentTagIDs: []string{"root-tag"}}
	s.Tags["unrelated"] = &things.Tag{UUID: "unrelated", Title: "Other", ShortHand: "z"}

	children := s.SubTags(root)
	if len(children) != 2 {
		t.Fatalf("expected 2 child tags, got %d", len(children))
	}
	// Sorted by ShortHand ascending: child-a ("a") before child-b ("b").
	if children[0].UUID != "child-a" || children[1].UUID != "child-b" {
		t.Errorf("expected child-a then child-b, got %q then %q", children[0].UUID, children[1].UUID)
	}
}

// TestHasAreaViaTasksWithoutArea exercises the recursive hasArea helper through
// the public TasksWithoutArea query: a top-level task with an area is excluded,
// while a top-level task with neither area nor parent is returned.
func TestHasAreaViaTasksWithoutArea(t *testing.T) {
	t.Parallel()
	s := NewState()
	s.Areas["area-1"] = &things.Area{UUID: "area-1", Title: "Work"}

	// Top-level task directly assigned to an area -> hasArea true -> excluded.
	s.Tasks["with-area"] = &things.Task{UUID: "with-area", Title: "With Area", AreaIDs: []string{"area-1"}, Index: 1}
	// Top-level task with no area and no parent -> hasArea false -> included.
	s.Tasks["loose"] = &things.Task{UUID: "loose", Title: "Loose", Index: 2}

	result := s.TasksWithoutArea()
	if !containsTask(result, "loose") {
		t.Error("expected loose task without area to be returned")
	}
	if containsTask(result, "with-area") {
		t.Error("task with an area should not be returned")
	}
}
