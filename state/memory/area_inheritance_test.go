package memory

import (
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Tasks inside an area-less project have no area — directly or inherited —
// and must appear in TasksWithoutArea. Tasks whose parent chain reaches an
// area inherit it and must not. This exercises hasArea's parent recursion,
// which was previously unreachable because parented tasks were skipped.
func TestTasksWithoutArea_ParentChainInheritance(t *testing.T) {
	state := NewState()
	state.Areas["area-1"] = &things.Area{UUID: "area-1", Title: "Work"}

	state.Tasks["proj-with-area"] = &things.Task{
		UUID: "proj-with-area", Title: "Homed project", AreaIDs: []string{"area-1"},
	}
	state.Tasks["child-of-homed"] = &things.Task{
		UUID: "child-of-homed", Title: "inherits area", ParentTaskIDs: []string{"proj-with-area"},
	}

	state.Tasks["proj-no-area"] = &things.Task{
		UUID: "proj-no-area", Title: "Homeless project",
	}
	state.Tasks["child-of-homeless"] = &things.Task{
		UUID: "child-of-homeless", Title: "no area anywhere", ParentTaskIDs: []string{"proj-no-area"},
	}

	got := map[string]bool{}
	for _, task := range state.TasksWithoutArea() {
		got[task.UUID] = true
	}

	if !got["proj-no-area"] {
		t.Error("proj-no-area missing from TasksWithoutArea")
	}
	if !got["child-of-homeless"] {
		t.Error("child-of-homeless missing from TasksWithoutArea — a task in an area-less project has no area")
	}
	if got["proj-with-area"] {
		t.Error("proj-with-area listed despite having an area")
	}
	if got["child-of-homed"] {
		t.Error("child-of-homed listed despite inheriting its parent's area")
	}
}
