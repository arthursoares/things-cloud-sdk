package sync

import (
	"database/sql"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// getTask retrieves a task by UUID from the database.
// Returns nil, nil if the task is not found.
func (s *Syncer) getTask(uuid string) (*things.Task, error) {
	row := s.db.QueryRow(`
		SELECT
			uuid, type, title, note, status, schedule,
			scheduled_date, deadline_date, completion_date, creation_date, modification_date,
			"index", today_index, in_trash, area_uuid, project_uuid, heading_uuid,
			alarm_time_offset, recurrence_rule, deleted
		FROM tasks
		WHERE uuid = ? AND deleted = 0
	`, uuid)

	var (
		t                things.Task
		taskType         int
		status           int
		schedule         int
		scheduledDate    sql.NullInt64
		deadlineDate     sql.NullInt64
		completionDate   sql.NullInt64
		creationDate     sql.NullInt64
		modificationDate sql.NullInt64
		inTrash          int
		areaUUID         sql.NullString
		projectUUID      sql.NullString
		headingUUID      sql.NullString
		alarmTimeOffset  sql.NullInt64
		recurrenceRule   sql.NullString
		deleted          int
	)

	err := row.Scan(
		&t.UUID, &taskType, &t.Title, &t.Note, &status, &schedule,
		&scheduledDate, &deadlineDate, &completionDate, &creationDate, &modificationDate,
		&t.Index, &t.TodayIndex, &inTrash, &areaUUID, &projectUUID, &headingUUID,
		&alarmTimeOffset, &recurrenceRule, &deleted,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Convert types
	t.Type = things.TaskType(taskType)
	t.Status = things.TaskStatus(status)
	t.Schedule = things.TaskSchedule(schedule)
	t.InTrash = inTrash == 1

	// Convert nullable timestamps
	if scheduledDate.Valid {
		ts := time.Unix(scheduledDate.Int64, 0).UTC()
		t.ScheduledDate = &ts
	}
	if deadlineDate.Valid {
		ts := time.Unix(deadlineDate.Int64, 0).UTC()
		t.DeadlineDate = &ts
	}
	if completionDate.Valid {
		ts := time.Unix(completionDate.Int64, 0).UTC()
		t.CompletionDate = &ts
	}
	if creationDate.Valid {
		t.CreationDate = time.Unix(creationDate.Int64, 0).UTC()
	}
	if modificationDate.Valid {
		ts := time.Unix(modificationDate.Int64, 0).UTC()
		t.ModificationDate = &ts
	}

	// Convert nullable foreign keys to slices
	if areaUUID.Valid && areaUUID.String != "" {
		t.AreaIDs = []string{areaUUID.String}
	}
	if projectUUID.Valid && projectUUID.String != "" {
		t.ParentTaskIDs = []string{projectUUID.String}
	}
	if headingUUID.Valid && headingUUID.String != "" {
		t.ActionGroupIDs = []string{headingUUID.String}
	}

	// Convert nullable alarm time offset
	if alarmTimeOffset.Valid {
		offset := int(alarmTimeOffset.Int64)
		t.AlarmTimeOffset = &offset
	}

	// Load tags from junction table
	rows, err := s.db.Query(`SELECT tag_uuid FROM task_tags WHERE task_uuid = ?`, uuid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tagUUID string
		if err := rows.Scan(&tagUUID); err != nil {
			return nil, err
		}
		t.TagIDs = append(t.TagIDs, tagUUID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &t, nil
}

// saveTask inserts or updates a task in the database.
func (s *Syncer) saveTask(t *things.Task) error {
	// Convert nullable timestamps to Unix integers
	var scheduledDate, deadlineDate, completionDate, creationDate, modificationDate sql.NullInt64

	if t.ScheduledDate != nil {
		scheduledDate = sql.NullInt64{Int64: t.ScheduledDate.Unix(), Valid: true}
	}
	if t.DeadlineDate != nil {
		deadlineDate = sql.NullInt64{Int64: t.DeadlineDate.Unix(), Valid: true}
	}
	if t.CompletionDate != nil {
		completionDate = sql.NullInt64{Int64: t.CompletionDate.Unix(), Valid: true}
	}
	if !t.CreationDate.IsZero() {
		creationDate = sql.NullInt64{Int64: t.CreationDate.Unix(), Valid: true}
	}
	if t.ModificationDate != nil {
		modificationDate = sql.NullInt64{Int64: t.ModificationDate.Unix(), Valid: true}
	}

	// Convert foreign key slices to single values
	var areaUUID, projectUUID, headingUUID sql.NullString

	if len(t.AreaIDs) > 0 && t.AreaIDs[0] != "" {
		areaUUID = sql.NullString{String: t.AreaIDs[0], Valid: true}
	}
	if len(t.ParentTaskIDs) > 0 && t.ParentTaskIDs[0] != "" {
		projectUUID = sql.NullString{String: t.ParentTaskIDs[0], Valid: true}
	}
	if len(t.ActionGroupIDs) > 0 && t.ActionGroupIDs[0] != "" {
		headingUUID = sql.NullString{String: t.ActionGroupIDs[0], Valid: true}
	}

	// Convert nullable alarm time offset
	var alarmTimeOffset sql.NullInt64
	if t.AlarmTimeOffset != nil {
		alarmTimeOffset = sql.NullInt64{Int64: int64(*t.AlarmTimeOffset), Valid: true}
	}

	// Convert InTrash to integer
	var inTrash int
	if t.InTrash {
		inTrash = 1
	}

	// Insert or replace the task
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO tasks (
			uuid, type, title, note, status, schedule,
			scheduled_date, deadline_date, completion_date, creation_date, modification_date,
			"index", today_index, in_trash, area_uuid, project_uuid, heading_uuid,
			alarm_time_offset, recurrence_rule, deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`,
		t.UUID, int(t.Type), t.Title, t.Note, int(t.Status), int(t.Schedule),
		scheduledDate, deadlineDate, completionDate, creationDate, modificationDate,
		t.Index, t.TodayIndex, inTrash, areaUUID, projectUUID, headingUUID,
		alarmTimeOffset, sql.NullString{}, // recurrence_rule not directly on Task struct
	)
	if err != nil {
		return err
	}

	// Delete and re-insert task_tags entries
	_, err = s.db.Exec(`DELETE FROM task_tags WHERE task_uuid = ?`, t.UUID)
	if err != nil {
		return err
	}

	for _, tagID := range t.TagIDs {
		_, err = s.db.Exec(`INSERT INTO task_tags (task_uuid, tag_uuid) VALUES (?, ?)`, t.UUID, tagID)
		if err != nil {
			return err
		}
	}

	return nil
}

// markTaskDeleted soft-deletes a task by setting its deleted flag to 1.
func (s *Syncer) markTaskDeleted(uuid string) error {
	_, err := s.db.Exec(`UPDATE tasks SET deleted = 1 WHERE uuid = ?`, uuid)
	return err
}

// getArea retrieves an area by UUID from the database.
// Returns nil, nil if the area is not found or is deleted.
func (s *Syncer) getArea(uuid string) (*things.Area, error) {
	row := s.db.QueryRow(`
		SELECT uuid, title
		FROM areas
		WHERE uuid = ? AND deleted = 0
	`, uuid)

	var a things.Area
	err := row.Scan(&a.UUID, &a.Title)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// saveArea inserts or updates an area in the database.
func (s *Syncer) saveArea(a *things.Area) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO areas (uuid, title, deleted)
		VALUES (?, ?, 0)
	`, a.UUID, a.Title)
	return err
}

// markAreaDeleted soft-deletes an area by setting its deleted flag to 1.
func (s *Syncer) markAreaDeleted(uuid string) error {
	_, err := s.db.Exec(`UPDATE areas SET deleted = 1 WHERE uuid = ?`, uuid)
	return err
}

// getTag retrieves a tag by UUID from the database.
// Returns nil, nil if the tag is not found or is deleted.
func (s *Syncer) getTag(uuid string) (*things.Tag, error) {
	row := s.db.QueryRow(`
		SELECT uuid, title, shortcut, parent_uuid
		FROM tags
		WHERE uuid = ? AND deleted = 0
	`, uuid)

	var (
		t          things.Tag
		shortcut   sql.NullString
		parentUUID sql.NullString
	)

	err := row.Scan(&t.UUID, &t.Title, &shortcut, &parentUUID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if shortcut.Valid {
		t.ShortHand = shortcut.String
	}
	if parentUUID.Valid && parentUUID.String != "" {
		t.ParentTagIDs = []string{parentUUID.String}
	}

	return &t, nil
}

// saveTag inserts or updates a tag in the database.
func (s *Syncer) saveTag(t *things.Tag) error {
	var parentUUID sql.NullString
	if len(t.ParentTagIDs) > 0 && t.ParentTagIDs[0] != "" {
		parentUUID = sql.NullString{String: t.ParentTagIDs[0], Valid: true}
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO tags (uuid, title, shortcut, parent_uuid, deleted)
		VALUES (?, ?, ?, ?, 0)
	`, t.UUID, t.Title, t.ShortHand, parentUUID)
	return err
}

// markTagDeleted soft-deletes a tag by setting its deleted flag to 1.
func (s *Syncer) markTagDeleted(uuid string) error {
	_, err := s.db.Exec(`UPDATE tags SET deleted = 1 WHERE uuid = ?`, uuid)
	return err
}

// getChecklistItem retrieves a checklist item by UUID from the database.
// Returns nil, nil if the checklist item is not found or is deleted.
func (s *Syncer) getChecklistItem(uuid string) (*things.CheckListItem, error) {
	row := s.db.QueryRow(`
		SELECT uuid, task_uuid, title, status, "index", creation_date, completion_date
		FROM checklist_items
		WHERE uuid = ? AND deleted = 0
	`, uuid)

	var (
		c              things.CheckListItem
		taskUUID       sql.NullString
		status         int
		creationDate   sql.NullInt64
		completionDate sql.NullInt64
	)

	err := row.Scan(&c.UUID, &taskUUID, &c.Title, &status, &c.Index, &creationDate, &completionDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	c.Status = things.TaskStatus(status)

	if taskUUID.Valid && taskUUID.String != "" {
		c.TaskIDs = []string{taskUUID.String}
	}
	if creationDate.Valid {
		c.CreationDate = time.Unix(creationDate.Int64, 0).UTC()
	}
	if completionDate.Valid {
		ts := time.Unix(completionDate.Int64, 0).UTC()
		c.CompletionDate = &ts
	}

	return &c, nil
}

// saveChecklistItem inserts or updates a checklist item in the database.
func (s *Syncer) saveChecklistItem(c *things.CheckListItem) error {
	var taskUUID sql.NullString
	if len(c.TaskIDs) > 0 && c.TaskIDs[0] != "" {
		taskUUID = sql.NullString{String: c.TaskIDs[0], Valid: true}
	}

	var creationDate, completionDate sql.NullInt64
	if !c.CreationDate.IsZero() {
		creationDate = sql.NullInt64{Int64: c.CreationDate.Unix(), Valid: true}
	}
	if c.CompletionDate != nil {
		completionDate = sql.NullInt64{Int64: c.CompletionDate.Unix(), Valid: true}
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO checklist_items (uuid, task_uuid, title, status, "index", creation_date, completion_date, deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, c.UUID, taskUUID, c.Title, int(c.Status), c.Index, creationDate, completionDate)
	return err
}

// markChecklistItemDeleted soft-deletes a checklist item by setting its deleted flag to 1.
func (s *Syncer) markChecklistItemDeleted(uuid string) error {
	_, err := s.db.Exec(`UPDATE checklist_items SET deleted = 1 WHERE uuid = ?`, uuid)
	return err
}

// getSyncState retrieves the current sync state from the database.
// Returns "", 0, nil if no sync state exists.
func (s *Syncer) getSyncState() (historyID string, serverIndex int, err error) {
	row := s.db.QueryRow(`
		SELECT history_id, server_index
		FROM sync_state
		WHERE id = 1
	`)

	err = row.Scan(&historyID, &serverIndex)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}

	return historyID, serverIndex, nil
}

// saveSyncState saves the current sync state to the database.
func (s *Syncer) saveSyncState(historyID string, serverIndex int) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO sync_state (id, history_id, server_index, last_sync_at)
		VALUES (1, ?, ?, ?)
	`, historyID, serverIndex, time.Now().Unix())
	return err
}

// logChange records a change in the change log.
func (s *Syncer) logChange(serverIndex int, change Change, payload string) error {
	_, err := s.db.Exec(`
		INSERT INTO change_log (server_index, synced_at, change_type, entity_type, entity_uuid, payload)
		VALUES (?, ?, ?, ?, ?, ?)
	`, serverIndex, time.Now().Unix(), change.ChangeType(), change.EntityType(), change.EntityUUID(), payload)
	return err
}
