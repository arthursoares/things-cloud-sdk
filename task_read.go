package thingscloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TaskReplayVersion identifies the task read semantics used to build derived
// state. Cached state from older generations must be replayed from scratch.
const TaskReplayVersion = 3

// UnsupportedTaskKindError means the task state would be incomplete if replay
// continued. ServerIndex is -1 when the source did not provide an index.
type UnsupportedTaskKindError struct {
	Kind        ItemKind
	ServerIndex int
}

func (e *UnsupportedTaskKindError) Error() string {
	kind := taskKindDiagnostic(e.Kind)
	if e.ServerIndex >= 0 {
		return fmt.Sprintf("incomplete sync: unsupported task kind %s at server index %d", kind, e.ServerIndex)
	}
	return fmt.Sprintf("incomplete sync: unsupported task kind %s", kind)
}

func taskKindDiagnostic(value ItemKind) string {
	// Only print the protocol's Task<number> shape. Do not let an arbitrary
	// server-supplied kind embed private text or control characters in logs.
	kind := string(value)
	suffix := strings.TrimPrefix(kind, "Task")
	if suffix == kind || suffix == "" || len(suffix) > 12 || strings.Trim(suffix, "0123456789") != "" {
		kind = "Task*"
	}
	return kind
}

// ValidateTaskReadKinds preflights an entire batch before any task state is
// mutated. Unrelated unknown entities retain their existing handling.
func ValidateTaskReadKinds(items []Item) error {
	for _, item := range items {
		switch item.Kind {
		case ItemKindTask, ItemKindTask7, ItemKindTask4, ItemKindTask3, ItemKindTaskPlain:
			continue
		}
		if strings.HasPrefix(string(item.Kind), "Task") {
			index := -1
			if item.HasServerIndex {
				index = item.ServerIndex
			}
			return &UnsupportedTaskKindError{Kind: item.Kind, ServerIndex: index}
		}
	}
	return nil
}

// TaskReadPayload adds explicit-null tracking to the existing wire payload
// without changing its write representation. Unknown properties remain
// available in the original Item.P; this helper interprets supported fields.
type TaskReadPayload struct {
	TaskActionItemPayload
	nullFields map[string]bool
}

// DecodeTaskReadPayload distinguishes an omitted property (preserve existing
// state) from an explicit null (clear a nullable property).
func DecodeTaskReadPayload(raw json.RawMessage) (TaskReadPayload, error) {
	var result TaskReadPayload
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || data[0] != '{' {
		return result, fmt.Errorf("task payload must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return result, fmt.Errorf("decoding task payload: %w", err)
	}
	if err := json.Unmarshal(data, &result.TaskActionItemPayload); err != nil {
		return result, fmt.Errorf("decoding task payload: %w", err)
	}
	if note, ok := fields["nt"]; ok {
		if err := validateTaskReadNote(note); err != nil {
			return result, err
		}
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if result.nullFields == nil {
				result.nullFields = make(map[string]bool)
			}
			result.nullFields[key] = true
		}
	}
	return result, nil
}

func validateTaskReadNote(raw json.RawMessage) error {
	value := bytes.TrimSpace(raw)
	if bytes.Equal(value, []byte("null")) || (len(value) > 0 && value[0] == '"') {
		return nil // The enclosing JSON decoder already checked string syntax.
	}
	if len(value) == 0 || value[0] != '{' {
		return fmt.Errorf("task note must be text, null, or a supported structured note")
	}
	var note Note
	if err := json.Unmarshal(value, &note); err != nil {
		return fmt.Errorf("decoding task note: %w", err)
	}
	if note.Type != NoteTypeFullText && note.Type != NoteTypeDelta {
		return fmt.Errorf("unsupported task note type %d", note.Type)
	}
	return nil
}

// ResolveNote applies the note field to current using the read protocol's
// plain-text, full-text, delta, and explicit-null semantics.
func (p TaskReadPayload) ResolveNote(current string) (string, error) {
	if p.nullFields["nt"] {
		return "", nil
	}
	if len(p.Note) == 0 {
		return current, nil
	}

	var noteText string
	if err := json.Unmarshal(p.Note, &noteText); err == nil {
		return noteText, nil
	}

	var note Note
	if err := json.Unmarshal(p.Note, &note); err != nil {
		return "", fmt.Errorf("decoding task note: %w", err)
	}
	switch note.Type {
	case NoteTypeFullText:
		return note.Value, nil
	case NoteTypeDelta:
		result, err := ApplyPatchesChecked(current, note.Patches)
		if err != nil {
			return "", fmt.Errorf("applying task note delta: %w", err)
		}
		return result, nil
	default:
		return "", fmt.Errorf("unsupported task note type %d", note.Type)
	}
}

// ApplyNulls clears nullable fields after the ordinary non-nil payload fields
// have been applied. In particular, tir is a reference date and never changes
// ScheduledDate; only sr controls that field.
func (p TaskReadPayload) ApplyNulls(task *Task) {
	for key := range p.nullFields {
		switch key {
		case "cd":
			task.CreationDate = time.Time{}
		case "md":
			task.ModificationDate = nil
		case "sr":
			task.ScheduledDate = nil
		case "sp":
			task.CompletionDate = nil
		case "dd":
			task.DeadlineDate = nil
		case "ato":
			task.AlarmTimeOffset = nil
		case "ar":
			task.AreaIDs = nil
		case "pr":
			task.ParentTaskIDs = nil
		case "agr":
			task.ActionGroupIDs = nil
		case "tg":
			task.TagIDs = nil
		case "rt":
			task.RecurrenceIDs = nil
		case "dl":
			task.DelegateIDs = nil
		case "nt":
			task.Note = ""
		}
	}
}
