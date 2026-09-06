package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	"github.com/arthursoares/things-cloud-sdk/state/memory"
)

func oldReplayCache() []byte {
	return []byte(`{"version":1,"historyId":"test-history","serverIndex":2,"state":{"Tasks":{}}}`)
}

func TestCLIStateCacheRebuildsTask7AtHead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := oldReplayCache()
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THINGS_CLI_CACHE", path)
	var starts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/items") {
			starts = append(starts, r.URL.Query().Get("start-index"))
			fmt.Fprint(w, task7CachePage)
		} else {
			fmt.Fprint(w, `{"latest-server-index":2}`)
		}
	}))
	defer server.Close()
	c := things.New(server.URL, "test@example.com", "test-password")
	ctx := cliContext{client: c, history: c.HistoryWithID("test-history")}
	state := ctx.loadState()
	if len(state.Tasks) != 1 || state.Tasks["BXmAcvS6yK1eDhW31MuZrL"] == nil || state.Tasks["BXmAcvS6yK1eDhW31MuZrL"].Title != "New task" {
		t.Fatal("old caught-up cache was not rebuilt with the missing Task7 task")
	}
	if len(starts) != 1 || starts[0] != "0" {
		t.Fatalf("replay starts = %v, want [0]", starts)
	}
	backups, err := filepath.Glob(path + ".before-replay-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, error %v", backups, err)
	}
	got, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(got, old) {
		t.Fatal("backup did not preserve original cache bytes")
	}
	info, err := os.Stat(backups[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("backup permissions are not 0600")
	}
	cache, err := loadCLIStateCache(path)
	if err != nil || cache == nil || cache.Version != things.TaskReplayVersion || cache.ServerIndex != 2 {
		t.Fatalf("rebuilt cache metadata is invalid: %+v, %v", cache, err)
	}
	ctx.loadState()
	if len(starts) != 1 {
		t.Fatal("second load replayed an already recovered cache")
	}
}

const task7CachePage = `{"items":[{"BXmAcvS6yK1eDhW31MuZrL":{"e":"Task7","t":0,"p":{"tt":"","tp":0,"st":0,"ss":0}}},{"BXmAcvS6yK1eDhW31MuZrL":{"e":"Task7","t":1,"p":{"tt":"New task"}}}],"current-item-index":2}`

func TestCLIStateCacheReplacementFailurePreservesOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failure requires an unprivileged user")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	old := oldReplayCache()
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	err := saveCLIStateCache(path, &cliStateCache{HistoryID: "test-history", ServerIndex: 2, State: memory.NewState()})
	if err == nil {
		t.Error("expected failure creating an atomic replacement in a read-only directory")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, old) {
		t.Error("failed replacement changed the original cache")
	}
}

func TestCLIRecoveryHelper(t *testing.T) {
	if os.Getenv("THINGS_CACHE_RECOVERY_HELPER") == "" {
		return
	}
	c := things.New(os.Getenv("THINGS_ENDPOINT"), "test@example.com", "test-password")
	ctx := cliContext{client: c, history: c.HistoryWithID("test-history")}
	ctx.loadState()
}

func TestCLIStateCacheFailedReplayPreservesOriginal(t *testing.T) {
	cases := []struct {
		name, page, diagnostic string
		concurrent             bool
	}{
		{"unsupported", `{"items":[{"private-id":{"e":"Task8","t":0,"p":{"tt":"private title"}}}],"current-item-index":2}`, "unsupported task kind Task8", false},
		{"no progress", `{"items":[],"current-item-index":2}`, "progress", false},
		{"premature end", `{"items":[{}],"current-item-index":1}`, "incomplete", false},
		{"concurrent cache", task7CachePage, "changed", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			// Version 0 forces a replay in the old implementation too.
			old := []byte(`{"version":0,"historyId":"test-history","serverIndex":2,"state":{"Tasks":{}}}`)
			if err := os.WriteFile(path, old, 0o600); err != nil {
				t.Fatal(err)
			}
			want := old
			if tc.concurrent {
				want = []byte(`{"version":2,"historyId":"test-history","serverIndex":3,"state":{"Tasks":{}}}`)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/items") {
					if tc.concurrent {
						if err := os.WriteFile(path, want, 0o600); err != nil {
							t.Error(err)
						}
					}
					fmt.Fprint(w, tc.page)
				} else {
					fmt.Fprint(w, `{"latest-server-index":2}`)
				}
			}))
			defer server.Close()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCLIRecoveryHelper$")
			cmd.Env = append(os.Environ(), "THINGS_CACHE_RECOVERY_HELPER=1", "THINGS_CLI_CACHE="+path, "THINGS_ENDPOINT="+server.URL)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("expected %q failure, got %v: %s", tc.diagnostic, err, output)
			}
			if strings.Contains(string(output), "private-id") || strings.Contains(string(output), "private title") {
				t.Fatal("diagnostic exposed private task data")
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatal("failed replay overwrote the original/concurrently updated cache")
			}
			var parsed map[string]any
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatal(err)
			}
		})
	}
}
