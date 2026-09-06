package sync

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	things "github.com/arthursoares/things-cloud-sdk"
)

func replayBackupFiles(t *testing.T, path string) []string {
	t.Helper()
	files, err := filepath.Glob(path + ".task7-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestReplayBackupReusedAcrossFailedRetriesAndReopen(t *testing.T) {
	for _, failure := range []string{"fetch", "decode"} {
		t.Run(failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/items") {
					fmt.Fprint(w, `{"latest-server-index":2}`)
					return
				}
				if failure == "fetch" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				fmt.Fprint(w, `{"items":[{"task":{"e":"Task7","t":1,"p":17}}],"current-item-index":2}`)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "live.db")
			client := things.New(server.URL, "test@example.com", "password")
			s := legacyReplayDB(t, path, client, 2)
			var original []byte
			for attempt := 0; attempt < 4; attempt++ {
				if _, err := s.Sync(); err == nil {
					t.Fatal("expected replay failure")
				}
				files := replayBackupFiles(t, path)
				if len(files) != 1 {
					t.Fatalf("attempt %d created %d backups", attempt, len(files))
				}
				data, err := os.ReadFile(files[0])
				if err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					original = data
				} else if !bytes.Equal(original, data) {
					t.Fatal("reused backup changed")
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				s, err = Open(path, client)
				if err != nil {
					t.Fatal(err)
				}
			}
			defer s.Close()
			got, err := s.getTask("task")
			if err != nil || got.Title != "old" || s.LastSyncedIndex() != 2 {
				t.Fatalf("failed retries changed state: %+v %v", got, err)
			}
		})
	}
}

func TestReplayBackupNewCursorGetsNewSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	s := legacyReplayDB(t, path, nil, 2)
	defer s.Close()
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	oldFiles := replayBackupFiles(t, path)
	oldBytes, err := os.ReadFile(oldFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := s.saveSyncState("history", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	if files := replayBackupFiles(t, path); len(files) != 2 {
		t.Fatalf("want one snapshot per cursor, got %v", files)
	}
	got, err := os.ReadFile(oldFiles[0])
	if err != nil || !bytes.Equal(oldBytes, got) {
		t.Fatalf("previous backup changed: %v", err)
	}
}

func TestReplayBackupCorruptCandidateIsPreservedAndNotReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	s := legacyReplayDB(t, path, nil, 2)
	defer s.Close()
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	files := replayBackupFiles(t, path)
	corrupt := []byte("user existing or damaged backup")
	if err := os.WriteFile(files[0], corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	if all := replayBackupFiles(t, path); len(all) != 2 {
		t.Fatalf("want corrupt original plus one valid backup, got %v", all)
	}
	got, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(corrupt, got) {
		t.Fatalf("corrupt original overwritten: %v", err)
	}
}

func TestReplayBackupUnsafeCandidatesAreNotReused(t *testing.T) {
	for _, condition := range []string{"metadata", "symlink", "permissions"} {
		t.Run(condition, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "live.db")
			s := legacyReplayDB(t, path, nil, 2)
			defer s.Close()
			if err := s.backupForReplay(); err != nil {
				t.Fatal(err)
			}
			candidate := replayBackupFiles(t, path)[0]
			switch condition {
			case "metadata":
				db, err := sql.Open("sqlite", candidate)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE sync_state SET history_id='other-history'`); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				foreign := filepath.Join(filepath.Dir(path), "foreign.db")
				if err := os.Rename(candidate, foreign); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, candidate); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(candidate, 0644); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.backupForReplay(); err != nil {
				t.Fatal(err)
			}
			if err := s.backupForReplay(); err != nil {
				t.Fatal(err)
			}
			if files := replayBackupFiles(t, path); len(files) != 2 {
				t.Fatalf("unsafe candidate reused or retries unbounded: %v", files)
			}
			after, err := os.ReadFile(candidate)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("unsafe candidate changed: %v", err)
			}
			if condition == "symlink" {
				info, err := os.Lstat(candidate)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("symlink changed: %v", err)
				}
			}
		})
	}
}

func TestReplayBackupSourceGuardRejectsConcurrentAdvance(t *testing.T) {
	for _, mutation := range []string{`UPDATE sync_state SET server_index=3`, `UPDATE sync_state SET history_id='other-history'`, `UPDATE task_replay_state SET version=2`} {
		t.Run(mutation, func(t *testing.T) {
			s := legacyReplayDB(t, filepath.Join(t.TempDir(), "live.db"), nil, 2)
			defer s.Close()
			before, err := readReplayBackupState(s.rawDB)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.rawDB.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			if err := s.checkReplayBackupState(before); err == nil {
				t.Fatal("concurrent source advance accepted")
			}
		})
	}
}

func TestReplayBackupSameTupleChangedAuditGetsCurrentSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	s := legacyReplayDB(t, path, nil, 2)
	defer s.Close()
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	original := replayBackupFiles(t, path)[0]
	originalBytes, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	// A restored/replaced database can have the identical history/cursor/read
	// generation but a different audit trail. Metadata alone cannot identify it.
	if _, err := s.rawDB.Exec(`UPDATE change_log SET id=84, synced_at=456, payload='{"restored":true}' WHERE id=41`); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	files := replayBackupFiles(t, path)
	if len(files) != 2 {
		t.Fatalf("want snapshots of both distinct states, got %v", files)
	}
	for _, file := range files {
		if file == original {
			continue
		}
		db, err := sql.Open("sqlite", file)
		if err != nil {
			t.Fatal(err)
		}
		var id, syncedAt int
		var payload string
		if err := db.QueryRow(`SELECT id, synced_at, payload FROM change_log`).Scan(&id, &syncedAt, &payload); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if id != 84 || syncedAt != 456 || payload != `{"restored":true}` {
			t.Fatalf("backup does not protect current audit: %d %d %s", id, syncedAt, payload)
		}
	}
	after, err := os.ReadFile(original)
	if err != nil || !bytes.Equal(originalBytes, after) {
		t.Fatalf("original backup changed: %v", err)
	}
}

func TestReplayBackupFailurePreservesRetainedSnapshot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires effective filesystem permission checks")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "live.db")
	s := legacyReplayDB(t, path, nil, 2)
	defer s.Close()
	if err := s.backupForReplay(); err != nil {
		t.Fatal(err)
	}
	retained := replayBackupFiles(t, path)[0]
	before, err := os.ReadFile(retained)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	}()
	if err := s.backupForReplay(); err == nil {
		t.Fatal("reused metadata without obtaining a current snapshot")
	}
	after, err := os.ReadFile(retained)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("backup failure damaged retained snapshot: %v", err)
	}
}
