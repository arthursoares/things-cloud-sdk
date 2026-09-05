package sync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func TestTask7ReadAndNulls(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	apply := func(kind, payload string) {
		t.Helper()
		if _, err := s.processItems([]things.Item{{UUID: "task", Kind: things.ItemKind(kind), P: json.RawMessage(payload)}}, 0); err != nil {
			t.Fatal(err)
		}
	}
	apply("Task6", `{"tt":"before","nt":"note","sr":100,"tir":200,"dd":300,"ato":10,"ar":["area"],"pr":["project"],"agr":["heading"],"tg":["tag"]}`)
	old, err := s.getTask("task")
	if err != nil {
		t.Fatal(err)
	}
	if old.ScheduledDate == nil || old.ScheduledDate.Unix() != 100 {
		t.Fatalf("sr was replaced by tir: %+v", old.ScheduledDate)
	}
	apply("Task7", `{"tt":"after"}`)
	got, err := s.getTask("task")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "after" || got.Note != "note" || got.ScheduledDate == nil || len(got.TagIDs) != 1 {
		t.Fatalf("partial Task7 update: %+v", got)
	}
	apply("Task7", `{"sr":null,"dd":null,"ato":null,"ar":null,"pr":null,"agr":null,"tg":null,"nt":null}`)
	got, err = s.getTask("task")
	if err != nil {
		t.Fatal(err)
	}
	if got.ScheduledDate != nil || got.DeadlineDate != nil || got.AlarmTimeOffset != nil || len(got.AreaIDs)+len(got.ParentTaskIDs)+len(got.ActionGroupIDs)+len(got.TagIDs) != 0 || got.Note != "" {
		t.Fatalf("nulls not cleared: %+v", got)
	}
	if _, err := s.processItems([]things.Item{{UUID: "task", Kind: "Task7", Action: things.ItemActionDeleted}}, 1); err != nil {
		t.Fatal(err)
	}
	got, err = s.getTask("task")
	if err != nil || got != nil {
		t.Fatalf("delete: %+v %v", got, err)
	}
}

func TestTask7ReplayFailurePreservesStateAndRetry(t *testing.T) {
	for _, failure := range []string{"fetch", "decode", "note", "unknown", "empty", "cursor", "install", "concurrent"} {
		t.Run(failure, func(t *testing.T) {
			var retry atomic.Bool
			var s *Syncer
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/items") {
					fmt.Fprint(w, `{"latest-server-index":2}`)
					return
				}
				if r.URL.Query().Get("start-index") == "0" {
					fmt.Fprint(w, `{"items":[{"task":{"e":"Task6","t":0,"p":{"tt":"staged","nt":"there"}}}],"current-item-index":2}`)
					return
				}
				if !retry.Load() {
					switch failure {
					case "fetch":
						w.WriteHeader(http.StatusForbidden)
						return
					case "decode":
						fmt.Fprint(w, `{"items":[{"private-uuid":{"e":"Task7","t":1,"p":17}}],"current-item-index":2}`)
						return
					case "note":
						fmt.Fprint(w, `{"items":[{"private-uuid":{"e":"Task7","t":1,"p":{"nt":42}}}],"current-item-index":2}`)
						return
					case "unknown":
						fmt.Fprint(w, `{"items":[{"private-uuid":{"e":"Task8","t":1,"p":{"tt":"private-title"}}}],"current-item-index":2}`)
						return
					case "empty":
						fmt.Fprint(w, `{"items":[],"current-item-index":2}`)
						return
					case "cursor":
						fmt.Fprint(w, `{"items":[{}],"current-item-index":1}`)
						return
					case "concurrent":
						// A separate SQLite connection advances the live state while
						// network replay is in progress; installation must reject it.
						if _, err := s.rawDB.Exec(`UPDATE sync_state SET server_index=3 WHERE id=1`); err != nil {
							t.Error(err)
						}
					}
				}
				fmt.Fprint(w, `{"items":[{"task":{"e":"Task7","t":1,"p":{"tt":"recovered","nt":{"t":2,"ps":[{"p":0,"l":0,"r":"Hi "}]}}}}],"current-item-index":2}`)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "live.db")
			s = legacyReplayDB(t, path, things.New(server.URL, "test@example.com", "password"), 2)
			defer s.Close()
			before := replayLog(t, s)
			if failure == "install" {
				if _, err := s.db.Exec(`CREATE TRIGGER fail_replay BEFORE DELETE ON tasks BEGIN SELECT RAISE(ABORT, 'injected install failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			changes, err := s.Sync()
			if err == nil || len(changes) != 0 {
				t.Fatalf("failure accepted: %v %v", changes, err)
			}
			if (failure == "unknown" || failure == "decode" || failure == "note") && strings.Contains(err.Error(), "private-") {
				t.Fatalf("private error: %v", err)
			}
			old, getErr := s.getTask("task")
			if getErr != nil || old.Title != "old" || old.Note != "Hi there" {
				t.Fatalf("old state damaged: %+v %v", old, getErr)
			}
			wantIndex := 2
			if failure == "concurrent" {
				wantIndex = 3
			}
			generation, getErr := s.taskReplayVersion()
			if getErr != nil || generation != 1 || s.LastSyncedIndex() != wantIndex || !reflect.DeepEqual(before, replayLog(t, s)) {
				t.Fatalf("metadata/audit changed: gen=%d idx=%d err=%v", generation, s.LastSyncedIndex(), getErr)
			}
			if failure == "install" {
				if _, err := s.db.Exec(`DROP TRIGGER fail_replay`); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "concurrent" {
				if err := s.saveSyncState("history", 2); err != nil {
					t.Fatal(err)
				}
			}
			retry.Store(true)
			if _, err := s.Sync(); err != nil {
				t.Fatalf("retry: %v", err)
			}
			got, getErr := s.getTask("task")
			if getErr != nil || got.Title != "recovered" || got.Note != "Hi there" {
				t.Fatalf("retry state: %+v %v", got, getErr)
			}
			if !reflect.DeepEqual(before, replayLog(t, s)) {
				t.Fatal("retry duplicated logs")
			}
		})
	}
}

func TestTask7ReplayBackupFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires effective filesystem permission checks")
	}
	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/items") {
			fetches.Add(1)
		}
		fmt.Fprint(w, `{"latest-server-index":2}`)
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "live.db")
	s := legacyReplayDB(t, path, things.New(server.URL, "test@example.com", "password"), 2)
	defer s.Close()
	before := replayLog(t, s)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	}()
	if _, err := s.Sync(); err == nil {
		t.Fatal("backup failure ignored")
	}
	if fetches.Load() != 0 {
		t.Fatal("replay started before backup")
	}
	got, err := s.getTask("task")
	if err != nil || got.Title != "old" || s.LastSyncedIndex() != 2 || !reflect.DeepEqual(before, replayLog(t, s)) {
		t.Fatalf("backup failure damaged state: %+v %v", got, err)
	}
	generation, err := s.taskReplayVersion()
	if err != nil || generation != 1 {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
}

func TestTask7ReplayEmptyOuterBatchesAndGrowth(t *testing.T) {
	var starts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/items") {
			fmt.Fprint(w, `{"latest-server-index":1}`)
			return
		}
		start := r.URL.Query().Get("start-index")
		starts = append(starts, start)
		if start == "0" {
			fmt.Fprint(w, `{"items":[{}],"current-item-index":3}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"task":{"e":"Task7","t":0,"p":{"tt":"grown"}}},{}],"current-item-index":3}`)
	}))
	defer server.Close()
	s := legacyReplayDB(t, filepath.Join(t.TempDir(), "live.db"), things.New(server.URL, "test@example.com", "password"), 1)
	defer s.Close()
	changes, err := s.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || s.LastSyncedIndex() != 3 || !reflect.DeepEqual(starts, []string{"0", "1"}) {
		t.Fatalf("changes=%d cursor=%d starts=%v", len(changes), s.LastSyncedIndex(), starts)
	}
}

func TestTask7ReplayGuardsAllMetadata(t *testing.T) {
	for _, mutation := range []string{`UPDATE sync_state SET history_id='other'`, `UPDATE sync_state SET server_index=3`, `UPDATE task_replay_state SET version=2`} {
		t.Run(mutation, func(t *testing.T) {
			s := legacyReplayDB(t, filepath.Join(t.TempDir(), "live.db"), nil, 2)
			defer s.Close()
			stage, err := Open(":memory:", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer stage.Close()
			stage.history = &things.History{ID: "history"}
			mustSaveTask(t, stage, &things.Task{UUID: "task", Title: "staged"})
			if _, err := s.rawDB.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			if err := s.installTaskReplay(stage, "history", 2, 1, 2); err == nil {
				t.Fatal("stale installation accepted")
			}
			got, err := s.getTask("task")
			if err != nil || got.Title != "old" {
				t.Fatalf("stale state installed: %+v %v", got, err)
			}
		})
	}
}

func TestTaskReplayGenerationFreshAndMemoryBackup(t *testing.T) {
	s, err := Open(":memory:", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	generation, err := s.taskReplayVersion()
	if err != nil || generation != things.TaskReplayVersion {
		t.Fatalf("fresh generation=%d err=%v", generation, err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatalf("memory backup: %v", err)
	}
}

func TestTaskReplayFinalFailureRollsBackEntitiesLogsAndCursor(t *testing.T) {
	s := legacyReplayDB(t, filepath.Join(t.TempDir(), "live.db"), nil, 2)
	defer s.Close()
	before := replayLog(t, s)
	staged, err := Open(":memory:", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	staged.history = &things.History{ID: "history", LoadedServerIndex: 3}
	if _, err := staged.processItems([]things.Item{{UUID: "task", Kind: "Task7", P: json.RawMessage(`{"tt":"staged","tg":["new-tag"]}`)}}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_generation BEFORE UPDATE ON task_replay_state BEGIN SELECT RAISE(ABORT, 'injected final failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.installTaskReplay(staged, "history", 2, 1, 3); err == nil {
		t.Fatal("final failure ignored")
	}
	got, err := s.getTask("task")
	if err != nil || got.Title != "old" || len(got.TagIDs) != 0 || s.LastSyncedIndex() != 2 || !reflect.DeepEqual(before, replayLog(t, s)) {
		t.Fatalf("partial install: %+v %v", got, err)
	}
	generation, err := s.taskReplayVersion()
	if err != nil || generation != 1 {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
}

func TestUnknownTaskRollsBackSanitized(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.processItems([]things.Item{{UUID: "valid", Kind: "Task6", P: json.RawMessage(`{"tt":"valid"}`)}, {UUID: "private-uuid", Kind: "Task8", P: json.RawMessage(`{"tt":"private-title"}`)}}, 0)
	if err == nil {
		t.Fatal("unsupported task accepted")
	}
	if strings.Contains(err.Error(), "private-") {
		t.Fatalf("private data in error: %v", err)
	}
	got, getErr := s.getTask("valid")
	if getErr != nil || got != nil {
		t.Fatalf("partial batch committed: %+v %v", got, getErr)
	}
}

// legacyReplayDB simulates the previous reader after it saved a Task6 delta
// and skipped a later Task7 update, including its original audit records.
func legacyReplayDB(t *testing.T, path string, client *things.Client, cursor int) *Syncer {
	t.Helper()
	s, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustSaveTask(t, s, &things.Task{UUID: "task", Title: "old", Note: "Hi there"})
	if err := s.saveSyncState("history", cursor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO change_log(id,server_index,synced_at,change_type,entity_type,entity_uuid,payload) VALUES(41,1,123,'TaskNoteChanged','Task','task','{"original":true}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS task_replay_state`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, client)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func replayLog(t *testing.T, s *Syncer) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT id,server_index,synced_at,change_type,entity_type,entity_uuid,payload FROM change_log ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var id, idx, ts int
		var kind, entity, uuid, payload string
		if err := rows.Scan(&id, &idx, &ts, &kind, &entity, &uuid, &payload); err != nil {
			t.Fatal(err)
		}
		result = append(result, fmt.Sprint(id, "|", idx, "|", ts, "|", kind, "|", entity, "|", uuid, "|", payload))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTask7ReplayPreservesAuditAndReopens(t *testing.T) {
	for _, cutoff := range []int{3, 4} {
		t.Run(fmt.Sprint(cutoff), func(t *testing.T) {
			var starts []string
			heads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/items") {
					heads++
					fmt.Fprint(w, `{"latest-server-index":4}`)
					return
				}
				starts = append(starts, r.URL.Query().Get("start-index"))
				fmt.Fprint(w, `{"items":[{"task":{"e":"Task6","t":0,"p":{"tt":"old","nt":"there"}}},{"task":{"e":"Task6","t":1,"p":{"nt":{"t":2,"ps":[{"p":0,"l":0,"r":"Hi "}]}}}},{"task":{"e":"Task7","t":1,"p":{"tt":"recovered"}}},{"task":{"e":"Task7","t":1,"p":{"tt":"new"}}}],"current-item-index":4}`)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "live.db")
			client := things.New(server.URL, "test@example.com", "password")
			s := legacyReplayDB(t, path, client, cutoff)
			before := replayLog(t, s)
			old, _ := s.getTask("task")
			if old.Title != "old" || s.LastSyncedIndex() != cutoff {
				t.Fatal("Open rebuilt offline")
			}
			changes, err := s.Sync()
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.getTask("task")
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != "new" || got.Note != "Hi there" {
				t.Fatalf("replay state: %+v", got)
			}
			after := replayLog(t, s)
			if !reflect.DeepEqual(before, after[:len(before)]) {
				t.Fatalf("audit changed: %v -> %v", before, after)
			}
			if len(changes) != 4-cutoff || len(after) != len(before)+4-cutoff {
				t.Fatalf("cutoff changes=%d logs=%d", len(changes), len(after))
			}
			if s.LastSyncedIndex() != 4 || !reflect.DeepEqual(starts, []string{"0"}) || heads != 1 {
				t.Fatalf("cursor=%d starts=%v heads=%d", s.LastSyncedIndex(), starts, heads)
			}
			backups, err := filepath.Glob(path + ".task7-backup-*")
			if err != nil || len(backups) != 1 {
				t.Fatalf("backups: %v %v", backups, err)
			}
			info, err := os.Stat(backups[0])
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("backup permissions: %v %v", info, err)
			}
			backup, err := sql.Open("sqlite", backups[0])
			if err != nil {
				t.Fatal(err)
			}
			var backupTitle string
			if err := backup.QueryRow(`SELECT title FROM tasks WHERE uuid='task'`).Scan(&backupTitle); err != nil {
				t.Fatal(err)
			}
			if backupTitle != "old" || !reflect.DeepEqual(before, replayLog(t, &Syncer{db: backup})) {
				t.Fatal("backup did not retain pre-recovery WAL state")
			}
			if err := backup.Close(); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = Open(path, client)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			changes, err = s.Sync()
			if err != nil || len(changes) != 0 {
				t.Fatalf("repeat: %v %v", changes, err)
			}
			if !reflect.DeepEqual(after, replayLog(t, s)) || len(starts) != 1 {
				t.Fatal("repeat replayed history")
			}
		})
	}
}
