package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

const task7Kind things.ItemKind = "Task7"

func taskReadItem(id string, kind things.ItemKind, action things.ItemAction, payload string) things.Item {
	return things.Item{UUID: id, Kind: kind, Action: action, P: json.RawMessage(payload)}
}

func requireTask(t *testing.T, state *State, id string) *things.Task {
	t.Helper()
	task := state.Tasks[id]
	if task == nil {
		t.Fatalf("task %q not found", id)
	}
	return task
}

func TestStateUpdateTask7CreateAndModify(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("task-7", task7Kind, things.ItemActionCreated, `{"tt":"","st":0,"tp":0}`),
		taskReadItem("task-7", task7Kind, things.ItemActionModified, `{"tt":"New task"}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(state.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(state.Tasks))
	}
	task := requireTask(t, state, "task-7")
	if task.Title != "New task" || task.Schedule != things.TaskScheduleInbox {
		t.Fatalf("task = title %q, schedule %v; want New task in Inbox", task.Title, task.Schedule)
	}
}

func TestStateUpdateTask6AndTask7ShareIdentity(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("same-task", things.ItemKindTask, things.ItemActionCreated, `{"tt":"Task6 title","ss":0}`),
		taskReadItem("same-task", task7Kind, things.ItemActionModified, `{"tt":"Task7 title","ss":3}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(state.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(state.Tasks))
	}
	task := requireTask(t, state, "same-task")
	if task.Title != "Task7 title" || task.Status != things.TaskStatusCompleted {
		t.Fatalf("mixed-kind task = title %q, status %v", task.Title, task.Status)
	}
}

func TestStateUpdateTask7Notes(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("notes", task7Kind, things.ItemActionCreated, `{"nt":{"_t":"Note","t":1,"v":"hello world"}}`),
		taskReadItem("notes", task7Kind, things.ItemActionModified, `{"nt":{"_t":"Note","t":2,"ps":[{"p":6,"l":5,"r":"Things","ch":0}]}}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := requireTask(t, state, "notes").Note; got != "hello Things" {
		t.Fatalf("note = %q, want %q", got, "hello Things")
	}
}

func TestStateUpdateTask7TitleEmptyAndOmitted(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("title", task7Kind, things.ItemActionCreated, `{"tt":"original"}`),
		taskReadItem("title", task7Kind, things.ItemActionModified, `{"ss":3}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := requireTask(t, state, "title").Title; got != "original" {
		t.Fatalf("title = %q after omitted update, want original", got)
	}
	if err := state.Update(
		taskReadItem("title", task7Kind, things.ItemActionModified, `{"tt":""}`),
		taskReadItem("title", task7Kind, things.ItemActionModified, `{"ss":0}`),
	); err != nil {
		t.Fatalf("empty title update: %v", err)
	}
	if got := requireTask(t, state, "title").Title; got != "" {
		t.Fatalf("title = %q, want explicit empty title preserved across omitted update", got)
	}
}

func TestStateUpdateTask7NullableFields(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("nullable", task7Kind, things.ItemActionCreated, `{
			"md":1700000000,"sr":1700000100,"sp":1700000200,"dd":1700000300,"ato":15,
			"ar":["area"],"pr":["project"],"agr":["heading"],"tg":["tag"],"rt":["repeat"],"dl":["delegate"]
		}`),
		taskReadItem("nullable", task7Kind, things.ItemActionModified, `{
			"md":null,"sr":null,"sp":null,"dd":null,"ato":null,
			"ar":null,"pr":[],"agr":null,"tg":[],"rt":null,"dl":[]
		}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	task := requireTask(t, state, "nullable")
	if task.ModificationDate != nil || task.ScheduledDate != nil || task.CompletionDate != nil || task.DeadlineDate != nil || task.AlarmTimeOffset != nil {
		t.Fatalf("explicit null did not clear dates/alarm: %+v", task)
	}
	if task.AreaIDs != nil || task.ActionGroupIDs != nil || task.RecurrenceIDs != nil {
		t.Fatalf("null relationships were not cleared to nil: ar=%v agr=%v rt=%v", task.AreaIDs, task.ActionGroupIDs, task.RecurrenceIDs)
	}
	if len(task.ParentTaskIDs) != 0 || len(task.TagIDs) != 0 || len(task.DelegateIDs) != 0 {
		t.Fatalf("empty relationships were not cleared: pr=%v tg=%v dl=%v", task.ParentTaskIDs, task.TagIDs, task.DelegateIDs)
	}
}

func TestStateUpdateTask7ScheduleStatusTrashAndDeletion(t *testing.T) {
	t.Parallel()

	for _, payload := range []json.RawMessage{nil, json.RawMessage(`null`)} {
		t.Run(string(payload), func(t *testing.T) {
			t.Parallel()
			state := NewState()
			if err := state.Update(taskReadItem("lifecycle", task7Kind, things.ItemActionCreated, `{"st":2,"ss":2,"tr":true}`)); err != nil {
				t.Fatalf("create: %v", err)
			}
			task := requireTask(t, state, "lifecycle")
			if task.Schedule != things.TaskScheduleSomeday || task.Status != things.TaskStatusCanceled || !task.InTrash {
				t.Fatalf("lifecycle fields = schedule %v status %v trash %v", task.Schedule, task.Status, task.InTrash)
			}
			if err := state.Update(things.Item{UUID: "lifecycle", Kind: task7Kind, Action: things.ItemActionDeleted, P: payload}); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, ok := state.Tasks["lifecycle"]; ok {
				t.Fatal("deleted Task7 remains in state")
			}
		})
	}
}

func TestStateUpdateTask7SyntheticProjectAndHeading(t *testing.T) {
	t.Parallel()

	state := NewState()
	err := state.Update(
		taskReadItem("project", task7Kind, things.ItemActionCreated, `{"tt":"Project","tp":1}`),
		taskReadItem("heading", task7Kind, things.ItemActionCreated, `{"tt":"Heading","tp":2,"pr":["project"]}`),
		taskReadItem("child", task7Kind, things.ItemActionCreated, `{"tt":"Child","tp":0,"pr":["project"],"agr":["heading"]}`),
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := state.Projects(); len(got) != 1 || got[0].UUID != "project" {
		t.Fatalf("Projects = %+v", got)
	}
	if got := state.Headings("project"); len(got) != 1 || got[0].UUID != "heading" {
		t.Fatalf("Headings = %+v", got)
	}
	if got := state.TasksByHeading("heading", ListOption{}); len(got) != 1 || got[0].UUID != "child" {
		t.Fatalf("TasksByHeading = %+v", got)
	}
}

func TestStateUpdateRejectsFutureTaskKindBeforeMutation(t *testing.T) {
	t.Parallel()

	const privateUUID = "PRIVATE-UUID-SHOULD-NOT-LEAK"
	const privatePayload = "PRIVATE-PAYLOAD-SHOULD-NOT-LEAK"
	state := NewState()
	if err := state.Update(taskReadItem("notes", task7Kind, things.ItemActionCreated, `{"nt":"base"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := state.Update(
		taskReadItem("notes", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":4,"l":0,"r":" once","ch":0}]}}`),
		taskReadItem(privateUUID, things.ItemKind("Task8"), things.ItemActionCreated, `{"tt":"`+privatePayload+`"}`),
	)
	if err == nil {
		t.Fatal("Update accepted unsupported Task8")
	}
	if got := requireTask(t, state, "notes").Note; got != "base" {
		t.Fatalf("batch mutated before unsupported kind error: note = %q", got)
	}
	if strings.Contains(err.Error(), privateUUID) || strings.Contains(err.Error(), privatePayload) {
		t.Fatalf("error leaked item details: %v", err)
	}
	if !strings.Contains(err.Error(), "Task8") {
		t.Fatalf("error %q does not identify unsupported kind", err)
	}
}

func TestStateUpdateRejectsMalformedTaskPayloadBeforeMutation(t *testing.T) {
	t.Parallel()

	const privateUUID = "PRIVATE-MALFORMED-UUID"
	const privatePayload = "PRIVATE-MALFORMED-PAYLOAD"
	state := NewState()
	if err := state.Update(taskReadItem("existing", task7Kind, things.ItemActionCreated, `{"tt":"before"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := state.Update(
		taskReadItem("existing", task7Kind, things.ItemActionModified, `{"tt":"after"}`),
		taskReadItem(privateUUID, task7Kind, things.ItemActionCreated, `"`+privatePayload+`"`),
	)
	if err == nil {
		t.Fatal("Update accepted non-object Task7 payload")
	}
	if got := requireTask(t, state, "existing").Title; got != "before" {
		t.Fatalf("batch mutated before malformed payload error: title = %q", got)
	}
	if strings.Contains(err.Error(), privateUUID) || strings.Contains(err.Error(), privatePayload) {
		t.Fatalf("error leaked item details: %v", err)
	}
}

func TestStateUpdateRejectsInvalidNoteDeltaBeforeBatchMutation(t *testing.T) {
	t.Parallel()

	state := NewState()
	if err := state.Update(
		taskReadItem("first", task7Kind, things.ItemActionCreated, `{"tt":"before","nt":"safe"}`),
		taskReadItem("unicode", task7Kind, things.ItemActionCreated, `{"nt":"α"}`),
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := state.Update(
		taskReadItem("first", task7Kind, things.ItemActionModified, `{"tt":"after","nt":"changed"}`),
		taskReadItem("created", task7Kind, things.ItemActionCreated, `{"tt":"must roll back"}`),
		taskReadItem("unicode", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":1,"l":1,"r":""}]}}`),
	)
	if err == nil {
		t.Fatal("Update accepted a note delta that produced invalid UTF-8")
	}
	if got := requireTask(t, state, "first"); got.Title != "before" || got.Note != "safe" {
		t.Fatalf("earlier task mutated: %+v", got)
	}
	if _, ok := state.Tasks["created"]; ok {
		t.Fatal("earlier task create survived rejected batch")
	}
	if got := requireTask(t, state, "unicode").Note; got != "α" {
		t.Fatalf("invalid delta changed note to %q", got)
	}
}

func TestStateUpdateNotePreflightTracksEarlierBatchEvents(t *testing.T) {
	t.Parallel()

	t.Run("create then malformed delta", func(t *testing.T) {
		state := NewState()
		err := state.Update(
			taskReadItem("note", task7Kind, things.ItemActionCreated, `{"nt":"α"}`),
			taskReadItem("note", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":1,"l":1,"r":""}]}}`),
		)
		if err == nil {
			t.Fatal("Update accepted malformed delta against an earlier create")
		}
		if _, ok := state.Tasks["note"]; ok {
			t.Fatal("create survived rejected batch")
		}
	})

	t.Run("valid edit then malformed delta", func(t *testing.T) {
		state := NewState()
		if err := state.Update(taskReadItem("note", task7Kind, things.ItemActionCreated, `{"nt":"α"}`)); err != nil {
			t.Fatal(err)
		}
		err := state.Update(
			taskReadItem("note", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":0,"l":0,"r":"A"}]}}`),
			taskReadItem("note", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":2,"l":1,"r":""}]}}`),
		)
		if err == nil {
			t.Fatal("Update accepted malformed delta against an earlier edit")
		}
		if got := requireTask(t, state, "note").Note; got != "α" {
			t.Fatalf("batch changed note to %q", got)
		}
	})

	t.Run("delete then delta starts from empty note", func(t *testing.T) {
		state := NewState()
		if err := state.Update(taskReadItem("note", task7Kind, things.ItemActionCreated, `{"nt":"α"}`)); err != nil {
			t.Fatal(err)
		}
		if err := state.Update(
			taskReadItem("note", task7Kind, things.ItemActionDeleted, `{}`),
			taskReadItem("note", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":1,"l":1,"r":""}]}}`),
		); err != nil {
			t.Fatalf("delta after delete should use an empty note: %v", err)
		}
		if got := requireTask(t, state, "note").Note; got != "" {
			t.Fatalf("note after delete and clamped delta = %q", got)
		}
	})

	t.Run("tombstone then delta starts from empty note", func(t *testing.T) {
		state := NewState()
		if err := state.Update(taskReadItem("note", task7Kind, things.ItemActionCreated, `{"nt":"α"}`)); err != nil {
			t.Fatal(err)
		}
		if err := state.Update(
			things.Item{UUID: "tombstone", Kind: things.ItemKindTombstone, P: json.RawMessage(`{"dloid":"note"}`)},
			taskReadItem("note", task7Kind, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":1,"l":1,"r":""}]}}`),
		); err != nil {
			t.Fatalf("delta after tombstone should use an empty note: %v", err)
		}
		if got := requireTask(t, state, "note").Note; got != "" {
			t.Fatalf("note after tombstone and clamped delta = %q", got)
		}
	})

	t.Run("legacy tombstone and task share normalized identity", func(t *testing.T) {
		const legacyID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
		currentID := things.EncodeLegacyIdentifier(legacyID)
		state := NewState()
		if err := state.Update(taskReadItem(legacyID, things.ItemKindTaskPlain, things.ItemActionCreated, `{"nt":"α"}`)); err != nil {
			t.Fatal(err)
		}
		if err := state.Update(
			things.Item{UUID: "legacy-tombstone", Kind: things.ItemKindTombstonePlain, P: json.RawMessage(`{"dloid":"` + legacyID + `"}`)},
			taskReadItem(legacyID, things.ItemKindTaskPlain, things.ItemActionModified, `{"nt":{"t":2,"ps":[{"p":1,"l":1,"r":""}]}}`),
		); err != nil {
			t.Fatalf("delta after legacy tombstone should use an empty note: %v", err)
		}
		if got := requireTask(t, state, currentID).Note; got != "" {
			t.Fatalf("note after legacy tombstone and clamped delta = %q", got)
		}
	})
}

func TestStateUpdateTask7DatePrecision(t *testing.T) {
	t.Parallel()

	state := NewState()
	if err := state.Update(taskReadItem("date", task7Kind, things.ItemActionCreated, `{"sr":1700000000.25}`)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := time.Unix(1700000000, 250000000).UTC()
	if got := requireTask(t, state, "date").ScheduledDate; got == nil || !got.Equal(want) {
		t.Fatalf("scheduled date = %v, want %v", got, want)
	}
}
