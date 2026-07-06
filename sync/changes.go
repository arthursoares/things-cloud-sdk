package sync

import (
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Change represents a semantic change event from sync
type Change interface {
	// ChangeType returns the type of change (e.g., "TaskCreated", "TaskCompleted")
	ChangeType() string
	// EntityType returns the type of entity (e.g., "Task", "Area", "Tag")
	EntityType() string
	// EntityUUID returns the UUID of the affected entity
	EntityUUID() string
	// ServerIndex returns the server index at which this change occurred
	ServerIndex() int
	// Timestamp returns when this change was synced
	Timestamp() time.Time
}

// TaskLocation represents where a task is located in the Things UI
type TaskLocation int

const (
	LocationUnknown TaskLocation = iota
	LocationInbox
	LocationToday
	LocationAnytime
	LocationSomeday
	LocationUpcoming
	LocationProject
)

// String returns the string representation of TaskLocation
func (l TaskLocation) String() string {
	switch l {
	case LocationInbox:
		return "Inbox"
	case LocationToday:
		return "Today"
	case LocationAnytime:
		return "Anytime"
	case LocationSomeday:
		return "Someday"
	case LocationUpcoming:
		return "Upcoming"
	case LocationProject:
		return "Project"
	default:
		return "Unknown"
	}
}

// baseChange provides common fields for all change types
type baseChange struct {
	serverIndex int
	timestamp   time.Time
}

// ServerIndex returns the server index at which this change occurred
func (b baseChange) ServerIndex() int {
	return b.serverIndex
}

// Timestamp returns when this change was synced
func (b baseChange) Timestamp() time.Time {
	return b.timestamp
}

// taskChange provides common fields for task-related changes
type taskChange struct {
	baseChange
	Task *things.Task
}

// EntityType returns "Task" for all task changes
func (t taskChange) EntityType() string {
	return "Task"
}

// EntityUUID returns the UUID of the task
func (t taskChange) EntityUUID() string {
	if t.Task == nil {
		return ""
	}
	return t.Task.UUID
}

// TaskCreated indicates a new task was created
type TaskCreated struct {
	taskChange
}

// ChangeType returns "TaskCreated"
func (c TaskCreated) ChangeType() string {
	return "TaskCreated"
}

// TaskDeleted indicates a task was permanently deleted
type TaskDeleted struct {
	taskChange
}

// ChangeType returns "TaskDeleted"
func (c TaskDeleted) ChangeType() string {
	return "TaskDeleted"
}

// TaskCompleted indicates a task was marked as completed
type TaskCompleted struct {
	taskChange
}

// ChangeType returns "TaskCompleted"
func (c TaskCompleted) ChangeType() string {
	return "TaskCompleted"
}

// TaskUncompleted indicates a completed task was marked as incomplete
type TaskUncompleted struct {
	taskChange
}

// ChangeType returns "TaskUncompleted"
func (c TaskUncompleted) ChangeType() string {
	return "TaskUncompleted"
}

// TaskCanceled indicates a task was canceled
type TaskCanceled struct {
	taskChange
}

// ChangeType returns "TaskCanceled"
func (c TaskCanceled) ChangeType() string {
	return "TaskCanceled"
}

// TaskTitleChanged indicates a task's title was modified
type TaskTitleChanged struct {
	taskChange
	OldTitle string
}

// ChangeType returns "TaskTitleChanged"
func (c TaskTitleChanged) ChangeType() string {
	return "TaskTitleChanged"
}

// TaskNoteChanged indicates a task's note was modified
type TaskNoteChanged struct {
	taskChange
	OldNote string
}

// ChangeType returns "TaskNoteChanged"
func (c TaskNoteChanged) ChangeType() string {
	return "TaskNoteChanged"
}

// TaskMovedToInbox indicates a task was moved to the Inbox
type TaskMovedToInbox struct {
	taskChange
	From TaskLocation
}

// ChangeType returns "TaskMovedToInbox"
func (c TaskMovedToInbox) ChangeType() string {
	return "TaskMovedToInbox"
}

// TaskMovedToToday indicates a task was moved to Today
type TaskMovedToToday struct {
	taskChange
	From TaskLocation
}

// ChangeType returns "TaskMovedToToday"
func (c TaskMovedToToday) ChangeType() string {
	return "TaskMovedToToday"
}

// TaskMovedToAnytime indicates a task was moved to Anytime
type TaskMovedToAnytime struct {
	taskChange
	From TaskLocation
}

// ChangeType returns "TaskMovedToAnytime"
func (c TaskMovedToAnytime) ChangeType() string {
	return "TaskMovedToAnytime"
}

// TaskMovedToSomeday indicates a task was moved to Someday
type TaskMovedToSomeday struct {
	taskChange
	From TaskLocation
}

// ChangeType returns "TaskMovedToSomeday"
func (c TaskMovedToSomeday) ChangeType() string {
	return "TaskMovedToSomeday"
}

// TaskMovedToUpcoming indicates a task was scheduled for a future date
type TaskMovedToUpcoming struct {
	taskChange
	From         TaskLocation
	ScheduledFor time.Time
}

// ChangeType returns "TaskMovedToUpcoming"
func (c TaskMovedToUpcoming) ChangeType() string {
	return "TaskMovedToUpcoming"
}

// TaskDeadlineChanged indicates a task's deadline was modified
type TaskDeadlineChanged struct {
	taskChange
	OldDeadline *time.Time
}

// ChangeType returns "TaskDeadlineChanged"
func (c TaskDeadlineChanged) ChangeType() string {
	return "TaskDeadlineChanged"
}

// TaskAssignedToProject indicates a task was assigned to a project
type TaskAssignedToProject struct {
	taskChange
	Project    *things.Task
	OldProject *things.Task
}

// ChangeType returns "TaskAssignedToProject"
func (c TaskAssignedToProject) ChangeType() string {
	return "TaskAssignedToProject"
}

// TaskAssignedToArea indicates a task was assigned to an area
type TaskAssignedToArea struct {
	taskChange
	Area    *things.Area
	OldArea *things.Area
}

// ChangeType returns "TaskAssignedToArea"
func (c TaskAssignedToArea) ChangeType() string {
	return "TaskAssignedToArea"
}

// TaskTrashed indicates a task was moved to trash
type TaskTrashed struct {
	taskChange
}

// ChangeType returns "TaskTrashed"
func (c TaskTrashed) ChangeType() string {
	return "TaskTrashed"
}

// TaskRestored indicates a task was restored from trash
type TaskRestored struct {
	taskChange
}

// ChangeType returns "TaskRestored"
func (c TaskRestored) ChangeType() string {
	return "TaskRestored"
}

// TaskTagsChanged indicates a task's tags were modified
type TaskTagsChanged struct {
	taskChange
	Added   []string
	Removed []string
}

// ChangeType returns "TaskTagsChanged"
func (c TaskTagsChanged) ChangeType() string {
	return "TaskTagsChanged"
}

// projectChange provides common fields for project-related changes
type projectChange struct {
	baseChange
	Project *things.Task
}

// EntityType returns "Project" for all project changes
func (p projectChange) EntityType() string {
	return "Project"
}

// EntityUUID returns the UUID of the project
func (p projectChange) EntityUUID() string {
	if p.Project == nil {
		return ""
	}
	return p.Project.UUID
}

// ProjectCreated indicates a new project was created
type ProjectCreated struct {
	projectChange
}

// ChangeType returns "ProjectCreated"
func (c ProjectCreated) ChangeType() string {
	return "ProjectCreated"
}

// ProjectDeleted indicates a project was permanently deleted
type ProjectDeleted struct {
	projectChange
}

// ChangeType returns "ProjectDeleted"
func (c ProjectDeleted) ChangeType() string {
	return "ProjectDeleted"
}

// ProjectCompleted indicates a project was marked as completed
type ProjectCompleted struct {
	projectChange
}

// ChangeType returns "ProjectCompleted"
func (c ProjectCompleted) ChangeType() string {
	return "ProjectCompleted"
}

// ProjectTitleChanged indicates a project's title was modified
type ProjectTitleChanged struct {
	projectChange
	OldTitle string
}

// ChangeType returns "ProjectTitleChanged"
func (c ProjectTitleChanged) ChangeType() string {
	return "ProjectTitleChanged"
}

// ProjectTrashed indicates a project was moved to trash
type ProjectTrashed struct {
	projectChange
}

// ChangeType returns "ProjectTrashed"
func (c ProjectTrashed) ChangeType() string {
	return "ProjectTrashed"
}

// ProjectRestored indicates a project was restored from trash
type ProjectRestored struct {
	projectChange
}

// ChangeType returns "ProjectRestored"
func (c ProjectRestored) ChangeType() string {
	return "ProjectRestored"
}

// headingChange provides common fields for heading-related changes
type headingChange struct {
	baseChange
	Heading *things.Task
}

// EntityType returns "Heading" for all heading changes
func (h headingChange) EntityType() string {
	return "Heading"
}

// EntityUUID returns the UUID of the heading
func (h headingChange) EntityUUID() string {
	if h.Heading == nil {
		return ""
	}
	return h.Heading.UUID
}

// HeadingCreated indicates a new heading was created
type HeadingCreated struct {
	headingChange
	Project *things.Task
}

// ChangeType returns "HeadingCreated"
func (c HeadingCreated) ChangeType() string {
	return "HeadingCreated"
}

// HeadingDeleted indicates a heading was permanently deleted
type HeadingDeleted struct {
	headingChange
}

// ChangeType returns "HeadingDeleted"
func (c HeadingDeleted) ChangeType() string {
	return "HeadingDeleted"
}

// HeadingTitleChanged indicates a heading's title was modified
type HeadingTitleChanged struct {
	headingChange
	OldTitle string
}

// ChangeType returns "HeadingTitleChanged"
func (c HeadingTitleChanged) ChangeType() string {
	return "HeadingTitleChanged"
}

// areaChange provides common fields for area-related changes
type areaChange struct {
	baseChange
	Area *things.Area
}

// EntityType returns "Area" for all area changes
func (a areaChange) EntityType() string {
	return "Area"
}

// EntityUUID returns the UUID of the area
func (a areaChange) EntityUUID() string {
	if a.Area == nil {
		return ""
	}
	return a.Area.UUID
}

// AreaCreated indicates a new area was created
type AreaCreated struct {
	areaChange
}

// ChangeType returns "AreaCreated"
func (c AreaCreated) ChangeType() string {
	return "AreaCreated"
}

// AreaDeleted indicates an area was permanently deleted
type AreaDeleted struct {
	areaChange
}

// ChangeType returns "AreaDeleted"
func (c AreaDeleted) ChangeType() string {
	return "AreaDeleted"
}

// AreaRenamed indicates an area's title was modified
type AreaRenamed struct {
	areaChange
	OldTitle string
}

// ChangeType returns "AreaRenamed"
func (c AreaRenamed) ChangeType() string {
	return "AreaRenamed"
}

// tagChange provides common fields for tag-related changes
type tagChange struct {
	baseChange
	Tag *things.Tag
}

// EntityType returns "Tag" for all tag changes
func (t tagChange) EntityType() string {
	return "Tag"
}

// EntityUUID returns the UUID of the tag
func (t tagChange) EntityUUID() string {
	if t.Tag == nil {
		return ""
	}
	return t.Tag.UUID
}

// TagCreated indicates a new tag was created
type TagCreated struct {
	tagChange
}

// ChangeType returns "TagCreated"
func (c TagCreated) ChangeType() string {
	return "TagCreated"
}

// TagDeleted indicates a tag was permanently deleted
type TagDeleted struct {
	tagChange
}

// ChangeType returns "TagDeleted"
func (c TagDeleted) ChangeType() string {
	return "TagDeleted"
}

// TagRenamed indicates a tag's title was modified
type TagRenamed struct {
	tagChange
	OldTitle string
}

// ChangeType returns "TagRenamed"
func (c TagRenamed) ChangeType() string {
	return "TagRenamed"
}

// TagShortcutChanged indicates a tag's keyboard shortcut was modified
type TagShortcutChanged struct {
	tagChange
	OldShortcut string
}

// ChangeType returns "TagShortcutChanged"
func (c TagShortcutChanged) ChangeType() string {
	return "TagShortcutChanged"
}

// checklistItemChange provides common fields for checklist item-related changes
type checklistItemChange struct {
	baseChange
	Item *things.CheckListItem
}

// EntityType returns "ChecklistItem" for all checklist item changes
func (c checklistItemChange) EntityType() string {
	return "ChecklistItem"
}

// EntityUUID returns the UUID of the checklist item
func (c checklistItemChange) EntityUUID() string {
	if c.Item == nil {
		return ""
	}
	return c.Item.UUID
}

// ChecklistItemCreated indicates a new checklist item was created
type ChecklistItemCreated struct {
	checklistItemChange
	Task *things.Task
}

// ChangeType returns "ChecklistItemCreated"
func (c ChecklistItemCreated) ChangeType() string {
	return "ChecklistItemCreated"
}

// ChecklistItemDeleted indicates a checklist item was permanently deleted
type ChecklistItemDeleted struct {
	checklistItemChange
}

// ChangeType returns "ChecklistItemDeleted"
func (c ChecklistItemDeleted) ChangeType() string {
	return "ChecklistItemDeleted"
}

// ChecklistItemCompleted indicates a checklist item was marked as completed
type ChecklistItemCompleted struct {
	checklistItemChange
	Task *things.Task
}

// ChangeType returns "ChecklistItemCompleted"
func (c ChecklistItemCompleted) ChangeType() string {
	return "ChecklistItemCompleted"
}

// ChecklistItemUncompleted indicates a checklist item was marked as incomplete
type ChecklistItemUncompleted struct {
	checklistItemChange
	Task *things.Task
}

// ChangeType returns "ChecklistItemUncompleted"
func (c ChecklistItemUncompleted) ChangeType() string {
	return "ChecklistItemUncompleted"
}

// ChecklistItemTitleChanged indicates a checklist item's title was modified
type ChecklistItemTitleChanged struct {
	checklistItemChange
	OldTitle string
}

// ChangeType returns "ChecklistItemTitleChanged"
func (c ChecklistItemTitleChanged) ChangeType() string {
	return "ChecklistItemTitleChanged"
}

// UnknownChange represents a change that could not be categorized
type UnknownChange struct {
	baseChange
	entityType string
	entityUUID string
	Details    string
}

// ChangeType returns "UnknownChange"
func (c UnknownChange) ChangeType() string {
	return "UnknownChange"
}

// EntityType returns the entity type string
func (c UnknownChange) EntityType() string {
	return c.entityType
}

// EntityUUID returns the entity UUID
func (c UnknownChange) EntityUUID() string {
	return c.entityUUID
}

// LoggedChange represents a semantic change restored from the persisted change log.
// It preserves the stored change type and metadata, but not type-specific fields
// that are not persisted in the current schema.
type LoggedChange struct {
	baseChange
	changeType string
	entityType string
	entityUUID string
	payload    string
}

// ChangeType returns the persisted semantic change type.
func (c LoggedChange) ChangeType() string {
	return c.changeType
}

// EntityType returns the persisted entity type.
func (c LoggedChange) EntityType() string {
	return c.entityType
}

// EntityUUID returns the persisted entity UUID.
func (c LoggedChange) EntityUUID() string {
	return c.entityUUID
}

// Payload returns the raw item payload stored with the change log entry.
func (c LoggedChange) Payload() string {
	return c.payload
}

// Compile-time interface implementation checks
var (
	_ Change = (*TaskCreated)(nil)
	_ Change = (*TaskDeleted)(nil)
	_ Change = (*TaskCompleted)(nil)
	_ Change = (*TaskUncompleted)(nil)
	_ Change = (*TaskCanceled)(nil)
	_ Change = (*TaskTitleChanged)(nil)
	_ Change = (*TaskNoteChanged)(nil)
	_ Change = (*TaskMovedToInbox)(nil)
	_ Change = (*TaskMovedToToday)(nil)
	_ Change = (*TaskMovedToAnytime)(nil)
	_ Change = (*TaskMovedToSomeday)(nil)
	_ Change = (*TaskMovedToUpcoming)(nil)
	_ Change = (*TaskDeadlineChanged)(nil)
	_ Change = (*TaskAssignedToProject)(nil)
	_ Change = (*TaskAssignedToArea)(nil)
	_ Change = (*TaskTrashed)(nil)
	_ Change = (*TaskRestored)(nil)
	_ Change = (*TaskTagsChanged)(nil)

	_ Change = (*ProjectCreated)(nil)
	_ Change = (*ProjectDeleted)(nil)
	_ Change = (*ProjectCompleted)(nil)
	_ Change = (*ProjectTitleChanged)(nil)
	_ Change = (*ProjectTrashed)(nil)
	_ Change = (*ProjectRestored)(nil)

	_ Change = (*HeadingCreated)(nil)
	_ Change = (*HeadingDeleted)(nil)
	_ Change = (*HeadingTitleChanged)(nil)

	_ Change = (*AreaCreated)(nil)
	_ Change = (*AreaDeleted)(nil)
	_ Change = (*AreaRenamed)(nil)

	_ Change = (*TagCreated)(nil)
	_ Change = (*TagDeleted)(nil)
	_ Change = (*TagRenamed)(nil)
	_ Change = (*TagShortcutChanged)(nil)

	_ Change = (*ChecklistItemCreated)(nil)
	_ Change = (*ChecklistItemDeleted)(nil)
	_ Change = (*ChecklistItemCompleted)(nil)
	_ Change = (*ChecklistItemUncompleted)(nil)
	_ Change = (*ChecklistItemTitleChanged)(nil)

	_ Change = (*UnknownChange)(nil)
	_ Change = (*LoggedChange)(nil)
)
