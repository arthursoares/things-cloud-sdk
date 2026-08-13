package syncutil

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	"github.com/arthursoares/things-cloud-sdk/sync"
	_ "modernc.org/sqlite"
)

// fakeChange is a minimal sync.Change implementation used to drive the
// change-list helpers with fully controlled ChangeType and Timestamp values.
// The real sync.Change types embed unexported timestamp fields that cannot be
// set from outside the sync package, so a stub is required to exercise
// timestamp-dependent logic like DaysSinceCreated.
type fakeChange struct {
	changeType string
	timestamp  time.Time
}

func (f fakeChange) ChangeType() string   { return f.changeType }
func (f fakeChange) EntityType() string   { return "Task" }
func (f fakeChange) EntityUUID() string   { return "" }
func (f fakeChange) ServerIndex() int     { return 0 }
func (f fakeChange) Timestamp() time.Time { return f.timestamp }

// changes builds a []sync.Change from a list of change-type strings.
func changes(types ...string) []sync.Change {
	result := make([]sync.Change, 0, len(types))
	for _, t := range types {
		result = append(result, fakeChange{changeType: t})
	}
	return result
}

func TestFilterChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      []sync.Change
		changeType string
		want       int
	}{
		{"nil input", nil, "TaskCreated", 0},
		{"empty input", changes(), "TaskCreated", 0},
		{
			name:       "single match",
			input:      changes("TaskCreated", "TaskCompleted"),
			changeType: "TaskCreated",
			want:       1,
		},
		{
			name:       "multiple matches",
			input:      changes("TaskCreated", "TaskCreated", "TaskCompleted"),
			changeType: "TaskCreated",
			want:       2,
		},
		{
			name:       "no match",
			input:      changes("TaskCreated", "TaskCompleted"),
			changeType: "TaskDeleted",
			want:       0,
		},
		{
			name:       "exact match not prefix",
			input:      changes("TaskMovedToToday", "TaskMovedToInbox"),
			changeType: "TaskMovedTo",
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterChanges(tt.input, tt.changeType)
			if len(got) != tt.want {
				t.Errorf("FilterChanges() returned %d changes, want %d", len(got), tt.want)
			}
			for _, c := range got {
				if c.ChangeType() != tt.changeType {
					t.Errorf("FilterChanges() returned change of type %q, want %q", c.ChangeType(), tt.changeType)
				}
			}
		})
	}
}

func TestFilterChangesPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  []sync.Change
		prefix string
		want   int
	}{
		{"nil input", nil, "Task", 0},
		{"empty input", changes(), "Task", 0},
		{
			name:   "prefix matches several",
			input:  changes("TaskMovedToToday", "TaskMovedToInbox", "TaskCreated"),
			prefix: "TaskMovedTo",
			want:   2,
		},
		{
			name:   "prefix matches all",
			input:  changes("TaskCreated", "TaskCompleted"),
			prefix: "Task",
			want:   2,
		},
		{
			name:   "empty prefix matches all",
			input:  changes("TaskCreated", "AreaCreated"),
			prefix: "",
			want:   2,
		},
		{
			name:   "no match",
			input:  changes("AreaCreated", "TagCreated"),
			prefix: "Task",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterChangesPrefix(tt.input, tt.prefix)
			if len(got) != tt.want {
				t.Errorf("FilterChangesPrefix() returned %d changes, want %d", len(got), tt.want)
			}
		})
	}
}

func TestDaysSinceCreated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []sync.Change
		want  int
	}{
		{"nil input", nil, 0},
		{"empty input", changes(), 0},
		{
			name:  "no TaskCreated change",
			input: changes("TaskCompleted", "TaskMovedToToday"),
			want:  0,
		},
		{
			name: "created three days ago",
			input: []sync.Change{
				fakeChange{changeType: "TaskCreated", timestamp: time.Now().Add(-72 * time.Hour)},
			},
			want: 3,
		},
		{
			name: "created just now rounds to zero days",
			input: []sync.Change{
				fakeChange{changeType: "TaskCreated", timestamp: time.Now()},
			},
			want: 0,
		},
		{
			name: "uses first TaskCreated encountered",
			input: []sync.Change{
				fakeChange{changeType: "TaskCompleted", timestamp: time.Now()},
				fakeChange{changeType: "TaskCreated", timestamp: time.Now().Add(-48 * time.Hour)},
				fakeChange{changeType: "TaskCreated", timestamp: time.Now().Add(-240 * time.Hour)},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DaysSinceCreated(tt.input)
			if got != tt.want {
				t.Errorf("DaysSinceCreated() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountMoves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []sync.Change
		want  int
	}{
		{"nil input", nil, 0},
		{"empty input", changes(), 0},
		{
			name:  "no moves",
			input: changes("TaskCreated", "TaskCompleted"),
			want:  0,
		},
		{
			name:  "counts all TaskMovedTo variants",
			input: changes("TaskMovedToToday", "TaskMovedToInbox", "TaskMovedToUpcoming", "TaskMovedToSomeday", "TaskMovedToAnytime"),
			want:  5,
		},
		{
			name:  "ignores non-move changes",
			input: changes("TaskMovedToToday", "TaskCreated", "TaskMovedToInbox"),
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CountMoves(tt.input)
			if got != tt.want {
				t.Errorf("CountMoves() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTaskAge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task *things.Task
		want int
	}{
		{
			name: "zero creation date",
			task: &things.Task{},
			want: 0,
		},
		{
			name: "created two days ago",
			task: &things.Task{CreationDate: time.Now().Add(-48 * time.Hour)},
			want: 2,
		},
		{
			name: "created just now",
			task: &things.Task{CreationDate: time.Now()},
			want: 0,
		},
		{
			name: "created ten days ago",
			task: &things.Task{CreationDate: time.Now().Add(-240 * time.Hour)},
			want: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TaskAge(tt.task)
			if got != tt.want {
				t.Errorf("TaskAge() = %d, want %d", got, tt.want)
			}
		})
	}
}

// newTestSyncer opens a real SQLite-backed Syncer (migrating the schema) and
// returns it alongside a raw connection to the same database file so tests can
// seed the change_log directly.
func newTestSyncer(t *testing.T) (*sync.Syncer, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "sync.db")
	syncer, err := sync.Open(dbPath, nil)
	if err != nil {
		t.Fatalf("sync.Open() error: %v", err)
	}
	t.Cleanup(func() { syncer.Close() })

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}
	t.Cleanup(func() { raw.Close() })

	return syncer, raw
}

func insertChange(t *testing.T, db *sql.DB, changeType string, syncedAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO change_log (server_index, synced_at, change_type, entity_type, entity_uuid, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		0, syncedAt.Unix(), changeType, "Task", "uuid", nil,
	)
	if err != nil {
		t.Fatalf("insert change_log row: %v", err)
	}
}

func TestBuildDailySummary(t *testing.T) {
	t.Parallel()

	t.Run("empty change log", func(t *testing.T) {
		t.Parallel()
		syncer, _ := newTestSyncer(t)
		got := BuildDailySummary(syncer)
		if (got != DailySummary{}) {
			t.Errorf("BuildDailySummary() = %+v, want zero summary", got)
		}
	})

	t.Run("counts today's activity by category", func(t *testing.T) {
		t.Parallel()
		syncer, raw := newTestSyncer(t)

		// today's changes (synced_at is well after today's UTC midnight)
		now := time.Now()
		insertChange(t, raw, "TaskCompleted", now)
		insertChange(t, raw, "TaskCompleted", now)
		insertChange(t, raw, "TaskCreated", now)
		insertChange(t, raw, "TaskMovedToToday", now)
		insertChange(t, raw, "TaskMovedToUpcoming", now) // rescheduled
		insertChange(t, raw, "TaskMovedToSomeday", now)  // rescheduled
		insertChange(t, raw, "TaskMovedToInbox", now)    // rescheduled

		// a change from many days ago that must be excluded
		insertChange(t, raw, "TaskCreated", now.Add(-240*time.Hour))

		got := BuildDailySummary(syncer)
		want := DailySummary{
			Completed:    2,
			Created:      1,
			MovedToToday: 1,
			Rescheduled:  3,
		}
		if got != want {
			t.Errorf("BuildDailySummary() = %+v, want %+v", got, want)
		}
	})

	t.Run("TaskMovedToToday is not counted as rescheduled", func(t *testing.T) {
		t.Parallel()
		syncer, raw := newTestSyncer(t)
		insertChange(t, raw, "TaskMovedToToday", time.Now())

		got := BuildDailySummary(syncer)
		if got.Rescheduled != 0 {
			t.Errorf("Rescheduled = %d, want 0", got.Rescheduled)
		}
		if got.MovedToToday != 1 {
			t.Errorf("MovedToToday = %d, want 1", got.MovedToToday)
		}
	})
}
