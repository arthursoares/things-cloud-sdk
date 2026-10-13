package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func openTestSyncer(t *testing.T, client *things.Client) *Syncer {
	t.Helper()
	syncer, err := Open(filepath.Join(t.TempDir(), "test.db"), client)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { syncer.Close() })
	return syncer
}

// A tombstone's deletion instant is not stored, so an undecodable dld must not
// stop the deletion or the sync (arthursoares/things-cloud-sdk#37).
func TestProcessTombstone_UndecodableDeletionDateStillDeletes(t *testing.T) {
	t.Parallel()
	syncer := openTestSyncer(t, nil)
	mustSaveTask(t, syncer, &things.Task{UUID: "doomed", Title: "Doomed Task"})

	changes, err := syncer.processItems([]things.Item{{
		UUID: "tomb-1", Kind: things.ItemKindTombstone, Action: things.ItemActionCreated,
		P: json.RawMessage(`{"dloid":"doomed","dld":"invalid"}`),
	}}, 0)
	if err != nil {
		t.Fatalf("processItems failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if _, ok := changes[0].(TaskDeleted); !ok {
		t.Errorf("expected TaskDeleted, got %T", changes[0])
	}
	if task, _ := syncer.State().Task("doomed"); task != nil {
		t.Error("expected the task to be deleted")
	}
}

// A tombstone without a usable target deletes nothing: it is skipped and
// logged instead of failing its batch.
func TestProcessTombstone_WithoutTargetIsSkippedAndLogged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		reason  string
	}{
		{"empty object", `{}`, "tombstone names no deletion target (dloid)"},
		{"deletion date only", `{"dld":1}`, "tombstone names no deletion target (dloid)"},
		{"null target", `{"dloid":null,"dld":1}`, "tombstone names no deletion target (dloid)"},
		{"empty target", `{"dloid":"","dld":1}`, "tombstone names no deletion target (dloid)"},
		{"nonstring target", `{"dloid":42,"dld":1}`, "tombstone names no deletion target (dloid)"},
		{"null payload", `null`, "tombstone names no deletion target (dloid)"},
		{"array payload", `[]`, "tombstone payload is not a JSON object"},
		{"string payload", `"garbage"`, "tombstone payload is not a JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			syncer := openTestSyncer(t, nil)
			mustSaveTask(t, syncer, &things.Task{UUID: "bystander", Title: "Bystander"})

			changes, err := syncer.processItems([]things.Item{{
				UUID: "tomb-x", Kind: things.ItemKindTombstone, Action: things.ItemActionCreated,
				P: json.RawMessage(tc.payload),
			}}, 7)
			if err != nil {
				t.Fatalf("processItems failed: %v", err)
			}
			if len(changes) != 1 {
				t.Fatalf("expected 1 change, got %d", len(changes))
			}
			skipped, ok := changes[0].(ItemSkipped)
			if !ok {
				t.Fatalf("expected ItemSkipped, got %T", changes[0])
			}
			if skipped.Reason != tc.reason || skipped.EntityType() != "Tombstone2" || skipped.EntityUUID() != "tomb-x" || skipped.ServerIndex() != 7 {
				t.Errorf("ItemSkipped = %+v, want reason %q for Tombstone2 tomb-x at index 7", skipped, tc.reason)
			}

			logged, err := syncer.ChangesForEntity("tomb-x")
			if err != nil {
				t.Fatalf("ChangesForEntity failed: %v", err)
			}
			if len(logged) != 1 || logged[0].ChangeType() != "ItemSkipped" || logged[0].ServerIndex() != 7 {
				t.Errorf("change log = %+v, want one ItemSkipped row at index 7", logged)
			}
			if task, _ := syncer.State().Task("bystander"); task == nil {
				t.Error("a skipped tombstone deleted an unrelated task")
			}
		})
	}
}

// historyServer serves the given history slots, one per items page.
func historyServer(t *testing.T, slots ...string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/items") {
			fmt.Fprintf(w, `{"latest-server-index":%d,"latest-schema-version":301}`, len(slots))
			return
		}
		start, err := strconv.Atoi(r.URL.Query().Get("start-index"))
		if err != nil {
			http.Error(w, "start-index", http.StatusBadRequest)
			return
		}
		page := "[]"
		if start < len(slots) {
			page = "[" + slots[start] + "]"
		}
		fmt.Fprintf(w, `{"items":%s,"current-item-index":%d,"schema":301}`, page, len(slots))
	}))
	t.Cleanup(server.Close)
	return server
}

// A malformed tombstone used to roll back its batch and keep the cursor in
// front of it, so every later sync failed on the same item.
func TestSync_MalformedTombstoneDoesNotBlockTheHistory(t *testing.T) {
	t.Parallel()

	server := historyServer(t,
		`{"VJ1edXTP9q3PmFDUuy8EQh":{"e":"Task6","t":0,"p":{"tt":"deleted later","tp":0,"st":1}}}`,
		`{"FQxaqvLBkbR5q2Q5oRoknc":{"e":"Tombstone2","t":0,"p":{"dloid":"VJ1edXTP9q3PmFDUuy8EQh","dld":"invalid"}},`+
			`"KQ3c1TFEu6p7iBNbhbmyAr":{"e":"Tombstone2","t":0,"p":{}}}`,
		`{"3nBZa1y7m6XnTjp7HtCxDy":{"e":"Task6","t":0,"p":{"tt":"after the tombstones","tp":0,"st":1}}}`,
	)
	syncer := openTestSyncer(t, things.New(server.URL, "test@example.com", "password"))
	if err := syncer.saveSyncState("h1", 0); err != nil {
		t.Fatalf("saveSyncState failed: %v", err)
	}

	changes, err := syncer.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if got := syncer.LastSyncedIndex(); got != 3 {
		t.Errorf("cursor = %d, want 3", got)
	}
	if task, _ := syncer.State().Task("VJ1edXTP9q3PmFDUuy8EQh"); task != nil {
		t.Error("the tombstone with an undecodable dld did not delete its target")
	}
	if task, _ := syncer.State().Task("3nBZa1y7m6XnTjp7HtCxDy"); task == nil {
		t.Error("the task after the tombstones is missing")
	}
	var skipped int
	for _, c := range changes {
		if _, ok := c.(ItemSkipped); ok {
			skipped++
		}
	}
	if skipped != 1 {
		t.Errorf("ItemSkipped changes = %d, want 1 (the tombstone without dloid)", skipped)
	}
}

// Skipping stays limited to tombstones: an entity payload that does not decode
// still stops the sync before its batch, so no task data is dropped.
func TestSync_MalformedTaskStillStopsTheSync(t *testing.T) {
	t.Parallel()

	server := historyServer(t,
		`{"VJ1edXTP9q3PmFDUuy8EQh":{"e":"Tombstone2","t":0,"p":{}}}`,
		`{"FQxaqvLBkbR5q2Q5oRoknc":{"e":"Task6","t":0,"p":{"tt":5}}}`,
	)
	syncer := openTestSyncer(t, things.New(server.URL, "test@example.com", "password"))
	if err := syncer.saveSyncState("h1", 0); err != nil {
		t.Fatalf("saveSyncState failed: %v", err)
	}

	_, err := syncer.Sync()
	if err == nil || !strings.Contains(err.Error(), "processing item at server index 1") {
		t.Fatalf("Sync error = %v, want the malformed task at index 1", err)
	}
	if got := syncer.LastSyncedIndex(); got != 1 {
		t.Errorf("cursor = %d, want 1 (after the skipped tombstone, before the task)", got)
	}
}
