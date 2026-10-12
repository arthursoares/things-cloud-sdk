package sync

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Calendar dates as Things writes them: midnight UTC of the local day.
const (
	day1 = 1760140800 // 2025-10-11
	day2 = 1760227200 // 2025-10-12
)

func unixPtrEqual(got *time.Time, want int64) bool {
	return got != nil && got.Unix() == want
}

func TestTaskViewFieldsArePersisted(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index := 0
	apply := func(uuid, payload string) {
		t.Helper()
		if _, err := s.processItems([]things.Item{{UUID: uuid, Kind: things.ItemKindTask7, P: json.RawMessage(payload)}}, index); err != nil {
			t.Fatal(err)
		}
		index++
	}
	get := func(uuid string) *things.Task {
		t.Helper()
		task, err := s.getTask(uuid)
		if err != nil || task == nil {
			t.Fatalf("getTask(%s) = %+v, %v", uuid, task, err)
		}
		return task
	}

	// A to-do in This Evening, and one with an overdue deadline the user
	// dismissed from Today.
	apply("evening", `{"tt":"evening","tp":0,"st":1,"sr":1760140800,"tir":1760140800,"sb":1,"dds":null}`)
	apply("dismissed", `{"tt":"dismissed","tp":0,"st":1,"sr":null,"tir":null,"sb":0,"dd":1760054400,"dds":1760140800}`)

	evening := get("evening")
	if evening.StartBucket != 1 || !unixPtrEqual(evening.TodayIndexReferenceDate, day1) || evening.DeadlineSuppressionDate != nil {
		t.Fatalf("evening: sb %d, tir %v, dds %v", evening.StartBucket, evening.TodayIndexReferenceDate, evening.DeadlineSuppressionDate)
	}
	dismissed := get("dismissed")
	if dismissed.StartBucket != 0 || dismissed.TodayIndexReferenceDate != nil || !unixPtrEqual(dismissed.DeadlineSuppressionDate, day1) {
		t.Fatalf("dismissed: sb %d, tir %v, dds %v", dismissed.StartBucket, dismissed.TodayIndexReferenceDate, dismissed.DeadlineSuppressionDate)
	}

	// A sparse update without sb, dds or tir keeps all three.
	apply("evening", `{"tt":"renamed"}`)
	apply("dismissed", `{"tt":"renamed"}`)
	if got := get("evening"); got.StartBucket != 1 || !unixPtrEqual(got.TodayIndexReferenceDate, day1) {
		t.Fatalf("sparse update dropped sb or tir: sb %d, tir %v", got.StartBucket, got.TodayIndexReferenceDate)
	}
	if got := get("dismissed"); !unixPtrEqual(got.DeadlineSuppressionDate, day1) {
		t.Fatalf("sparse update dropped dds: %v", got.DeadlineSuppressionDate)
	}

	// New values replace the old ones; tir never moves the start date.
	apply("evening", `{"sb":0,"tir":1760227200}`)
	apply("dismissed", `{"dds":1760227200}`)
	if got := get("evening"); got.StartBucket != 0 || !unixPtrEqual(got.TodayIndexReferenceDate, day2) || !unixPtrEqual(got.ScheduledDate, day1) {
		t.Fatalf("evening after change: sb %d, tir %v, sr %v", got.StartBucket, got.TodayIndexReferenceDate, got.ScheduledDate)
	}
	if got := get("dismissed"); !unixPtrEqual(got.DeadlineSuppressionDate, day2) {
		t.Fatalf("dds change: %v", got.DeadlineSuppressionDate)
	}

	// Explicit nulls clear them, and a null tir leaves sr alone.
	apply("evening", `{"sb":1}`)
	apply("evening", `{"sb":null,"tir":null}`)
	apply("dismissed", `{"dds":null}`)
	if got := get("evening"); got.StartBucket != 0 || got.TodayIndexReferenceDate != nil || !unixPtrEqual(got.ScheduledDate, day1) {
		t.Fatalf("evening after nulls: sb %d, tir %v, sr %v", got.StartBucket, got.TodayIndexReferenceDate, got.ScheduledDate)
	}
	if got := get("dismissed"); got.DeadlineSuppressionDate != nil || !unixPtrEqual(got.DeadlineDate, 1760054400) {
		t.Fatalf("dismissed after null: dds %v, dd %v", got.DeadlineSuppressionDate, got.DeadlineDate)
	}
}

func taskColumns(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return columns
}

var viewFieldColumns = []string{"start_bucket", "deadline_suppression_date", "today_index_reference_date"}

// schema3Database returns the path of a database with the schema 3 tasks
// table: a fresh one with the migration 4 columns dropped.
func schema3Database(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{`UPDATE schema_version SET version = 3`}
	for _, column := range viewFieldColumns {
		stmts = append(stmts, `ALTER TABLE tasks DROP COLUMN `+column)
	}
	for _, stmt := range stmts {
		if _, err := s.rawDB.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMigration4AddsTaskViewColumns(t *testing.T) {
	path := schema3Database(t)
	s, err := Open(path, nil)
	if err != nil {
		t.Fatalf("reopen schema 3 database: %v", err)
	}
	defer s.Close()
	var version int
	if err := s.rawDB.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version = %d, %v; want %d", version, err, schemaVersion)
	}
	columns := taskColumns(t, s.rawDB)
	for _, column := range viewFieldColumns {
		if !columns[column] {
			t.Errorf("migration 4 did not add tasks.%s", column)
		}
	}
	if _, err := s.processItems([]things.Item{{UUID: "task", Kind: things.ItemKindTask7,
		P: json.RawMessage(`{"tt":"x","sb":1,"tir":1760140800,"dds":1760227200}`)}}, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.getTask("task")
	if err != nil || got == nil {
		t.Fatalf("getTask: %+v, %v", got, err)
	}
	if got.StartBucket != 1 || !unixPtrEqual(got.TodayIndexReferenceDate, day1) || !unixPtrEqual(got.DeadlineSuppressionDate, day2) {
		t.Fatalf("after migration: sb %d, tir %v, dds %v", got.StartBucket, got.TodayIndexReferenceDate, got.DeadlineSuppressionDate)
	}
}

func TestMigrationFailureKeepsOldSchema(t *testing.T) {
	path := schema3Database(t)
	// A column that migration 4 adds last already exists, so its final
	// statement fails after the first two succeeded.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN today_index_reference_date INTEGER`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if s, err := Open(path, nil); err == nil || !strings.Contains(err.Error(), "duplicate column") {
		if s != nil {
			_ = s.Close()
		}
		t.Fatalf("Open = %v, want the duplicate column error", err)
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("schema version = %d, %v; want 3", version, err)
	}
	columns := taskColumns(t, db)
	if columns["start_bucket"] || columns["deadline_suppression_date"] {
		t.Fatalf("failed migration left columns behind: %v", columns)
	}
}
