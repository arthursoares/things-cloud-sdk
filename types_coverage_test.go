package thingscloud

import (
	"encoding/json"
	"testing"
)

func TestActionItem_UUID(t *testing.T) {
	t.Parallel()
	const id = "VJ1edXTP9q3PmFDUuy8EQh"

	if got := (TagActionItem{Item: Item{UUID: id}}).UUID(); got != id {
		t.Errorf("TagActionItem.UUID() = %q, want %q", got, id)
	}
	if got := (AreaActionItem{Item: Item{UUID: id}}).UUID(); got != id {
		t.Errorf("AreaActionItem.UUID() = %q, want %q", got, id)
	}
	if got := (CheckListActionItem{Item: Item{UUID: id}}).UUID(); got != id {
		t.Errorf("CheckListActionItem.UUID() = %q, want %q", got, id)
	}
	if got := (TombstoneActionItem{Item: Item{UUID: id}}).UUID(); got != id {
		t.Errorf("TombstoneActionItem.UUID() = %q, want %q", got, id)
	}
}

func TestTimestamp_UnmarshalJSON_Error(t *testing.T) {
	t.Parallel()
	var ts Timestamp
	// A JSON string is not a number and must fail to unmarshal into a float.
	if err := ts.UnmarshalJSON([]byte(`"not a number"`)); err == nil {
		t.Error("expected UnmarshalJSON to fail on a non-numeric value, got nil")
	}
}

func TestBoolean_UnmarshalJSON_Error(t *testing.T) {
	t.Parallel()
	var b Boolean
	// A JSON string is not an int and must fail to unmarshal.
	if err := b.UnmarshalJSON([]byte(`"true"`)); err == nil {
		t.Error("expected UnmarshalJSON to fail on a non-integer value, got nil")
	}
}

func TestTimestamp_UnmarshalJSON_SubSecond(t *testing.T) {
	t.Parallel()
	var ts Timestamp
	if err := json.Unmarshal([]byte(`1770713623.4716659`), &ts); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	tt := ts.Time()
	if tt.Unix() != 1770713623 {
		t.Errorf("Unix seconds = %d, want 1770713623", tt.Unix())
	}
	if tt.Nanosecond() == 0 {
		t.Error("sub-second precision was dropped")
	}
}

func TestItemAction_String(t *testing.T) {
	t.Parallel()
	cases := map[ItemAction]string{
		ItemActionCreated:  "ItemActionCreated",
		ItemActionModified: "ItemActionModified",
		ItemActionDeleted:  "ItemActionDeleted",
	}
	for action, want := range cases {
		if got := action.String(); got != want {
			t.Errorf("ItemAction(%d).String() = %q, want %q", action, got, want)
		}
	}
	// Out-of-range values fall through to the fmt.Sprintf branch.
	if got := ItemAction(99).String(); got == "" {
		t.Error("out-of-range ItemAction.String() returned empty")
	}
}

func TestTaskStatus_String(t *testing.T) {
	t.Parallel()
	cases := map[TaskStatus]string{
		TaskStatusPending:   "TaskStatusPending",
		TaskStatusCanceled:  "TaskStatusCanceled",
		TaskStatusCompleted: "TaskStatusCompleted",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("TaskStatus(%d).String() = %q, want %q", status, got, want)
		}
	}
	// A value outside the known set hits the default fmt.Sprintf branch.
	if got := TaskStatus(99).String(); got == "" {
		t.Error("out-of-range TaskStatus.String() returned empty")
	}
}

func TestTaskSchedule_String(t *testing.T) {
	t.Parallel()
	cases := map[TaskSchedule]string{
		TaskScheduleInbox:   "TaskScheduleInbox",
		TaskScheduleAnytime: "TaskScheduleAnytime",
		TaskScheduleSomeday: "TaskScheduleSomeday",
	}
	for sched, want := range cases {
		if got := sched.String(); got != want {
			t.Errorf("TaskSchedule(%d).String() = %q, want %q", sched, got, want)
		}
	}
	if got := TaskSchedule(99).String(); got == "" {
		t.Error("out-of-range TaskSchedule.String() returned empty")
	}
}
