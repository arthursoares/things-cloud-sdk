// Package sync provides a persistent sync engine for Things Cloud.
// It stores state in SQLite and surfaces semantic change events.
package sync

import (
	"database/sql"
	"strings"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	_ "modernc.org/sqlite"
)

const (
	maxRetries    = 3
	retryBaseWait = 2 * time.Second
)

// dbExecutor is the interface for database operations (satisfied by *sql.DB and *sql.Tx)
type dbExecutor interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// Syncer manages persistent sync with Things Cloud
type Syncer struct {
	rawDB   *sql.DB    // underlying connection for Close() and Begin()
	db      dbExecutor // current executor (db or tx)
	client  *things.Client
	history *things.History
}

// Open creates or opens a sync database and connects to Things Cloud
func Open(dbPath string, client *things.Client) (*Syncer, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	s := &Syncer{
		rawDB:  db,
		db:     db,
		client: client,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the database connection
func (s *Syncer) Close() error {
	return s.rawDB.Close()
}

// isRetryableError returns true if the error is a temporary server error worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504")
}

// Sync fetches new items from Things Cloud, updates local state,
// and returns the list of changes in order
func (s *Syncer) Sync() ([]Change, error) {
	// Get current sync state first
	storedHistoryID, startIndex, err := s.getSyncState()
	if err != nil {
		return nil, err
	}

	// Reuse stored history ID if available (avoids extra /account call)
	// Things.app does this - it goes straight to /items with the stored history ID
	if s.history == nil {
		if storedHistoryID != "" {
			// Use stored history ID directly - no network call needed
			s.history = s.client.HistoryWithID(storedHistoryID)
		} else {
			// First sync - need to fetch history ID from server
			var h *things.History
			var err error
			for attempt := 0; attempt < maxRetries; attempt++ {
				h, err = s.client.OwnHistory()
				if err == nil {
					break
				}
				if !isRetryableError(err) {
					return nil, err
				}
				time.Sleep(retryBaseWait * time.Duration(1<<attempt))
			}
			if err != nil {
				return nil, err
			}
			s.history = h
		}
	}

	// If history changed (shouldn't happen normally), start fresh
	if storedHistoryID != "" && storedHistoryID != s.history.ID {
		startIndex = 0
	}

	// Pre-check: Get latest server index to avoid out-of-bounds requests
	// A 500 error occurs when start-index > server's current-item-index
	serverIndex, err := s.getServerIndex()
	if err != nil {
		return nil, err
	}

	// If our cursor is already at or beyond the server's index, nothing to fetch
	if startIndex >= serverIndex {
		return nil, nil
	}

	// Fetch items from server
	var allChanges []Change
	hasMore := true

	for hasMore {
		// Fetch with retry for transient errors
		var items []things.Item
		var more bool
		var fetchErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			items, more, fetchErr = s.history.Items(things.ItemsOptions{StartIndex: startIndex})
			if fetchErr == nil {
				break
			}
			if !isRetryableError(fetchErr) {
				return nil, fetchErr
			}
			time.Sleep(retryBaseWait * time.Duration(1<<attempt))
		}
		if fetchErr != nil {
			return nil, fetchErr
		}

		// No items returned means we're caught up
		if len(items) == 0 {
			break
		}

		// Process each item
		changes, err := s.processItems(items, startIndex)
		if err != nil {
			return nil, err
		}
		allChanges = append(allChanges, changes...)

		// Use loaded server item-batches as the next start position.
		// The expanded item count can differ from the server batch count.
		startIndex = s.history.LoadedServerIndex
		hasMore = more
	}

	// Save sync state
	if err := s.saveSyncState(s.history.ID, s.history.LatestServerIndex); err != nil {
		return nil, err
	}

	return allChanges, nil
}

// LastSyncedIndex returns the server index we've synced up to
func (s *Syncer) LastSyncedIndex() int {
	_, idx, _ := s.getSyncState()
	return idx
}

// ChangesSince returns changes that occurred after the given timestamp
func (s *Syncer) ChangesSince(timestamp time.Time) ([]Change, error) {
	rows, err := s.db.Query(`
		SELECT id, server_index, synced_at, change_type, entity_type, entity_uuid, payload
		FROM change_log
		WHERE synced_at > ?
		ORDER BY id
	`, timestamp.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanChangeLog(rows)
}

// ChangesSinceIndex returns changes that occurred after the given server index
func (s *Syncer) ChangesSinceIndex(serverIndex int) ([]Change, error) {
	rows, err := s.db.Query(`
		SELECT id, server_index, synced_at, change_type, entity_type, entity_uuid, payload
		FROM change_log
		WHERE server_index > ?
		ORDER BY id
	`, serverIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanChangeLog(rows)
}

// ChangesForEntity returns all changes for a specific entity
func (s *Syncer) ChangesForEntity(entityUUID string) ([]Change, error) {
	rows, err := s.db.Query(`
		SELECT id, server_index, synced_at, change_type, entity_type, entity_uuid, payload
		FROM change_log
		WHERE entity_uuid = ?
		ORDER BY id
	`, entityUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanChangeLog(rows)
}

func (s *Syncer) scanChangeLog(rows *sql.Rows) ([]Change, error) {
	var changes []Change

	for rows.Next() {
		var id int
		var serverIndex int
		var syncedAt int64
		var changeType, entityType, entityUUID string
		var payload sql.NullString

		if err := rows.Scan(&id, &serverIndex, &syncedAt, &changeType, &entityType, &entityUUID, &payload); err != nil {
			return nil, err
		}

		base := baseChange{
			serverIndex: serverIndex,
			timestamp:   time.Unix(syncedAt, 0),
		}

		// Return UnknownChange with the change type as details
		// A more complete implementation would reconstruct full typed changes
		changes = append(changes, UnknownChange{
			baseChange: base,
			entityType: entityType,
			entityUUID: entityUUID,
			Details:    changeType,
		})
	}

	return changes, nil
}

// getServerIndex fetches the latest server index from Things Cloud.
// This is used to pre-check before fetching items to avoid 500 errors
// when our stored cursor is ahead of the server's current-item-index.
func (s *Syncer) getServerIndex() (int, error) {
	h, err := s.client.History(s.history.ID)
	if err != nil {
		return 0, err
	}
	return h.LatestServerIndex, nil
}
