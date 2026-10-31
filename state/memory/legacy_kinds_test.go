package memory

import (
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Older histories contain Task2 tasks and Tag2 tags. They carry the same
// payload keys as the newer kinds and legacy identifiers.
func TestStateUpdateTask2AndTag2UseDerivedIdentifiers(t *testing.T) {
	t.Parallel()

	const (
		legacyTaskID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
		legacyTagID  = "44444444-5555-6666-7777-888888888888"
	)
	taskID := things.EncodeLegacyIdentifier(legacyTaskID)
	tagID := things.EncodeLegacyIdentifier(legacyTagID)

	state := NewState()
	err := state.Update(
		things.Item{UUID: legacyTagID, Kind: things.ItemKindTag2, Action: things.ItemActionCreated, P: []byte(`{"tt":"Errand"}`)},
		taskReadItem(legacyTaskID, things.ItemKindTask2, things.ItemActionCreated, `{"tt":"Old task","st":1,"tp":0,"tg":["`+legacyTagID+`"]}`),
		taskReadItem(taskID, things.ItemKindTask7, things.ItemActionModified, `{"tt":"Renamed"}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(state.Tasks) != 1 {
		t.Fatalf("got %d tasks, want the Task2 and Task7 events to share one task", len(state.Tasks))
	}
	task := requireTask(t, state, taskID)
	if task.Title != "Renamed" || len(task.TagIDs) != 1 || task.TagIDs[0] != tagID {
		t.Fatalf("task = title %q, tags %v; want Renamed with tag %s", task.Title, task.TagIDs, tagID)
	}
	if tag := state.Tags[tagID]; tag == nil || tag.Title != "Errand" {
		t.Fatalf("Tag2 not stored under derived identifier: %+v", state.Tags)
	}
}

func TestValidateTaskReadKindsAcceptsTask2(t *testing.T) {
	t.Parallel()

	items := []things.Item{{UUID: "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE", Kind: things.ItemKindTask2, Action: things.ItemActionCreated, P: []byte(`{"tt":"Old task"}`)}}
	if err := things.ValidateTaskReadKinds(items); err != nil {
		t.Fatalf("ValidateTaskReadKinds(Task2) = %v, want nil", err)
	}
}
