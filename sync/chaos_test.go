package sync

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// outerItem is one entry in the synthetic history's outer array.
type outerItem struct {
	uuid    string
	kind    string
	action  int
	payload string
}

// buildSyntheticHistory returns a deterministic sequence of history items:
// task creates followed by modifies and completes on those same tasks, so
// replaying a committed batch would visibly duplicate change_log rows and
// re-drive state transitions.
func buildSyntheticHistory(nTasks int) []outerItem {
	ids := make([]string, nTasks)
	for i := range ids {
		ids[i] = things.NewUUID()
	}

	var items []outerItem
	for _, id := range ids {
		items = append(items, outerItem{
			uuid: id, kind: "Task6", action: 0,
			payload: `{"tt":"task ` + id[:6] + `","tp":0,"st":1,"ss":0}`,
		})
	}
	// Second pass: rename each task (modify) and complete every third one.
	for i, id := range ids {
		items = append(items, outerItem{
			uuid: id, kind: "Task6", action: 1,
			payload: `{"tt":"renamed ` + id[:6] + `"}`,
		})
		if i%3 == 0 {
			items = append(items, outerItem{
				uuid: id, kind: "Task6", action: 1,
				payload: `{"ss":3}`,
			})
		}
	}
	return items
}

// chaosServer serves a synthetic history in fixed-size pages and can be
// told to fail exactly once at a chosen start-index, simulating a server
// error or a truncated response mid-sync.
type chaosServer struct {
	items     []outerItem
	batchSize int
	failAt    int    // start-index to fail at; -1 disables
	failMode  string // "500" or "truncate"
}

func (cs *chaosServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/items") {
			fmt.Fprintf(w, `{"latest-server-index":%d,"latest-schema-version":301}`, len(cs.items))
			return
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("start-index"))
		if cs.failAt >= 0 && start == cs.failAt {
			switch cs.failMode {
			case "500":
				w.WriteHeader(http.StatusInternalServerError)
				return
			case "truncate":
				fmt.Fprint(w, `{"items":[{"broken":`) // valid header, truncated body
				return
			}
		}

		end := start + cs.batchSize
		if end > len(cs.items) {
			end = len(cs.items)
		}
		var b strings.Builder
		b.WriteString(`{"items":[`)
		for i := start; i < end; i++ {
			if i > start {
				b.WriteByte(',')
			}
			it := cs.items[i]
			fmt.Fprintf(&b, `{%q:{"e":%q,"t":%d,"p":%s}}`, it.uuid, it.kind, it.action, it.payload)
		}
		fmt.Fprintf(&b, `],"current-item-index":%d,"schema":301}`, len(cs.items))
		fmt.Fprint(w, b.String())
	}
}

func openSyncerFor(t *testing.T, cs *chaosServer, dbName string) (*Syncer, string, func()) {
	server := httptest.NewServer(cs.handler(t))
	client := things.New(server.URL, "test@example.com", "password")
	dbPath := filepath.Join(t.TempDir(), dbName)
	syncer, err := Open(dbPath, client)
	if err != nil {
		server.Close()
		t.Fatalf("Open: %v", err)
	}
	if err := syncer.saveSyncState("chaos-history", 0); err != nil {
		server.Close()
		syncer.Close()
		t.Fatalf("saveSyncState: %v", err)
	}
	return syncer, dbPath, func() { syncer.Close(); server.Close() }
}

// changeLogFingerprint returns the sorted set of (server_index, entity_uuid,
// change_type) tuples and flags any duplicate. Duplicates are the signature
// of a replayed committed batch — exactly what the atomic-cursor fix prevents.
func changeLogFingerprint(t *testing.T, dbPath string) (fingerprint string, duplicates int) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db for inspection: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT server_index, entity_uuid, change_type, COUNT(*)
		FROM change_log GROUP BY server_index, entity_uuid, change_type
		ORDER BY server_index, entity_uuid, change_type`)
	if err != nil {
		t.Fatalf("query change_log: %v", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var idx, count int
		var uuid, ctype string
		if err := rows.Scan(&idx, &uuid, &ctype, &count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if count > 1 {
			duplicates += count - 1
		}
		lines = append(lines, fmt.Sprintf("%d|%s|%s", idx, uuid, ctype))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate change_log: %v", err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), duplicates
}

func taskStateFingerprint(t *testing.T, dbPath string) string {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT uuid, title, status, deleted FROM tasks ORDER BY uuid`)
	if err != nil {
		t.Fatalf("query tasks: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var uuid, title string
		var status, deleted int
		if err := rows.Scan(&uuid, &title, &status, &deleted); err != nil {
			t.Fatalf("scan: %v", err)
		}
		lines = append(lines, fmt.Sprintf("%s|%s|%d|%d", uuid, title, status, deleted))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tasks: %v", err)
	}
	return strings.Join(lines, "\n")
}

// TestChaos_MidSyncFailuresNeverCorrupt runs many trials, each injecting a
// failure at a random batch boundary, then recovering. After recovery the
// database must be byte-for-byte equivalent to a clean single-shot sync:
// no duplicated change_log rows, identical final task state.
func TestChaos_MidSyncFailuresNeverCorrupt(t *testing.T) {
	t.Parallel()

	history := buildSyntheticHistory(40) // ~93 items
	batchSize := 5

	// Reference: a clean sync with no faults.
	refCS := &chaosServer{items: history, batchSize: batchSize, failAt: -1}
	refSyncer, refDBPath, refClose := openSyncerFor(t, refCS, "ref.db")
	if _, err := refSyncer.Sync(); err != nil {
		refClose()
		t.Fatalf("reference sync: %v", err)
	}
	refChanges, refDups := changeLogFingerprint(t, refDBPath)
	refState := taskStateFingerprint(t, refDBPath)
	if refDups != 0 {
		refClose()
		t.Fatalf("reference sync itself has %d duplicate change_log rows", refDups)
	}
	if refChanges == "" {
		refClose()
		t.Fatal("reference sync produced no changes — history did not apply")
	}
	refClose()

	rng := rand.New(rand.NewSource(1))

	for trial := 0; trial < 40; trial++ {
		failAt := (rng.Intn(len(history)/batchSize-1) + 1) * batchSize // a non-zero batch boundary
		// "truncate" is a non-retryable decode error that fails instantly,
		// so the bulk of trials stay fast. One trial exercises the
		// retry-then-exhaust path via a 500, which pays the real retry
		// backoff (~14s) — skip it under -short.
		mode := "truncate"
		if trial == 0 && !testing.Short() {
			mode = "500"
		}

		cs := &chaosServer{items: history, batchSize: batchSize, failAt: failAt, failMode: mode}
		syncer, dbPath, closeFn := openSyncerFor(t, cs, fmt.Sprintf("trial%d.db", trial))

		// First sync fails partway through.
		if _, err := syncer.Sync(); err == nil {
			closeFn()
			t.Fatalf("trial %d (%s@%d): expected failure, got nil", trial, mode, failAt)
		}
		// Cursor must sit exactly at the last committed batch, never past
		// the failed one.
		if got := syncer.LastSyncedIndex(); got != failAt {
			closeFn()
			t.Fatalf("trial %d (%s@%d): cursor = %d, want %d (last committed batch)", trial, mode, failAt, got, failAt)
		}

		// Clear the fault and drive to completion.
		cs.failAt = -1
		if _, err := syncer.Sync(); err != nil {
			closeFn()
			t.Fatalf("trial %d recovery sync: %v", trial, err)
		}

		gotChanges, gotDups := changeLogFingerprint(t, dbPath)
		gotState := taskStateFingerprint(t, dbPath)
		closeFn()

		if gotDups != 0 {
			t.Errorf("trial %d (%s@%d): %d duplicate change_log rows after recovery", trial, mode, failAt, gotDups)
		}
		if gotChanges != refChanges {
			t.Errorf("trial %d (%s@%d): change_log differs from clean sync", trial, mode, failAt)
		}
		if gotState != refState {
			t.Errorf("trial %d (%s@%d): final task state differs from clean sync", trial, mode, failAt)
		}
	}
}
