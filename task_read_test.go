package thingscloud

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTaskReadPayloadNullsAndOmissions(t *testing.T) {
	date := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	alarm := 30
	task := Task{Title: "keep", Note: "clear", ScheduledDate: &date, DeadlineDate: &date,
		CompletionDate: &date, ModificationDate: &date, AlarmTimeOffset: &alarm,
		AreaIDs: []string{"area"}, ParentTaskIDs: []string{"project"}, ActionGroupIDs: []string{"heading"}, TagIDs: []string{"tag"}}
	omitted, err := DecodeTaskReadPayload([]byte(`{"tt":"unrelated","tir":null}`))
	if err != nil {
		t.Fatal(err)
	}
	omitted.ApplyNulls(&task)
	if task.ScheduledDate != &date || task.Note != "clear" || len(task.AreaIDs) != 1 {
		t.Fatal("omitted fields or null tir changed existing state")
	}
	cleared, err := DecodeTaskReadPayload([]byte(`{"sr":null,"dd":null,"sp":null,"md":null,"ato":null,"ar":null,"pr":null,"agr":null,"tg":null,"nt":null}`))
	if err != nil {
		t.Fatal(err)
	}
	cleared.ApplyNulls(&task)
	if task.ScheduledDate != nil || task.DeadlineDate != nil || task.CompletionDate != nil || task.ModificationDate != nil || task.AlarmTimeOffset != nil {
		t.Error("explicit null did not clear optional dates/alarm")
	}
	if len(task.AreaIDs)+len(task.ParentTaskIDs)+len(task.ActionGroupIDs)+len(task.TagIDs) != 0 || task.Note != "" || task.Title != "keep" {
		t.Error("explicit null did not clear relationships/notes or changed unrelated title")
	}
}

func TestTaskReadPayloadRejectsNonObjects(t *testing.T) {
	for _, raw := range []string{"", "null", "[]", `"text"`, `{"tt":[]}`, `{"nt":42}`, `{"nt":true}`, `{"nt":[]}`, `{"nt":{"t":"bad"}}`, `{"nt":{"t":3,"v":"private"}}`, `{"nt":{"t":2,"ps":"bad"}}`} {
		if _, err := DecodeTaskReadPayload([]byte(raw)); err == nil {
			t.Errorf("accepted invalid task payload %q", raw)
		}
	}
}

func TestTaskReadPayloadAcceptsKnownNoteFormats(t *testing.T) {
	for _, raw := range []string{`{"nt":null}`, `{"nt":"legacy text"}`, `{"nt":{"t":1,"v":""}}`, `{"nt":{"t":2,"ps":[{"p":0,"l":0,"r":"hello"}]}}`} {
		if _, err := DecodeTaskReadPayload([]byte(raw)); err != nil {
			t.Errorf("known note format rejected: %v", err)
		}
	}
}

func TestTaskReadKindsAndSanitizedError(t *testing.T) {
	if err := ValidateTaskReadKinds([]Item{{Kind: "Task6"}, {Kind: "Task7"}, {Kind: "Task4"}, {Kind: "Settings5"}}); err != nil {
		t.Fatal(err)
	}
	err := ValidateTaskReadKinds([]Item{{Kind: "Task8", UUID: "private-id", P: []byte(`{"tt":"private title"}`), ServerIndex: 42, HasServerIndex: true}})
	var unsupported *UnsupportedTaskKindError
	if !errors.As(err, &unsupported) || unsupported.Kind != "Task8" || unsupported.ServerIndex != 42 {
		t.Fatalf("expected indexed unsupported-task error, got %v", err)
	}
	if strings.Contains(err.Error(), "private") || !strings.Contains(err.Error(), "Task8") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
	err = ValidateTaskReadKinds([]Item{{Kind: "Task8\nprivate payload"}})
	if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("unsafe kind diagnostic: %v", err)
	}
}
