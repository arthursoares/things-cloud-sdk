package sync

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Repeat rules as Things 3.24 writes them on a template: every week on a fixed
// schedule (tp 0) and every month after completion (tp 1).
const (
	weeklyRule  = `{"fu":256,"fa":1,"of":[{"wd":1}],"tp":0,"ia":1760140800,"sr":1760140800,"ed":64092211200,"rc":0,"rrv":4,"ts":0}`
	monthlyRule = `{"fu":8,"fa":1,"of":[{"dy":11}],"tp":1,"ia":1760140800,"sr":1760140800,"ed":64092211200,"rc":0,"rrv":4,"ts":0}`
)

func TestRecurrenceFieldsArePersisted(t *testing.T) {
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

	apply("template", `{"tt":"repeat weekly","tp":0,"st":1,"rr":`+weeklyRule+`,"rt":[]}`)
	apply("instance", `{"tt":"repeat weekly","tp":0,"st":1,"sr":1760140800,"rt":["template"]}`)

	tmpl := get("template")
	if tmpl.Repeater == nil || tmpl.Repeater.FrequencyUnit != things.FrequencyUnitWeekly || tmpl.Repeater.FrequencyAmplitude != 1 || tmpl.Repeater.Type != 0 {
		t.Fatalf("template repeat rule = %+v", tmpl.Repeater)
	}
	if !tmpl.IsRepeatingTemplate() || tmpl.IsRepeatingInstance() {
		t.Errorf("template: IsRepeatingTemplate %v, IsRepeatingInstance %v", tmpl.IsRepeatingTemplate(), tmpl.IsRepeatingInstance())
	}
	inst := get("instance")
	if !reflect.DeepEqual(inst.RecurrenceIDs, []string{"template"}) || inst.Repeater != nil {
		t.Fatalf("instance rt = %v, rr = %+v", inst.RecurrenceIDs, inst.Repeater)
	}
	if inst.IsRepeatingTemplate() || !inst.IsRepeatingInstance() {
		t.Errorf("instance: IsRepeatingTemplate %v, IsRepeatingInstance %v", inst.IsRepeatingTemplate(), inst.IsRepeatingInstance())
	}

	// A sparse update without rr or rt keeps both.
	apply("template", `{"tt":"renamed"}`)
	apply("instance", `{"tt":"renamed"}`)
	if got := get("template"); got.Repeater == nil || got.Repeater.FrequencyUnit != things.FrequencyUnitWeekly {
		t.Fatalf("sparse update dropped rr: %+v", got.Repeater)
	}
	if got := get("instance"); !reflect.DeepEqual(got.RecurrenceIDs, []string{"template"}) {
		t.Fatalf("sparse update dropped rt: %v", got.RecurrenceIDs)
	}

	// A new rule replaces the old one.
	apply("template", `{"rr":`+monthlyRule+`}`)
	if got := get("template"); got.Repeater == nil || got.Repeater.FrequencyUnit != things.FrequencyUnitMonthly || got.Repeater.Type != 1 {
		t.Fatalf("rule change: %+v", got.Repeater)
	}

	// Explicit nulls clear them.
	apply("template", `{"rr":null}`)
	apply("instance", `{"rt":null}`)
	if got := get("template"); got.Repeater != nil || got.IsRepeatingTemplate() {
		t.Fatalf("rr null not cleared: %+v", got.Repeater)
	}
	if got := get("instance"); got.RecurrenceIDs != nil || got.IsRepeatingInstance() {
		t.Fatalf("rt null not cleared: %v", got.RecurrenceIDs)
	}
}

func TestMigration3AddsRecurrenceIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Turn the fresh database into a schema 2 one.
	for _, stmt := range []string{
		`ALTER TABLE tasks DROP COLUMN recurrence_ids`,
		`UPDATE schema_version SET version = 2`,
	} {
		if _, err := s.rawDB.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path, nil)
	if err != nil {
		t.Fatalf("reopen schema 2 database: %v", err)
	}
	defer s.Close()
	var version int
	if err := s.rawDB.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version = %d, %v; want %d", version, err, schemaVersion)
	}
	var found bool
	rows, err := s.rawDB.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		found = found || name == "recurrence_ids"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("migration 3 did not add tasks.recurrence_ids")
	}
	if _, err := s.processItems([]things.Item{{UUID: "instance", Kind: things.ItemKindTask7, P: json.RawMessage(`{"tt":"x","rt":["template"]}`)}}, 0); err != nil {
		t.Fatal(err)
	}
	if got, err := s.getTask("instance"); err != nil || got == nil || !reflect.DeepEqual(got.RecurrenceIDs, []string{"template"}) {
		t.Fatalf("after migration: %+v, %v", got, err)
	}
}
