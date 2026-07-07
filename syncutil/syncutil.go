// Package syncutil provides shared utilities for sync-based CLI tools.
package syncutil

import (
	"strings"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	"github.com/arthursoares/things-cloud-sdk/sync"
)

// FilterChanges returns changes matching the exact change type.
func FilterChanges(changes []sync.Change, changeType string) []sync.Change {
	var result []sync.Change
	for _, c := range changes {
		if c.ChangeType() == changeType {
			result = append(result, c)
		}
	}
	return result
}

// FilterChangesPrefix returns changes with types starting with the prefix.
func FilterChangesPrefix(changes []sync.Change, prefix string) []sync.Change {
	var result []sync.Change
	for _, c := range changes {
		if strings.HasPrefix(c.ChangeType(), prefix) {
			result = append(result, c)
		}
	}
	return result
}

// DaysSinceCreated returns the number of days since a task was created,
// based on the TaskCreated change in the change history.
func DaysSinceCreated(changes []sync.Change) int {
	for _, c := range changes {
		if c.ChangeType() == "TaskCreated" {
			return int(time.Since(c.Timestamp()).Hours() / 24)
		}
	}
	return 0
}

// CountMoves returns the number of times a task was moved (TaskMovedTo* changes).
func CountMoves(changes []sync.Change) int {
	count := 0
	for _, c := range changes {
		if strings.HasPrefix(c.ChangeType(), "TaskMovedTo") {
			count++
		}
	}
	return count
}

// TaskAge returns the number of days since the task was created.
func TaskAge(task *things.Task) int {
	if task.CreationDate.IsZero() {
		return 0
	}
	return int(time.Since(task.CreationDate).Hours() / 24)
}

// DailySummary holds counts of today's activity.
type DailySummary struct {
	Completed    int `json:"completed"`
	Created      int `json:"created"`
	MovedToToday int `json:"movedToToday"`
	Rescheduled  int `json:"rescheduled"`
}

// BuildDailySummary calculates activity stats from today's changes.
// timeNow is a test seam.
var timeNow = time.Now

func BuildDailySummary(syncer *sync.Syncer) DailySummary {
	// Local midnight — Truncate would give midnight UTC and count part of
	// yesterday (or miss part of today) in any other timezone.
	now := timeNow()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	changes, _ := syncer.ChangesSince(today)

	summary := DailySummary{}
	for _, c := range changes {
		switch c.ChangeType() {
		case "TaskCompleted":
			summary.Completed++
		case "TaskCreated":
			summary.Created++
		case "TaskMovedToToday":
			summary.MovedToToday++
		}
		if strings.HasPrefix(c.ChangeType(), "TaskMovedTo") && c.ChangeType() != "TaskMovedToToday" {
			summary.Rescheduled++
		}
	}
	return summary
}
