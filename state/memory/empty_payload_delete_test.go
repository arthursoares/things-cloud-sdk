package memory

import (
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Things writes deletes of areas, checklist items and tags with an empty
// payload (no "p", or null). They must still be applied.
func TestStateUpdateDeletesWithEmptyPayload(t *testing.T) {
	t.Parallel()

	const legacyArea = "E6CE6BA0-036D-4380-BE60-B2C073DFF21E"
	state := NewState()
	err := state.Update(
		things.Item{UUID: legacyArea, Kind: things.ItemKindAreaPlain, Action: things.ItemActionCreated, P: []byte(`{"tt":"Family","ix":-1998}`)},
		things.Item{UUID: "item", Kind: things.ItemKindChecklistItem3, Action: things.ItemActionCreated, P: []byte(`{"tt":"Step","ts":["task"]}`)},
		things.Item{UUID: "tag", Kind: things.ItemKindTag4, Action: things.ItemActionCreated, P: []byte(`{"tt":"Errand"}`)},
		things.Item{UUID: legacyArea, Kind: things.ItemKindAreaPlain, Action: things.ItemActionDeleted},
		things.Item{UUID: "item", Kind: things.ItemKindChecklistItem3, Action: things.ItemActionDeleted},
		things.Item{UUID: "tag", Kind: things.ItemKindTag4, Action: things.ItemActionDeleted, P: []byte(`null`)},
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(state.Areas) != 0 || len(state.CheckListItems) != 0 || len(state.Tags) != 0 {
		t.Fatalf("deleted objects remain: areas %v, checklist %v, tags %v", state.Areas, state.CheckListItems, state.Tags)
	}
}
