package sync

import (
	"encoding/json"
	"fmt"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Note: database/sql types are used via the dbExecutor interface defined in sync.go

// processItems processes a batch of Things Cloud items into semantic changes.
// The baseIndex is the starting server index for this batch.
func (s *Syncer) processItems(items []things.Item, baseIndex int) ([]Change, error) {
	if len(items) == 0 {
		return nil, nil
	}

	// Wrap entire batch in a transaction for massive performance improvement
	tx, err := s.rawDB.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() // No-op if committed

	// Store original db, swap in transaction
	origDB := s.db
	s.db = tx
	defer func() { s.db = origDB }()

	var allChanges []Change

	for i, item := range items {
		serverIndex := baseIndex + i
		if item.HasServerIndex {
			serverIndex = item.ServerIndex
		}
		ts := time.Now()

		changes, err := s.processItem(item, serverIndex, ts)
		if err != nil {
			return nil, fmt.Errorf("processing item %s: %w", item.UUID, err)
		}

		// Log each change
		for _, change := range changes {
			payload, _ := json.Marshal(item.P)
			if err := s.logChange(serverIndex, change, string(payload)); err != nil {
				return nil, fmt.Errorf("logging change: %w", err)
			}
		}

		allChanges = append(allChanges, changes...)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return allChanges, nil
}

// processItem routes an item to the correct handler based on its Kind.
func (s *Syncer) processItem(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	switch item.Kind {
	case things.ItemKindTask, things.ItemKindTask4, things.ItemKindTask3, things.ItemKindTaskPlain:
		return s.processTaskItem(item, serverIndex, ts)
	case things.ItemKindArea, things.ItemKindArea3, things.ItemKindAreaPlain:
		return s.processAreaItem(item, serverIndex, ts)
	case things.ItemKindTag, things.ItemKindTag4, things.ItemKindTagPlain:
		return s.processTagItem(item, serverIndex, ts)
	case things.ItemKindChecklistItem, things.ItemKindChecklistItem2, things.ItemKindChecklistItem3:
		return s.processChecklistItem(item, serverIndex, ts)
	case things.ItemKindTombstone:
		return s.processTombstone(item, serverIndex, ts)
	case things.ItemKindSettings:
		// Settings items are ignored for now
		return nil, nil
	default:
		// Unknown item kind - create an UnknownChange
		return []Change{UnknownChange{
			baseChange: baseChange{serverIndex: serverIndex, timestamp: ts},
			entityType: string(item.Kind),
			entityUUID: item.UUID,
			Details:    fmt.Sprintf("unknown item kind: %s", item.Kind),
		}}, nil
	}
}

// processTaskItem handles task/project/heading items.
func (s *Syncer) processTaskItem(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	// Get the old state
	old, err := s.getTask(item.UUID)
	if err != nil {
		return nil, fmt.Errorf("getting task %s: %w", item.UUID, err)
	}

	// Handle deletion
	if item.Action == things.ItemActionDeleted {
		if old != nil {
			if err := s.markTaskDeleted(item.UUID); err != nil {
				return nil, fmt.Errorf("marking task deleted: %w", err)
			}
		}
		return detectTaskChanges(old, nil, serverIndex, ts), nil
	}

	// Unmarshal the payload
	var payload things.TaskActionItemPayload
	if err := json.Unmarshal(item.P, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling task payload: %w", err)
	}

	// Apply payload to build new state
	newTask := applyTaskPayload(old, item.UUID, payload)

	// Save the new state
	if err := s.saveTask(newTask); err != nil {
		return nil, fmt.Errorf("saving task: %w", err)
	}

	// Detect and return changes
	return detectTaskChanges(old, newTask, serverIndex, ts), nil
}

// processAreaItem handles area items.
func (s *Syncer) processAreaItem(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	// Get the old state
	old, err := s.getArea(item.UUID)
	if err != nil {
		return nil, fmt.Errorf("getting area %s: %w", item.UUID, err)
	}

	// Handle deletion
	if item.Action == things.ItemActionDeleted {
		if old != nil {
			if err := s.markAreaDeleted(item.UUID); err != nil {
				return nil, fmt.Errorf("marking area deleted: %w", err)
			}
		}
		return detectAreaChanges(old, nil, serverIndex, ts), nil
	}

	// Unmarshal the payload
	var payload things.AreaActionItemPayload
	if err := json.Unmarshal(item.P, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling area payload: %w", err)
	}

	// Build new state from old or create new
	newArea := &things.Area{UUID: item.UUID}
	if old != nil {
		// Copy old state
		newArea.Title = old.Title
	}

	// Apply payload fields
	if payload.Title != nil {
		newArea.Title = *payload.Title
	}

	// Save the new state
	if err := s.saveArea(newArea); err != nil {
		return nil, fmt.Errorf("saving area: %w", err)
	}

	// Detect and return changes
	return detectAreaChanges(old, newArea, serverIndex, ts), nil
}

// processTagItem handles tag items.
func (s *Syncer) processTagItem(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	// Get the old state
	old, err := s.getTag(item.UUID)
	if err != nil {
		return nil, fmt.Errorf("getting tag %s: %w", item.UUID, err)
	}

	// Handle deletion
	if item.Action == things.ItemActionDeleted {
		if old != nil {
			if err := s.markTagDeleted(item.UUID); err != nil {
				return nil, fmt.Errorf("marking tag deleted: %w", err)
			}
		}
		return detectTagChanges(old, nil, serverIndex, ts), nil
	}

	// Unmarshal the payload
	var payload things.TagActionItemPayload
	if err := json.Unmarshal(item.P, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling tag payload: %w", err)
	}

	// Build new state from old or create new
	newTag := &things.Tag{UUID: item.UUID}
	if old != nil {
		// Copy old state
		newTag.Title = old.Title
		newTag.ShortHand = old.ShortHand
		newTag.ParentTagIDs = old.ParentTagIDs
	}

	// Apply payload fields
	if payload.Title != nil {
		newTag.Title = *payload.Title
	}
	if payload.ShortHand != nil {
		newTag.ShortHand = *payload.ShortHand
	}
	if payload.ParentTagIDs != nil {
		newTag.ParentTagIDs = *payload.ParentTagIDs
	}

	// Save the new state
	if err := s.saveTag(newTag); err != nil {
		return nil, fmt.Errorf("saving tag: %w", err)
	}

	// Detect and return changes
	return detectTagChanges(old, newTag, serverIndex, ts), nil
}

// processChecklistItem handles checklist items.
func (s *Syncer) processChecklistItem(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	// Get the old state
	old, err := s.getChecklistItem(item.UUID)
	if err != nil {
		return nil, fmt.Errorf("getting checklist item %s: %w", item.UUID, err)
	}

	// Handle deletion
	if item.Action == things.ItemActionDeleted {
		if old != nil {
			if err := s.markChecklistItemDeleted(item.UUID); err != nil {
				return nil, fmt.Errorf("marking checklist item deleted: %w", err)
			}
		}
		return detectChecklistChanges(old, nil, nil, serverIndex, ts), nil
	}

	// Unmarshal the payload
	var payload things.CheckListActionItemPayload
	if err := json.Unmarshal(item.P, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling checklist payload: %w", err)
	}

	// Build new state from old or create new
	newItem := &things.CheckListItem{UUID: item.UUID}
	if old != nil {
		// Copy old state
		newItem.Title = old.Title
		newItem.Status = old.Status
		newItem.Index = old.Index
		newItem.CreationDate = old.CreationDate
		newItem.ModificationDate = old.ModificationDate
		newItem.CompletionDate = old.CompletionDate
		newItem.TaskIDs = old.TaskIDs
	}

	// Apply payload fields
	if payload.Title != nil {
		newItem.Title = *payload.Title
	}
	if payload.Status != nil {
		newItem.Status = *payload.Status
	}
	if payload.Index != nil {
		newItem.Index = *payload.Index
	}
	if payload.CreationDate != nil {
		newItem.CreationDate = *payload.CreationDate.Time()
	}
	if payload.ModificationDate != nil {
		newItem.ModificationDate = payload.ModificationDate.Time()
	}
	if payload.CompletionDate != nil {
		newItem.CompletionDate = payload.CompletionDate.Time()
	}
	if payload.TaskIDs != nil {
		newItem.TaskIDs = *payload.TaskIDs
	}

	// Get the parent task for context in changes (if available)
	var task *things.Task
	if len(newItem.TaskIDs) > 0 {
		task, _ = s.getTask(newItem.TaskIDs[0])
	}

	// Save the new state
	if err := s.saveChecklistItem(newItem); err != nil {
		return nil, fmt.Errorf("saving checklist item: %w", err)
	}

	// Detect and return changes
	return detectChecklistChanges(old, newItem, task, serverIndex, ts), nil
}

// processTombstone handles tombstone deletion records.
// Tombstones are used for permanent deletions in Things Cloud.
func (s *Syncer) processTombstone(item things.Item, serverIndex int, ts time.Time) ([]Change, error) {
	// Unmarshal the tombstone payload
	var payload things.TombstoneActionItemPayload
	if err := json.Unmarshal(item.P, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling tombstone payload: %w", err)
	}

	deletedUUID := payload.DeletedObjectID
	var changes []Change

	// Try to find and delete the object in each table
	// Check if it's a task
	if task, _ := s.getTask(deletedUUID); task != nil {
		if err := s.markTaskDeleted(deletedUUID); err != nil {
			return nil, fmt.Errorf("marking task deleted via tombstone: %w", err)
		}
		changes = append(changes, detectTaskChanges(task, nil, serverIndex, ts)...)
		return changes, nil
	}

	// Check if it's an area
	if area, _ := s.getArea(deletedUUID); area != nil {
		if err := s.markAreaDeleted(deletedUUID); err != nil {
			return nil, fmt.Errorf("marking area deleted via tombstone: %w", err)
		}
		changes = append(changes, detectAreaChanges(area, nil, serverIndex, ts)...)
		return changes, nil
	}

	// Check if it's a tag
	if tag, _ := s.getTag(deletedUUID); tag != nil {
		if err := s.markTagDeleted(deletedUUID); err != nil {
			return nil, fmt.Errorf("marking tag deleted via tombstone: %w", err)
		}
		changes = append(changes, detectTagChanges(tag, nil, serverIndex, ts)...)
		return changes, nil
	}

	// Check if it's a checklist item
	if checklistItem, _ := s.getChecklistItem(deletedUUID); checklistItem != nil {
		if err := s.markChecklistItemDeleted(deletedUUID); err != nil {
			return nil, fmt.Errorf("marking checklist item deleted via tombstone: %w", err)
		}
		changes = append(changes, detectChecklistChanges(checklistItem, nil, nil, serverIndex, ts)...)
		return changes, nil
	}

	// Object not found - might have been deleted already or never synced
	return nil, nil
}

// applyTaskPayload applies a task payload to an existing task state (or creates a new one).
func applyTaskPayload(old *things.Task, uuid string, p things.TaskActionItemPayload) *things.Task {
	// Start with old state or create new with defaults
	t := &things.Task{
		UUID:     uuid,
		Schedule: things.TaskScheduleAnytime, // Default schedule for new tasks
	}

	if old != nil {
		// Copy all fields from old state
		t.CreationDate = old.CreationDate
		t.ModificationDate = old.ModificationDate
		t.Status = old.Status
		t.Title = old.Title
		t.Note = old.Note
		t.ScheduledDate = old.ScheduledDate
		t.CompletionDate = old.CompletionDate
		t.DeadlineDate = old.DeadlineDate
		t.Index = old.Index
		t.AreaIDs = old.AreaIDs
		t.ParentTaskIDs = old.ParentTaskIDs
		t.ActionGroupIDs = old.ActionGroupIDs
		t.InTrash = old.InTrash
		t.Schedule = old.Schedule
		t.Type = old.Type
		t.TodayIndex = old.TodayIndex
		t.DueOrder = old.DueOrder
		t.AlarmTimeOffset = old.AlarmTimeOffset
		t.TagIDs = old.TagIDs
		t.RecurrenceIDs = old.RecurrenceIDs
		t.DelegateIDs = old.DelegateIDs
	}

	// Apply each non-nil field from payload
	if p.CreationDate != nil {
		t.CreationDate = *p.CreationDate.Time()
	}
	if p.ModificationDate != nil {
		t.ModificationDate = p.ModificationDate.Time()
	}
	if p.Status != nil {
		t.Status = *p.Status
	}
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.ScheduledDate != nil {
		t.ScheduledDate = p.ScheduledDate.Time()
	}
	if p.TaskIR != nil {
		// TaskIR (tir) is an alternative scheduled date field
		t.ScheduledDate = p.TaskIR.Time()
	}
	if p.CompletionDate != nil {
		t.CompletionDate = p.CompletionDate.Time()
	}
	if p.DeadlineDate != nil {
		t.DeadlineDate = p.DeadlineDate.Time()
	}
	if p.Index != nil {
		t.Index = *p.Index
	}
	if p.AreaIDs != nil {
		t.AreaIDs = *p.AreaIDs
	}
	if p.ParentTaskIDs != nil {
		t.ParentTaskIDs = *p.ParentTaskIDs
	}
	if p.ActionGroupIDs != nil {
		t.ActionGroupIDs = *p.ActionGroupIDs
	}
	if p.InTrash != nil {
		t.InTrash = *p.InTrash
	}
	if p.Schedule != nil {
		t.Schedule = *p.Schedule
	}
	if p.Type != nil {
		t.Type = *p.Type
	}
	if p.TaskIndex != nil {
		t.TodayIndex = *p.TaskIndex
	}
	if p.DueOrder != nil {
		t.DueOrder = *p.DueOrder
	}
	if p.AlarmTimeOffset != nil {
		t.AlarmTimeOffset = p.AlarmTimeOffset
	}
	if p.TagIDs != nil {
		t.TagIDs = p.TagIDs
	}
	if p.RecurrenceTaskIDs != nil {
		t.RecurrenceIDs = *p.RecurrenceTaskIDs
	}
	if p.DelegateIDs != nil {
		t.DelegateIDs = *p.DelegateIDs
	}

	// Handle Note specially: can be string or Note struct with patches
	if len(p.Note) > 0 {
		t.Note = parseNotePayload(t.Note, p.Note)
	}

	return t
}

// parseNotePayload parses the note field from a task payload.
// The note can be either a plain string or a structured Note object with patches.
func parseNotePayload(currentNote string, raw json.RawMessage) string {
	// First, try to unmarshal as a string (most common case)
	var noteStr string
	if err := json.Unmarshal(raw, &noteStr); err == nil {
		return noteStr
	}

	// Try to unmarshal as a structured Note
	var note things.Note
	if err := json.Unmarshal(raw, &note); err != nil {
		// If both fail, return the current note unchanged
		return currentNote
	}

	// Handle based on note type
	switch note.Type {
	case things.NoteTypeFullText:
		// Full text replacement
		return note.Value
	case things.NoteTypeDelta:
		// Apply patches to current note
		return things.ApplyPatches(currentNote, note.Patches)
	default:
		// Unknown type, return value if present
		if note.Value != "" {
			return note.Value
		}
		return currentNote
	}
}
