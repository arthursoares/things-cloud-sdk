package memory

import (
	"encoding/json"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func legacyPayload(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	return payload
}

func TestState_LegacyUUIDRemapping(t *testing.T) {
	t.Parallel()

	const (
		legacyTaskID       = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE-20240131"
		legacyAreaID       = "11111111-2222-3333-4444-555555555555"
		legacyProjectID    = "22222222-3333-4444-5555-666666666666"
		legacyHeadingID    = "33333333-4444-5555-6666-777777777777"
		legacyTagID        = "44444444-5555-6666-7777-888888888888"
		legacyParentTagID  = "55555555-6666-7777-8888-999999999999"
		legacyRecurrenceID = "66666666-7777-8888-9999-AAAAAAAAAAAA"
		legacyDelegateID   = "77777777-8888-9999-AAAA-BBBBBBBBBBBB"
		legacyChecklistID  = "88888888-9999-AAAA-BBBB-CCCCCCCCCCCC"
	)

	currentTaskID := things.EncodeLegacyIdentifier(legacyTaskID)
	unrelatedTaskID := things.EncodeLegacyIdentifier("99999999-AAAA-BBBB-CCCC-DDDDDDDDDDDD")
	state := NewState()
	err := state.Update(
		things.Item{
			UUID:   legacyAreaID,
			Kind:   things.ItemKindArea,
			Action: things.ItemActionCreated,
			P:      legacyPayload(t, map[string]any{"tt": "Context"}),
		},
		things.Item{
			UUID:   legacyTagID,
			Kind:   things.ItemKindTag,
			Action: things.ItemActionCreated,
			P: legacyPayload(t, map[string]any{
				"tt": "Tag",
				"pn": []string{legacyParentTagID},
			}),
		},
		things.Item{
			UUID:   legacyTaskID,
			Kind:   things.ItemKindTask3,
			Action: things.ItemActionCreated,
			P: legacyPayload(t, map[string]any{
				"tt":  "Repeated title",
				"ar":  []string{legacyAreaID},
				"pr":  []string{legacyProjectID},
				"agr": []string{legacyHeadingID},
				"tg":  []string{legacyTagID},
				"rt":  []string{legacyRecurrenceID},
				"dl":  []string{legacyDelegateID},
			}),
		},
		things.Item{
			UUID:   legacyTaskID,
			Kind:   things.ItemKindTask4,
			Action: things.ItemActionModified,
			P:      legacyPayload(t, map[string]any{"ss": things.TaskStatusCompleted}),
		},
		things.Item{
			UUID:   currentTaskID,
			Kind:   things.ItemKindTask,
			Action: things.ItemActionModified,
			P:      legacyPayload(t, map[string]any{"ar": []string{}}),
		},
		things.Item{
			UUID:   legacyChecklistID,
			Kind:   things.ItemKindChecklistItem2,
			Action: things.ItemActionCreated,
			P: legacyPayload(t, map[string]any{
				"tt": "Checklist entry",
				"ts": []string{legacyTaskID},
			}),
		},
		things.Item{
			UUID:   unrelatedTaskID,
			Kind:   things.ItemKindTask,
			Action: things.ItemActionCreated,
			P:      legacyPayload(t, map[string]any{"tt": "Repeated title"}),
		},
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if len(state.Tasks) != 2 {
		t.Fatalf("got %d tasks, want the remapped task and unrelated same-title task", len(state.Tasks))
	}
	task := state.Tasks[currentTaskID]
	if task == nil {
		t.Fatalf("remapped task %q not found", currentTaskID)
	}
	if task.Title != "Repeated title" || task.Status != things.TaskStatusCompleted {
		t.Errorf("remapped task content = title %q, status %v", task.Title, task.Status)
	}
	if len(task.AreaIDs) != 0 {
		t.Errorf("Task6 stub did not override legacy area IDs: %v", task.AreaIDs)
	}
	assertIDs(t, "parent tasks", task.ParentTaskIDs, things.EncodeLegacyIdentifier(legacyProjectID))
	assertIDs(t, "action groups", task.ActionGroupIDs, things.EncodeLegacyIdentifier(legacyHeadingID))
	assertIDs(t, "tags", task.TagIDs, things.EncodeLegacyIdentifier(legacyTagID))
	assertIDs(t, "recurrences", task.RecurrenceIDs, things.EncodeLegacyIdentifier(legacyRecurrenceID))
	assertIDs(t, "delegates", task.DelegateIDs, things.EncodeLegacyIdentifier(legacyDelegateID))
	if _, ok := state.Tasks[legacyTaskID]; ok {
		t.Errorf("legacy task key %q remains in state", legacyTaskID)
	}
	if other := state.Tasks[unrelatedTaskID]; other == nil || other.Title != task.Title {
		t.Errorf("unrelated same-title task was merged: %#v", other)
	}

	if state.Areas[things.EncodeLegacyIdentifier(legacyAreaID)] == nil {
		t.Errorf("legacy area was not stored under its migrated identifier")
	}
	tag := state.Tags[things.EncodeLegacyIdentifier(legacyTagID)]
	if tag == nil {
		t.Fatalf("legacy tag was not stored under its migrated identifier")
	}
	assertIDs(t, "parent tags", tag.ParentTagIDs, things.EncodeLegacyIdentifier(legacyParentTagID))
	checklist := state.CheckListItems[things.EncodeLegacyIdentifier(legacyChecklistID)]
	if checklist == nil {
		t.Fatalf("legacy checklist item was not stored under its migrated identifier")
	}
	assertIDs(t, "checklist tasks", checklist.TaskIDs, currentTaskID)
}

func TestState_LegacyDeletesCanonicalItem(t *testing.T) {
	t.Parallel()

	t.Run("item delete", func(t *testing.T) {
		const legacyID = "AAAAAAAA-1111-2222-3333-BBBBBBBBBBBB"
		state := NewState()
		if err := state.Update(
			things.Item{UUID: legacyID, Kind: things.ItemKindTask3, Action: things.ItemActionCreated, P: json.RawMessage(`{"tt":"Task"}`)},
			things.Item{UUID: legacyID, Kind: things.ItemKindTask4, Action: things.ItemActionDeleted, P: json.RawMessage(`{}`)},
		); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if _, ok := state.Tasks[things.EncodeLegacyIdentifier(legacyID)]; ok {
			t.Errorf("legacy delete left canonical task in state")
		}
	})

	t.Run("legacy tombstone", func(t *testing.T) {
		const legacyID = "CCCCCCCC-1111-2222-3333-DDDDDDDDDDDD"
		state := NewState()
		if err := state.Update(
			things.Item{UUID: legacyID, Kind: things.ItemKindChecklistItem2, Action: things.ItemActionCreated, P: json.RawMessage(`{"tt":"Entry"}`)},
			things.Item{
				UUID:   "EEEEEEEE-1111-2222-3333-FFFFFFFFFFFF",
				Kind:   things.ItemKindTombstonePlain,
				Action: things.ItemActionCreated,
				P:      legacyPayload(t, map[string]any{"dloid": legacyID, "dld": 1}),
			},
		); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if _, ok := state.CheckListItems[things.EncodeLegacyIdentifier(legacyID)]; ok {
			t.Errorf("legacy tombstone left canonical checklist item in state")
		}
	})
}

func TestState_LegacyKindWithCurrentIdentifierIsNotRederived(t *testing.T) {
	t.Parallel()

	// A legacy-kind item can carry an already-migrated Base58 identifier
	// (if any kind kept being emitted after its objects moved to current
	// identifiers). Deriving again would split the object across a phantom
	// key and the real one.
	currentID := things.EncodeLegacyIdentifier("AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE")
	currentTagID := things.EncodeLegacyIdentifier("11111111-2222-3333-4444-555555555555")
	state := NewState()
	if err := state.Update(
		things.Item{
			UUID:   currentID,
			Kind:   things.ItemKindTask4,
			Action: things.ItemActionCreated,
			P:      legacyPayload(t, map[string]any{"tt": "Carried over", "tg": []string{currentTagID}}),
		},
		things.Item{
			UUID:   currentID,
			Kind:   things.ItemKindTask,
			Action: things.ItemActionModified,
			P:      legacyPayload(t, map[string]any{"ss": things.TaskStatusCompleted}),
		},
	); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if len(state.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: legacy-kind item with current identifier must not fork", len(state.Tasks))
	}
	task := state.Tasks[currentID]
	if task == nil {
		t.Fatalf("task not found under its current identifier %q", currentID)
	}
	if task.Title != "Carried over" || task.Status != things.TaskStatusCompleted {
		t.Errorf("task = title %q, status %v; legacy and current events did not merge", task.Title, task.Status)
	}
	assertIDs(t, "tags", task.TagIDs, currentTagID)

	if err := state.Update(things.Item{
		UUID:   "FFFFFFFF-1111-2222-3333-444444444444",
		Kind:   things.ItemKindTombstonePlain,
		Action: things.ItemActionCreated,
		P:      legacyPayload(t, map[string]any{"dloid": currentID, "dld": 1}),
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if _, ok := state.Tasks[currentID]; ok {
		t.Errorf("legacy tombstone with current dloid did not delete the task")
	}
}

func TestState_AllLegacyKindsUseMigratedKeys(t *testing.T) {
	t.Parallel()

	const legacyID = "AAAAAAAA-2222-3333-4444-BBBBBBBBBBBB"
	currentID := things.EncodeLegacyIdentifier(legacyID)
	tests := []struct {
		name string
		kind things.ItemKind
		has  func(*State, string) bool
	}{
		{"Task4", things.ItemKindTask4, func(s *State, id string) bool { return s.Tasks[id] != nil }},
		{"Task3", things.ItemKindTask3, func(s *State, id string) bool { return s.Tasks[id] != nil }},
		{"Task", things.ItemKindTaskPlain, func(s *State, id string) bool { return s.Tasks[id] != nil }},
		{"ChecklistItem2", things.ItemKindChecklistItem2, func(s *State, id string) bool { return s.CheckListItems[id] != nil }},
		{"ChecklistItem", things.ItemKindChecklistItem, func(s *State, id string) bool { return s.CheckListItems[id] != nil }},
		{"Area2", things.ItemKindArea, func(s *State, id string) bool { return s.Areas[id] != nil }},
		{"Area", things.ItemKindAreaPlain, func(s *State, id string) bool { return s.Areas[id] != nil }},
		{"Tag3", things.ItemKindTag, func(s *State, id string) bool { return s.Tags[id] != nil }},
		{"Tag", things.ItemKindTagPlain, func(s *State, id string) bool { return s.Tags[id] != nil }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := NewState()
			if err := state.Update(things.Item{
				UUID:   legacyID,
				Kind:   test.kind,
				Action: things.ItemActionCreated,
				P:      json.RawMessage(`{}`),
			}); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			if !test.has(state, currentID) {
				t.Errorf("%s item not found under migrated key %q", test.kind, currentID)
			}
			if test.has(state, legacyID) {
				t.Errorf("%s item remains under legacy key %q", test.kind, legacyID)
			}
		})
	}
}

func assertIDs(t *testing.T, field string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", field, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %v, want %v", field, got, want)
			return
		}
	}
}
