package syncutil

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/arthursoares/things-cloud-sdk/sync"
	_ "modernc.org/sqlite"
)

// BuildDailySummary's "today" must start at LOCAL midnight, not UTC
// midnight. Fixed scenario: now = 01:00 on Jul 6 in UTC+10.
//   - local midnight = Jul 5 14:00 UTC
//   - UTC truncation would give Jul 5 00:00 UTC
//
// A change at Jul 5 10:00 UTC (= Jul 5 20:00 local — YESTERDAY evening)
// sits between the two cutoffs: the buggy UTC truncation counts it,
// local-midnight semantics must not.
func TestBuildDailySummary_LocalMidnightBoundary(t *testing.T) {
	zone := time.FixedZone("UTC+10", 10*3600)
	now := time.Date(2026, 7, 6, 1, 0, 0, 0, zone)
	orig := timeNow
	timeNow = func() time.Time { return now }
	defer func() { timeNow = orig }()

	dbPath := filepath.Join(t.TempDir(), "tz.db")
	syncer, err := sync.Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer syncer.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	insert := func(syncedAt time.Time, changeType string) {
		if _, err := db.Exec(`INSERT INTO change_log (server_index, synced_at, change_type, entity_type, entity_uuid, payload)
			VALUES (1, ?, ?, 'Task', 'u1', '{}')`, syncedAt.Unix(), changeType); err != nil {
			t.Fatal(err)
		}
	}

	yesterdayLocalEvening := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC) // Jul 5 20:00 local
	todayLocalMorning := time.Date(2026, 7, 5, 14, 30, 0, 0, time.UTC)    // Jul 6 00:30 local
	insert(yesterdayLocalEvening, "TaskCompleted")
	insert(todayLocalMorning, "TaskCompleted")

	summary := BuildDailySummary(syncer)
	if summary.Completed != 1 {
		t.Errorf("Completed = %d, want 1 — yesterday's local-evening change must not count toward today", summary.Completed)
	}
}

func TestStartOfToday_LocalMidnight(t *testing.T) {
	zone := time.FixedZone("UTC+10", 10*3600)
	orig := timeNow
	timeNow = func() time.Time { return time.Date(2026, 7, 6, 1, 0, 0, 0, zone) }
	defer func() { timeNow = orig }()

	got := StartOfToday()
	want := time.Date(2026, 7, 6, 0, 0, 0, 0, zone)
	if !got.Equal(want) {
		t.Errorf("StartOfToday() = %v, want %v (local midnight)", got, want)
	}
}
