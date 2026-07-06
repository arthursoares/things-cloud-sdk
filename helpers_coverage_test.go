package thingscloud

import "testing"

func TestSchedule_Ptr(t *testing.T) {
	t.Parallel()
	p := Schedule(TaskScheduleSomeday)
	if p == nil {
		t.Fatal("Schedule returned nil")
	}
	if *p != TaskScheduleSomeday {
		t.Errorf("*Schedule = %d, want %d", *p, TaskScheduleSomeday)
	}
}

func TestTaskTypePtr_Ptr(t *testing.T) {
	t.Parallel()
	p := TaskTypePtr(TaskTypeProject)
	if p == nil {
		t.Fatal("TaskTypePtr returned nil")
	}
	if *p != TaskTypeProject {
		t.Errorf("*TaskTypePtr = %d, want %d", *p, TaskTypeProject)
	}
}
