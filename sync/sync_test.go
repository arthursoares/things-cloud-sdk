package sync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func TestSync_PaginatesFromStoredCursor(t *testing.T) {
	t.Parallel()

	const historyID = "test-history-id"

	page101 := `{"items":[{"task-page101":{"e":"Task6","t":0,"p":{"tt":"Page 101 Task","tp":0}}}],"current-item-index":102,"schema":301}`
	page102 := `{"items":[{"task-page102":{"e":"Task6","t":0,"p":{"tt":"Page 102 Task","tp":0}}}],"current-item-index":102,"schema":301}`
	historyMeta := `{"latest-server-index":102,"latest-schema-version":301,"is-empty":false,"latest-total-content-size":0}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/items") {
			switch r.URL.Query().Get("start-index") {
			case "100":
				fmt.Fprint(w, page101)
			case "101":
				fmt.Fprint(w, page102)
			default:
				fmt.Fprint(w, `{"items":[],"current-item-index":102,"schema":301}`)
			}
			return
		}
		fmt.Fprint(w, historyMeta)
	}))
	defer ts.Close()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	client := things.New(ts.URL, "test@example.com", "password")
	syncer, err := Open(dbPath, client)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	if err := syncer.saveSyncState(historyID, 100); err != nil {
		t.Fatalf("saveSyncState failed: %v", err)
	}

	changes, err := syncer.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(changes) != 2 {
		t.Errorf("changes = %d, want %d", len(changes), 2)
	}
	if got := syncer.LastSyncedIndex(); got != 102 {
		t.Errorf("LastSyncedIndex = %d, want %d", got, 102)
	}
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("creates new database", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "test.db")

		syncer, err := Open(dbPath, nil)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer syncer.Close()

		// Verify file was created
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Fatal("Database file was not created")
		}

		// Verify schema was applied by checking tables exist
		var tableName string
		err = syncer.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='tasks'").Scan(&tableName)
		if err != nil {
			t.Fatalf("tasks table not created: %v", err)
		}
	})

	t.Run("reopens existing database", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "test.db")

		// Create and close
		syncer1, err := Open(dbPath, nil)
		if err != nil {
			t.Fatalf("First Open failed: %v", err)
		}

		// Insert test data
		_, err = syncer1.db.Exec("INSERT INTO areas (uuid, title) VALUES ('test-uuid', 'Test Area')")
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		syncer1.Close()

		// Reopen
		syncer2, err := Open(dbPath, nil)
		if err != nil {
			t.Fatalf("Second Open failed: %v", err)
		}
		defer syncer2.Close()

		// Verify data persisted
		var title string
		err = syncer2.db.QueryRow("SELECT title FROM areas WHERE uuid = 'test-uuid'").Scan(&title)
		if err != nil {
			t.Fatalf("Data not persisted: %v", err)
		}
		if title != "Test Area" {
			t.Fatalf("Expected 'Test Area', got %q", title)
		}
	})
}
