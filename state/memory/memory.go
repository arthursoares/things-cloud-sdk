package memory

import (
	"encoding/json"
	"sort"

	things "github.com/arthursoares/things-cloud-sdk"
)

// itemKindArea2 is things.ItemKindArea. Read paths reference it through this
// alias so the write-only deprecation is suppressed on one line instead of
// across a whole switch clause, where it would also hide unrelated findings.
//
//nolint:staticcheck // deprecated for writes only; reading Area2 stays supported
var itemKindArea2 = things.ItemKindArea

// State is created by applying all history items in order.
// Note that the hierarchy within the state (e.g. area > tasks > tasks > check list items)
// is modelled with pointers between the different maps, so concurrent modification
// is not safe.
type State struct {
	Areas          map[string]*things.Area
	Tasks          map[string]*things.Task
	Tags           map[string]*things.Tag
	CheckListItems map[string]*things.CheckListItem
}

// NewState creates a new, empty state
func NewState() *State {
	return &State{
		Areas:          map[string]*things.Area{},
		Tags:           map[string]*things.Tag{},
		CheckListItems: map[string]*things.CheckListItem{},
		Tasks:          map[string]*things.Task{},
	}
}

// Things Cloud migrated from UUID-style identifiers to Base58 identifiers
// without writing mapping records into the history: the current identifier
// is derived by hashing the old one (see things.EncodeLegacyIdentifier).
// Items of the kinds below are keyed by old identifiers, so replay stores
// them under their derived identifiers; later current-kind events then find
// and update the same objects.
func isLegacyItemKind(kind things.ItemKind) bool {
	switch kind {
	case things.ItemKindTask4, things.ItemKindTask3, things.ItemKindTaskPlain,
		things.ItemKindChecklistItem, things.ItemKindChecklistItem2,
		itemKindArea2, things.ItemKindAreaPlain,
		things.ItemKindTag, things.ItemKindTagPlain,
		things.ItemKindTombstonePlain:
		return true
	default:
		return false
	}
}

// encodeLegacyIDs rewrites each identifier in place to its derived
// equivalent. Identifiers that already parse as canonical Base58 are
// current-generation and left untouched: deriving them again would produce a
// phantom key. Genuine legacy identifiers always contain '-', which is not
// in the Base58 alphabet, so they can never be skipped by mistake.
func encodeLegacyIDs(ids []string) {
	for i := range ids {
		if things.ValidateUUID(ids[i]) == nil {
			continue
		}
		ids[i] = things.EncodeLegacyIdentifier(ids[i])
	}
}

func encodeLegacyTaskReferences(p *things.TaskActionItemPayload) {
	if p.AreaIDs != nil {
		encodeLegacyIDs(*p.AreaIDs)
	}
	if p.ParentTaskIDs != nil {
		encodeLegacyIDs(*p.ParentTaskIDs)
	}
	if p.ActionGroupIDs != nil {
		encodeLegacyIDs(*p.ActionGroupIDs)
	}
	encodeLegacyIDs(p.TagIDs)
	// rt and dl are rewritten on the same assumption as the fields above —
	// that they hold object identifiers — but nothing in this package
	// resolves them against state, so unlike ar/pr/agr/tg the assumption is
	// unverified against Things' own database.
	if p.RecurrenceTaskIDs != nil {
		encodeLegacyIDs(*p.RecurrenceTaskIDs)
	}
	if p.DelegateIDs != nil {
		encodeLegacyIDs(*p.DelegateIDs)
	}
}

func (s *State) updateTask(item things.TaskActionItem) *things.Task {
	t, ok := s.Tasks[item.UUID()]
	if !ok {
		t = &things.Task{
			Schedule: things.TaskScheduleAnytime,
		}
	}
	t.UUID = item.UUID()

	if item.P.Title != nil {
		t.Title = *item.P.Title
	}
	if item.P.Type != nil {
		t.Type = *item.P.Type
	}
	if item.P.Status != nil {
		t.Status = *item.P.Status
	}
	if item.P.Index != nil {
		t.Index = *item.P.Index
	}
	if item.P.InTrash != nil {
		t.InTrash = *item.P.InTrash
	}
	if item.P.Schedule != nil {
		t.Schedule = *item.P.Schedule
	}
	if item.P.ScheduledDate != nil {
		t.ScheduledDate = item.P.ScheduledDate.Time()
	}
	if item.P.CompletionDate != nil {
		t.CompletionDate = item.P.CompletionDate.Time()
	}
	if item.P.DeadlineDate != nil {
		t.DeadlineDate = item.P.DeadlineDate.Time()
	}
	if item.P.CreationDate != nil {
		cd := item.P.CreationDate.Time()
		t.CreationDate = *cd
	}
	if item.P.ModificationDate != nil {
		t.ModificationDate = item.P.ModificationDate.Time()
	}
	if item.P.AreaIDs != nil {
		ids := *item.P.AreaIDs
		t.AreaIDs = ids
	}
	if item.P.ActionGroupIDs != nil {
		ids := *item.P.ActionGroupIDs
		t.ActionGroupIDs = ids
	}
	if item.P.ParentTaskIDs != nil {
		ids := *item.P.ParentTaskIDs
		t.ParentTaskIDs = ids
	}
	if item.P.Note != nil {
		var noteStr string
		if err := json.Unmarshal(item.P.Note, &noteStr); err == nil {
			t.Note = noteStr
		} else {
			var note things.Note
			if err := json.Unmarshal(item.P.Note, &note); err == nil {
				switch note.Type {
				case things.NoteTypeFullText:
					t.Note = note.Value
				case things.NoteTypeDelta:
					t.Note = things.ApplyPatches(t.Note, note.Patches)
				}
			}
		}
	}
	if item.P.AlarmTimeOffset != nil {
		t.AlarmTimeOffset = item.P.AlarmTimeOffset
	}
	if item.P.TagIDs != nil {
		t.TagIDs = item.P.TagIDs
	}
	if item.P.DueOrder != nil {
		t.DueOrder = *item.P.DueOrder
	}
	if item.P.TaskIndex != nil {
		t.TodayIndex = *item.P.TaskIndex
	}
	if item.P.DelegateIDs != nil {
		t.DelegateIDs = *item.P.DelegateIDs
	}
	if item.P.RecurrenceTaskIDs != nil {
		t.RecurrenceIDs = *item.P.RecurrenceTaskIDs
	}

	return t
}

func (s *State) updateCheckListItem(item things.CheckListActionItem) *things.CheckListItem {
	c, ok := s.CheckListItems[item.UUID()]
	if !ok {
		c = &things.CheckListItem{}
	}
	c.UUID = item.UUID()

	if item.P.CreationDate != nil {
		t := item.P.CreationDate.Time()
		c.CreationDate = *t
	}
	if item.P.ModificationDate != nil {
		c.ModificationDate = item.P.ModificationDate.Time()
	}
	if item.P.Index != nil {
		c.Index = *item.P.Index
	}
	if item.P.Title != nil {
		c.Title = *item.P.Title
	}
	if item.P.Status != nil {
		c.Status = *item.P.Status
	}
	if item.P.TaskIDs != nil {
		ids := *item.P.TaskIDs
		c.TaskIDs = ids
	}

	return c
}

func (s *State) updateArea(item things.AreaActionItem) *things.Area {
	a, ok := s.Areas[item.UUID()]
	if !ok {
		a = &things.Area{}
	}
	a.UUID = item.UUID()

	if item.P.Title != nil {
		a.Title = *item.P.Title
	}

	return a
}

func (s *State) updateTag(item things.TagActionItem) *things.Tag {
	t, ok := s.Tags[item.UUID()]
	if !ok {
		t = &things.Tag{}
	}
	t.UUID = item.UUID()

	if item.P.Title != nil {
		t.Title = *item.P.Title
	}
	if item.P.ShortHand != nil {
		t.ShortHand = *item.P.ShortHand
	}
	if item.P.ParentTagIDs != nil {
		var ids = *item.P.ParentTagIDs
		t.ParentTagIDs = ids
	}

	return t
}

// Update applies all items to update the aggregated state
func (s *State) Update(items ...things.Item) error {
	if err := things.ValidateTaskReadKinds(items); err != nil {
		return err
	}

	// Decode every task create/modify before applying any item. A malformed
	// task payload late in the batch must not leave earlier mutations behind,
	// especially note deltas that would be applied twice on retry.
	taskPayloads := make([]things.TaskReadPayload, len(items))
	decodedTaskPayload := make([]bool, len(items))
	for i, rawItem := range items {
		switch rawItem.Kind {
		case things.ItemKindTask, things.ItemKindTask7, things.ItemKindTask4, things.ItemKindTask3, things.ItemKindTaskPlain:
			if rawItem.Action != things.ItemActionCreated && rawItem.Action != things.ItemActionModified {
				continue
			}
			payload, err := things.DecodeTaskReadPayload(rawItem.P)
			if err != nil {
				return err
			}
			taskPayloads[i] = payload
			decodedTaskPayload[i] = true
		}
	}

	for i, rawItem := range items {
		legacy := isLegacyItemKind(rawItem.Kind)
		// A legacy-kind item whose key already parses as canonical Base58
		// carries a current-generation identifier and must not be re-derived
		// (see encodeLegacyIDs for why this can never skip a genuine legacy
		// identifier).
		if legacy && things.ValidateUUID(rawItem.UUID) != nil {
			rawItem.UUID = things.EncodeLegacyIdentifier(rawItem.UUID)
		}

		switch rawItem.Kind {
		case things.ItemKindTask, things.ItemKindTask7, things.ItemKindTask4, things.ItemKindTask3, things.ItemKindTaskPlain:
			item := things.TaskActionItem{Item: rawItem}
			var payload things.TaskReadPayload
			if decodedTaskPayload[i] {
				payload = taskPayloads[i]
				item.P = payload.TaskActionItemPayload
			}
			if legacy {
				encodeLegacyTaskReferences(&item.P)
			}

			switch item.Action {
			case things.ItemActionCreated:
				fallthrough
			case things.ItemActionModified:
				task := s.updateTask(item)
				payload.ApplyNulls(task)
				s.Tasks[item.UUID()] = task
			case things.ItemActionDeleted:
				delete(s.Tasks, item.UUID())
			default:
				// Unsupported action: skip
			}

		case things.ItemKindChecklistItem, things.ItemKindChecklistItem2, things.ItemKindChecklistItem3:
			item := things.CheckListActionItem{Item: rawItem}
			if err := json.Unmarshal(rawItem.P, &item.P); err != nil {
				continue // Skip unparseable items
			}
			if legacy && item.P.TaskIDs != nil {
				encodeLegacyIDs(*item.P.TaskIDs)
			}

			switch item.Action {
			case things.ItemActionCreated:
				fallthrough
			case things.ItemActionModified:
				s.CheckListItems[item.UUID()] = s.updateCheckListItem(item)
			case things.ItemActionDeleted:
				delete(s.CheckListItems, item.UUID())
			default:
				// Unsupported action: skip
			}

		case itemKindArea2, things.ItemKindArea3, things.ItemKindAreaPlain:
			item := things.AreaActionItem{Item: rawItem}
			if err := json.Unmarshal(rawItem.P, &item.P); err != nil {
				continue // Skip unparseable items
			}
			// updateArea currently discards tg, but rewrite it anyway so a
			// future consumer of area tags sees consistent keys.
			if legacy {
				encodeLegacyIDs(item.P.TagIDs)
			}

			switch item.Action {
			case things.ItemActionCreated:
				fallthrough
			case things.ItemActionModified:
				s.Areas[item.UUID()] = s.updateArea(item)

			case things.ItemActionDeleted:
				delete(s.Areas, item.UUID())
			default:
				// Unsupported action: skip
			}

		case things.ItemKindTag, things.ItemKindTag4, things.ItemKindTagPlain:
			item := things.TagActionItem{Item: rawItem}
			if err := json.Unmarshal(rawItem.P, &item.P); err != nil {
				continue // Skip unparseable items
			}
			if legacy && item.P.ParentTagIDs != nil {
				encodeLegacyIDs(*item.P.ParentTagIDs)
			}

			switch item.Action {
			case things.ItemActionCreated:
				fallthrough
			case things.ItemActionModified:
				s.Tags[item.UUID()] = s.updateTag(item)
			case things.ItemActionDeleted:
				delete(s.Tags, item.UUID())
			default:
				// Unsupported action: skip
			}

		case things.ItemKindTombstone, things.ItemKindTombstonePlain:
			// Plain Tombstone items were ignored before the legacy-identifier
			// remapping: they record pre-migration deletes, so they must
			// delete objects just like Tombstone2.
			item := things.TombstoneActionItem{Item: rawItem}
			if err := json.Unmarshal(rawItem.P, &item.P); err != nil {
				continue
			}
			oid := item.P.DeletedObjectID
			if legacy && things.ValidateUUID(oid) != nil {
				oid = things.EncodeLegacyIdentifier(oid)
			}
			delete(s.Tasks, oid)
			delete(s.Areas, oid)
			delete(s.Tags, oid)
			delete(s.CheckListItems, oid)

		default:
			// Unsupported kind: skip
		}
	}
	return nil
}

// Projects returns all projects for this history
func (s *State) Projects() []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Type != things.TaskTypeProject {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// Subtasks returns tasks grouped together with under a root task
func (s *State) Subtasks(root *things.Task, opts ListOption) []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Status == things.TaskStatusCompleted && opts.ExcludeCompleted {
			continue
		}
		if task == root {
			continue
		}
		if task.InTrash && opts.ExcludeInTrash {
			continue
		}
		isChild := false
		for _, taskID := range task.ParentTaskIDs {
			isChild = isChild || taskID == root.UUID
		}
		if isChild {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Index < tasks[j].Index
	})
	return tasks
}

func hasArea(task *things.Task, state *State) bool {
	if task == nil {
		return false
	}
	if len(task.AreaIDs) != 0 {
		return true
	}
	if len(task.ParentTaskIDs) == 0 {
		return false
	}
	for _, taskID := range task.ParentTaskIDs {
		if hasArea(state.Tasks[taskID], state) {
			return true
		}
	}
	return false
}

// TasksWithoutArea looks up tasks not assigned to any area, directly or
// through their parent chain (a task in a project that lives in an area
// inherits that area).
func (s *State) TasksWithoutArea() []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Status == things.TaskStatusCompleted {
			continue
		}
		if task.InTrash {
			continue
		}
		if !hasArea(task, s) {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Index < tasks[j].Index
	})
	return tasks
}

// AreaByName returns an Area if the name matches
func (s *State) AreaByName(name string) *things.Area {
	for _, area := range s.Areas {
		if area.Title == name {
			return area
		}
	}
	return nil
}

// ProjectByName returns an project if the name matches
func (s *State) ProjectByName(name string) *things.Task {
	for _, task := range s.Tasks {
		if task.Type != things.TaskTypeProject {
			continue
		}
		if task.Title == name {
			return task
		}
	}
	return nil
}

// ListOption allows the result set to be filtered
type ListOption struct {
	ExcludeCompleted bool
	ExcludeInTrash   bool
}

// TasksByArea returns tasks associated with a given area
func (s *State) TasksByArea(area *things.Area, opts ListOption) []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Status == things.TaskStatusCompleted && opts.ExcludeCompleted {
			continue
		}
		if task.InTrash && opts.ExcludeInTrash {
			continue
		}
		isChild := false
		for _, areaID := range task.AreaIDs {
			isChild = isChild || areaID == area.UUID
		}
		if isChild {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Index < tasks[j].Index
	})
	return tasks
}

// CheckListItemsByTask returns check lists associated with a particular item
func (s *State) CheckListItemsByTask(task *things.Task, opts ListOption) []*things.CheckListItem {
	items := []*things.CheckListItem{}
	for _, item := range s.CheckListItems {
		if item.Status == things.TaskStatusCompleted && opts.ExcludeCompleted {
			continue
		}
		isChild := false
		for _, taskID := range item.TaskIDs {
			isChild = isChild || task.UUID == taskID
		}
		if isChild {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Index < items[j].Index
	})
	return items
}

// Headings returns all headings within a project
func (s *State) Headings(projectID string) []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Type != things.TaskTypeHeading {
			continue
		}
		for _, pid := range task.ParentTaskIDs {
			if pid == projectID {
				tasks = append(tasks, task)
				break
			}
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Index < tasks[j].Index
	})
	return tasks
}

// TasksByHeading returns tasks assigned to a specific heading (action group)
func (s *State) TasksByHeading(headingID string, opts ListOption) []*things.Task {
	tasks := []*things.Task{}
	for _, task := range s.Tasks {
		if task.Type != things.TaskTypeTask {
			continue
		}
		if task.Status == things.TaskStatusCompleted && opts.ExcludeCompleted {
			continue
		}
		if task.InTrash && opts.ExcludeInTrash {
			continue
		}
		for _, agr := range task.ActionGroupIDs {
			if agr == headingID {
				tasks = append(tasks, task)
				break
			}
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Index < tasks[j].Index
	})
	return tasks
}

// SubTags returns all child tags for a given root, ensuring sort order is kept intact
func (s *State) SubTags(root *things.Tag) []*things.Tag {
	children := []*things.Tag{}
	for _, tag := range s.Tags {
		if tag == root {
			continue
		}

		isChild := false
		for _, parentID := range tag.ParentTagIDs {
			isChild = isChild || parentID == root.UUID
		}
		if isChild {
			children = append(children, tag)
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].ShortHand < children[j].ShortHand
	})
	return children
}
