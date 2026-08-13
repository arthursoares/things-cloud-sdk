package sync

import (
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// The mustSave* helpers exist so fixture setup can stay a flat list of
// entities: the store writes are preconditions, not the assertion under test,
// and a failure there means the rest of the test proves nothing.

func mustSaveTask(t *testing.T, s *Syncer, task *things.Task) {
	t.Helper()
	if err := s.saveTask(task); err != nil {
		t.Fatalf("saveTask %s: %v", task.UUID, err)
	}
}

func mustSaveArea(t *testing.T, s *Syncer, area *things.Area) {
	t.Helper()
	if err := s.saveArea(area); err != nil {
		t.Fatalf("saveArea %s: %v", area.UUID, err)
	}
}

func mustSaveTag(t *testing.T, s *Syncer, tag *things.Tag) {
	t.Helper()
	if err := s.saveTag(tag); err != nil {
		t.Fatalf("saveTag %s: %v", tag.UUID, err)
	}
}

func mustSaveChecklistItem(t *testing.T, s *Syncer, item *things.CheckListItem) {
	t.Helper()
	if err := s.saveChecklistItem(item); err != nil {
		t.Fatalf("saveChecklistItem %s: %v", item.UUID, err)
	}
}
