package thingscloud

import "time"

// String returns a pointer to a string
func String(str string) *string {
	return &str
}

// Status returns a pointer to a TaskStatus
func Status(val TaskStatus) *TaskStatus {
	return &val
}

// Schedule returns a pointer to a TaskSchedule
func Schedule(val TaskSchedule) *TaskSchedule {
	return &val
}

// TaskTypePtr returns a pointer to a TaskType
func TaskTypePtr(val TaskType) *TaskType {
	return &val
}

// Time returns a pointer to a Time. The zero time returns nil: its
// UnixNano is out of range and would marshal to a garbage 1754 date.
func Time(val time.Time) *Timestamp {
	if val.IsZero() {
		return nil
	}
	ts := Timestamp(val)
	return &ts
}
