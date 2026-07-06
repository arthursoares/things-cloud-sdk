package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// TestSync_CursorSurvivesMidSyncFailure: each successfully committed batch
// must persist its cursor. If a later batch fails, resuming must not replay
// the committed batches (replays double-apply note delta patches and
// duplicate change_log rows).
func TestSync_CursorSurvivesMidSyncFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/items") {
			switch r.URL.Query().Get("start-index") {
			case "0":
				// Batch 1: one valid task create.
				fmt.Fprint(w, `{"items":[{"VJ1edXTP9q3PmFDUuy8EQh":{"e":"Task6","t":0,"p":{"tt":"good task","tp":0,"st":1}}}],"current-item-index":2,"schema":301}`)
			case "1":
				// Batch 2: payload is a bare string — unmarshal into the
				// task payload struct fails, so this batch errors.
				fmt.Fprint(w, `{"items":[{"FQxaqvLBkbR5q2Q5oRoknc":{"e":"Task6","t":0,"p":"garbage"}}],"current-item-index":2,"schema":301}`)
			default:
				fmt.Fprint(w, `{"items":[],"current-item-index":2,"schema":301}`)
			}
			return
		}
		fmt.Fprint(w, `{"latest-server-index":2,"latest-schema-version":301}`)
	}))
	defer server.Close()

	client := things.New(server.URL, "test@example.com", "password")
	syncer, err := Open(filepath.Join(t.TempDir(), "test.db"), client)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	if err := syncer.saveSyncState("h1", 0); err != nil {
		t.Fatalf("saveSyncState failed: %v", err)
	}

	if _, err := syncer.Sync(); err == nil {
		t.Fatal("Sync: want error from corrupt batch 2, got nil")
	}

	if got := syncer.LastSyncedIndex(); got != 1 {
		t.Errorf("cursor after mid-sync failure = %d, want 1 (end of committed batch 1); a wrong cursor replays or skips items on the next sync", got)
	}
}

// TestGetTask_ExcludesSoftDeleted: getTask must filter deleted rows like
// every other entity, otherwise delete replays emit duplicate TaskDeleted
// events and State.Task returns tasks the user deleted.
func TestGetTask_ExcludesSoftDeleted(t *testing.T) {
	t.Parallel()

	syncer, err := Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	createPayload, _ := json.Marshal(map[string]any{"tt": "victim", "tp": 0, "st": 1})
	create := things.Item{UUID: "VJ1edXTP9q3PmFDUuy8EQh", Kind: things.ItemKindTask, Action: things.ItemActionCreated, P: createPayload}
	deleteItem := things.Item{UUID: "VJ1edXTP9q3PmFDUuy8EQh", Kind: things.ItemKindTask, Action: things.ItemActionDeleted, P: json.RawMessage(`{}`)}

	if _, err := syncer.processItems([]things.Item{create}, 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	changes, err := syncer.processItems([]things.Item{deleteItem}, 1)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := countByType(changes, "TaskDeleted"); n != 1 {
		t.Fatalf("first delete: %d TaskDeleted changes, want 1", n)
	}

	task, err := syncer.getTask("VJ1edXTP9q3PmFDUuy8EQh")
	if err != nil {
		t.Fatalf("getTask: %v", err)
	}
	if task != nil {
		t.Errorf("getTask returned soft-deleted task %q, want nil", task.Title)
	}

	// Replaying the same delete (e.g. after a crash) must not emit another
	// TaskDeleted event.
	changes, err = syncer.processItems([]things.Item{deleteItem}, 1)
	if err != nil {
		t.Fatalf("replayed delete: %v", err)
	}
	if n := countByType(changes, "TaskDeleted"); n != 0 {
		t.Errorf("replayed delete: %d TaskDeleted changes, want 0", n)
	}
}

func countByType(changes []Change, changeType string) int {
	n := 0
	for _, c := range changes {
		if c.ChangeType() == changeType {
			n++
		}
	}
	return n
}

// TestIsRetryableError: retry decisions must come from the HTTP status
// code, not substring-matching digits in arbitrary error text.
func TestIsRetryableError(t *testing.T) {
	t.Parallel()

	retryable := []error{
		&things.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"},
		&things.HTTPError{StatusCode: 502, Status: "502 Bad Gateway"},
		&things.HTTPError{StatusCode: 503, Status: "503 Service Unavailable"},
		&things.HTTPError{StatusCode: 504, Status: "504 Gateway Timeout"},
		fmt.Errorf("fetching items: %w", &things.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}),
	}
	for _, err := range retryable {
		if !isRetryableError(err) {
			t.Errorf("isRetryableError(%v) = false, want true", err)
		}
	}

	notRetryable := []error{
		nil,
		&things.HTTPError{StatusCode: 404, Status: "404 Not Found"},
		&things.HTTPError{StatusCode: 401, Status: "401 Unauthorized"},
		fmt.Errorf("request took 1504ms and was cancelled"), // digits are not a status
		fmt.Errorf("connect: connection refused on port 5001"),
	}
	for _, err := range notRetryable {
		if isRetryableError(err) {
			t.Errorf("isRetryableError(%v) = true, want false", err)
		}
	}
}
