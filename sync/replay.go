package sync

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

// Replay semantics evolve independently of the physical schema. Opening an
// old database records its generation without clearing any usable offline data.
func (s *Syncer) ensureTaskReplayState() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS task_replay_state (
		id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL
	)`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO task_replay_state (id, version)
		SELECT 1, CASE WHEN
			EXISTS(SELECT 1 FROM sync_state) OR EXISTS(SELECT 1 FROM tasks) OR
			EXISTS(SELECT 1 FROM areas) OR EXISTS(SELECT 1 FROM tags) OR
			EXISTS(SELECT 1 FROM checklist_items) OR EXISTS(SELECT 1 FROM change_log) OR
			EXISTS(SELECT 1 FROM task_tags) OR EXISTS(SELECT 1 FROM area_tags)
		THEN 1 ELSE ? END`, things.TaskReplayVersion)
	return err
}

func (s *Syncer) taskReplayVersion() (int, error) {
	var version int
	err := s.db.QueryRow(`SELECT version FROM task_replay_state WHERE id = 1`).Scan(&version)
	return version, err
}

// backupForReplay makes a consistent SQLite snapshot including committed WAL
// content. The reserved, exclusive filename is permission restricted before
// SQLite writes any data. Every attempt snapshots the actual current database;
// only a byte-identical, validated retained snapshot can replace the new copy.
// This bounds retained copies across identical retries, including process
// restarts, while protecting externally restored databases with the same cursor.
// Each retry still requires temporary space and I/O for a complete snapshot.
// In-memory/temporary databases have no persistent source file to protect;
// their original state stays in memory until the final atomic installation.
func (s *Syncer) backupForReplay() error {
	rows, err := s.rawDB.Query(`PRAGMA database_list`)
	if err != nil {
		return err
	}
	var path string
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			rows.Close()
			return err
		}
		if name == "main" {
			path = file
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || path == "" {
		return err
	}
	before, err := readReplayBackupState(s.rawDB)
	if err != nil {
		return err
	}
	// Hash the tuple so backup filenames never expose the private history ID.
	key := sha256.Sum256([]byte(fmt.Sprintf("%q:%d:%d", before.historyID, before.cursor, before.generation)))
	prefix := fmt.Sprintf("%s.task7-backup-%x-", filepath.Base(path), key)
	// Capture candidates before creating our file: never deduplicate against
	// ourselves or a younger concurrent attempt that could also be removed.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), prefix+"*")
	if err != nil {
		return err
	}
	backupPath := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	if _, err := s.rawDB.Exec(`VACUUM INTO ?`, backupPath); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	if !validReplayBackup(backupPath, before) {
		// This file was exclusively created by this attempt. Replay has not
		// started, so removing an invalid snapshot cannot remove its safety net.
		_ = os.Remove(backupPath)
		return fmt.Errorf("task replay backup failed integrity or source-state validation")
	}
	currentHash, err := replayBackupHash(backupPath)
	if err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	duplicate := false
	for _, entry := range entries {
		candidate := filepath.Join(filepath.Dir(path), entry.Name())
		if !strings.HasPrefix(entry.Name(), prefix) || !validReplayBackup(candidate, before) {
			continue
		}
		candidateHash, err := replayBackupHash(candidate)
		if err == nil && candidateHash == currentHash {
			duplicate = true
			break
		}
	}
	if err := s.checkReplayBackupState(before); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	if duplicate {
		// Delete only this attempt's exclusive snapshot, after proving a valid
		// retained copy contains every byte. Never remove a previous backup.
		return os.Remove(backupPath)
	}
	return nil
}

func replayBackupHash(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return sum, err
	}
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

type replayBackupState struct {
	historyID          string
	cursor, generation int
}

// One SELECT observes a coherent metadata tuple even if another writer syncs.
func readReplayBackupState(db dbExecutor) (replayBackupState, error) {
	var state replayBackupState
	err := db.QueryRow(`SELECT COALESCE(s.history_id, ''), COALESCE(s.server_index, 0), r.version
		FROM task_replay_state r LEFT JOIN sync_state s ON s.id = r.id WHERE r.id = 1`).Scan(&state.historyID, &state.cursor, &state.generation)
	return state, err
}

func (s *Syncer) checkReplayBackupState(before replayBackupState) error {
	after, err := readReplayBackupState(s.rawDB)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("sync state changed while backing up task replay; retry sync")
	}
	return nil
}

func validReplayBackup(path string, expected replayBackupState) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return false
	}
	// Standalone VACUUM snapshots need no WAL. Read-only immutable mode avoids
	// modifying the retained snapshot or creating journal/SHM sidecar files.
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&immutable=1"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return false
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		return false
	}
	actual, err := readReplayBackupState(db)
	return err == nil && actual == expected
}

func (s *Syncer) recoverTaskReplay(historyID string, cutoff, generation, target int) ([]Change, error) {
	if cutoff < 0 || target < cutoff {
		return nil, fmt.Errorf("cannot recover task replay: server cursor %d is behind stored cursor %d", target, cutoff)
	}
	if err := s.backupForReplay(); err != nil {
		return nil, fmt.Errorf("backing up task replay database: %w", err)
	}
	staged, err := Open(":memory:", s.client)
	if err != nil {
		return nil, fmt.Errorf("opening task replay staging database: %w", err)
	}
	defer staged.Close()
	// Staging never runs nested state queries; pin its private in-memory DB
	// to one connection without restricting live readers of a file database.
	staged.rawDB.SetMaxOpenConns(1)
	staged.history = s.client.HistoryWithID(s.history.ID)
	staged.replayLogCutoff = cutoff
	var changes []Change
	start := 0
	for start < target {
		var items []things.Item
		var more bool
		for attempt := 0; attempt < maxRetries; attempt++ {
			items, more, err = staged.history.Items(things.ItemsOptions{StartIndex: start})
			if err == nil || !isRetryableError(err) {
				break
			}
			time.Sleep(retryBaseWait * time.Duration(1<<attempt))
		}
		if err != nil {
			return nil, fmt.Errorf("fetching task replay: %w", err)
		}
		if err := validateReplayPage(staged.history, start, target); err != nil {
			return nil, err
		}
		batchChanges, err := staged.processItems(items, start)
		if err != nil {
			return nil, fmt.Errorf("replaying task history: %w", err)
		}
		changes = append(changes, batchChanges...)
		start = staged.history.LoadedServerIndex
		// Include growth observed in a page, following complete outer batches.
		if more {
			target = staged.history.LatestServerIndex
		}
	}
	if err := s.installTaskReplay(staged, historyID, cutoff, generation, start); err != nil {
		return nil, fmt.Errorf("installing task replay: %w", err)
	}
	return changes, nil
}

// installTaskReplay preserves the active WAL database and every original audit
// row. Reading the guard and replacing tables in one transaction makes a
// concurrent advance either fail the guard or fail SQLite's write upgrade.
func (s *Syncer) installTaskReplay(staged *Syncer, historyID string, cutoff, generation, next int) error {
	tx, err := s.rawDB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	guard := &Syncer{db: tx}
	actualHistory, actualIndex, err := guard.getSyncState()
	if err != nil {
		return err
	}
	actualGeneration, err := guard.taskReplayVersion()
	if err != nil {
		return err
	}
	if actualHistory != historyID || actualIndex != cutoff || actualGeneration != generation {
		return fmt.Errorf("sync state changed during task replay; retry sync")
	}
	for _, table := range []string{"task_tags", "area_tags", "checklist_items", "tasks", "areas", "tags"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return err
		}
		if err := copyReplayRows(tx, staged.rawDB, table, "*"); err != nil {
			return err
		}
	}
	// Omit staging IDs so SQLite allocates new IDs after the retained history.
	if err := copyReplayRows(tx, staged.rawDB, "change_log", "server_index,synced_at,change_type,entity_type,entity_uuid,payload"); err != nil {
		return err
	}
	if err := guard.saveSyncState(staged.history.ID, next); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE task_replay_state SET version = ? WHERE id = 1`, things.TaskReplayVersion); err != nil {
		return err
	}
	return tx.Commit()
}

// Only fixed internal table/column names reach this row copier. Copying the
// complete stored rows also retains deleted flags and junction relationships.
func copyReplayRows(tx *sql.Tx, staged *sql.DB, table, columns string) error {
	query := `SELECT ` + columns + ` FROM ` + table
	if table == "change_log" {
		query += " ORDER BY id"
	}
	rows, err := staged.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return err
	}
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = `"` + name + `"`
	}
	query = `INSERT INTO ` + table + ` (` + strings.Join(quoted, ",") + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(names)), ",") + `)`
	for rows.Next() {
		values := make([]any, len(names))
		refs := make([]any, len(names))
		for i := range values {
			refs[i] = &values[i]
		}
		if err := rows.Scan(refs...); err != nil {
			return err
		}
		if _, err := tx.Exec(query, values...); err != nil {
			return err
		}
	}
	return rows.Err()
}
