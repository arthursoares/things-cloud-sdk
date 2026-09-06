package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

const cliValidationHelper = "THINGS_CLI_VALIDATION_HELPER"

// TestCLIValidationHelper runs the real main function in a subprocess so its
// os.Exit paths can be asserted without changing production error handling.
func TestCLIValidationHelper(t *testing.T) {
	if os.Getenv(cliValidationHelper) == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("THINGS_CLI_VALIDATION_ARGS")), &args); err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"things-cli"}, args...)
	main()
}

type cliValidationCloud struct {
	server       *httptest.Server
	mu           sync.Mutex
	bodies       [][]byte
	historyReads int
}

func newCLIValidationCloud(t *testing.T, ordinaryTaskIDs ...string) *cliValidationCloud {
	t.Helper()
	c := &cliValidationCloud{}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/commit"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read commit: %v", err)
			}
			c.mu.Lock()
			c.bodies = append(c.bodies, body)
			c.mu.Unlock()
			_, _ = io.WriteString(w, `{"server-head-index":2}`)
		case strings.HasSuffix(r.URL.Path, "/items"):
			c.mu.Lock()
			c.historyReads++
			c.mu.Unlock()
			items := []map[string]any{}
			if len(ordinaryTaskIDs) > 0 {
				created := make(map[string]any, len(ordinaryTaskIDs))
				for _, id := range ordinaryTaskIDs {
					created[id] = map[string]any{
						"e": "Task7",
						"t": 0,
						"p": map[string]any{
							"tt": "seeded ordinary task",
							"tp": 0,
							"st": 1,
							"ss": 0,
							"rr": nil,
							"rp": nil,
							"rt": []string{},
						},
					}
				}
				items = append(items, created)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"items":              items,
				"current-item-index": 1,
				"schema":             301,
			}); err != nil {
				t.Errorf("encode history fixture: %v", err)
			}
		case strings.Contains(r.URL.Path, "/account/"):
			_, _ = io.WriteString(w, `{"email":"validation@example.com","status":"SYAccountStatusActive","history-key":"validation-history"}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(c.server.Close)
	return c
}

func (c *cliValidationCloud) commits() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.bodies...)
}

func (c *cliValidationCloud) historyReadCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.historyReads
}

func runValidationCLI(t *testing.T, cloud *cliValidationCloud, stdin string, args ...string) (int, string) {
	t.Helper()
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLIValidationHelper$")
	cmd.Env = append(os.Environ(),
		cliValidationHelper+"=1",
		"THINGS_CLI_VALIDATION_ARGS="+string(encodedArgs),
		"THINGS_ENDPOINT="+cloud.server.URL,
		"THINGS_USERNAME=validation@example.com",
		"THINGS_PASSWORD=test-password",
	)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ctx.Err() != nil {
		t.Fatalf("CLI subprocess timed out: %v\n%s", ctx.Err(), out)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), string(out)
	}
	t.Fatalf("run CLI: %v", err)
	return -1, ""
}

func TestCLIWriteValidationRejectsInvalidInputBeforeCommit(t *testing.T) {
	id := thingscloud.NewUUID()
	otherID := thingscloud.NewUUID()
	validCreate := fmt.Sprintf(`{"cmd":"create","title":"valid","uuid":%q}`, otherID)

	tests := []struct {
		name  string
		args  []string
		stdin string
		field string
	}{
		{"area malformed tag", []string{"create-area", "Area", "--tags", "bad-tag"}, "", "tags"},
		{"area padded tag", []string{"create-area", "Area", "--tags", " " + id + " "}, "", "tags"},
		{"area empty tag component", []string{"create-area", "Area", "--tags", id + ",," + otherID}, "", "tags"},
		{"area empty tags value", []string{"create-area", "Area", "--tags", ""}, "", "tags"},
		{"tag malformed parent", []string{"create-tag", "Tag", "--parent", "bad-parent"}, "", "parent"},
		{"tag padded parent", []string{"create-tag", "Tag", "--parent", " " + id + " "}, "", "parent"},
		{"tag empty parent value", []string{"create-tag", "Tag", "--parent", ""}, "", "parent"},
		{"create empty project value", []string{"create", "Task", "--project", ""}, "", "project"},
		{"create empty uuid value", []string{"create", "Task", "--uuid", ""}, "", "uuid"},
		{"create padded tag", []string{"create", "Task", "--tags", " " + id + " "}, "", "tags"},
		{"edit padded tag", []string{"edit", id, "--tags", " " + otherID + " "}, "", "tags"},
		{"edit empty tags value", []string{"edit", id, "--tags", ""}, "", "tags"},
		{"edit area with project", []string{"edit", id, "--area", id, "--project", otherID}, "", "area"},
		{"edit area with heading", []string{"edit", id, "--area", id, "--heading", otherID}, "", "area"},
		{"create invalid when", []string{"create", "Task", "--when", "tomorrow"}, "", "when"},
		{"create empty when", []string{"create", "Task", "--when", ""}, "", "when"},
		{"create missing when value", []string{"create", "Task", "--when"}, "", "when"},
		{"edit invalid when", []string{"edit", id, "--when", "tomorrow"}, "", "when"},
		{"edit empty when", []string{"edit", id, "--when", ""}, "", "when"},
		{"edit missing when value", []string{"edit", id, "--when"}, "", "when"},
		{"create impossible deadline", []string{"create", "Task", "--deadline", "2026-02-30"}, "", "deadline"},
		{"create malformed deadline", []string{"create", "Task", "--deadline", "02/28/2026"}, "", "deadline"},
		{"create empty deadline", []string{"create", "Task", "--deadline", ""}, "", "deadline"},
		{"create missing deadline value", []string{"create", "Task", "--deadline"}, "", "deadline"},
		{"edit impossible deadline", []string{"edit", id, "--deadline", "2026-02-30"}, "", "deadline"},
		{"edit malformed deadline", []string{"edit", id, "--deadline", "02/28/2026"}, "", "deadline"},
		{"edit empty deadline", []string{"edit", id, "--deadline", ""}, "", "deadline"},
		{"edit missing deadline value", []string{"edit", id, "--deadline"}, "", "deadline"},
		{"create impossible scheduled", []string{"create", "Task", "--scheduled", "2026-02-30"}, "", "scheduled"},
		{"create malformed scheduled", []string{"create", "Task", "--scheduled", "next-week"}, "", "scheduled"},
		{"create empty scheduled", []string{"create", "Task", "--scheduled", ""}, "", "scheduled"},
		{"create missing scheduled value", []string{"create", "Task", "--scheduled"}, "", "scheduled"},
		{"edit impossible scheduled", []string{"edit", id, "--scheduled", "2026-02-30"}, "", "scheduled"},
		{"edit malformed scheduled", []string{"edit", id, "--scheduled", "next-week"}, "", "scheduled"},
		{"edit empty scheduled", []string{"edit", id, "--scheduled", ""}, "", "scheduled"},
		{"edit missing scheduled value", []string{"edit", id, "--scheduled"}, "", "scheduled"},
		{"create invalid type", []string{"create", "Task", "--type", "area"}, "", "type"},
		{"purge invalid target", []string{"purge", "bad-target"}, "", "uuid"},
		{"batch create invalid when", []string{"batch"}, `[{"cmd":"create","title":"x","when":"tomorrow"}]`, "when"},
		{"batch create invalid deadline", []string{"batch"}, `[{"cmd":"create","title":"x","deadline":"2026-02-30"}]`, "deadline"},
		{"batch create padded tag", []string{"batch"}, fmt.Sprintf(`[{"cmd":"create","title":"x","tags":[%q]}]`, " "+id+" "), "tags"},
		{"batch edit invalid when", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"when":"tomorrow"}]`, id), "when"},
		{"batch edit invalid deadline", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"deadline":"bad-date"}]`, id), "deadline"},
		{"batch edit padded tag", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"tags":[%q]}]`, id, " "+otherID+" "), "tags"},
		{"batch edit area with project", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"area":%q,"project":%q}]`, id, id, otherID), "area"},
		{"batch edit area with heading", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"area":%q,"heading":%q}]`, id, id, otherID), "area"},
		{"batch extra project overrides valid field", []string{"batch"}, fmt.Sprintf(`[{"cmd":"create","title":"x","project":%q,"extra":{"project":"bad-project"}}]`, id), "project"},
		{"batch extra empty project overrides valid field", []string{"batch"}, fmt.Sprintf(`[{"cmd":"create","title":"x","project":%q,"extra":{"project":""}}]`, id), "project"},
		{"batch extra tags overrides valid field", []string{"batch"}, fmt.Sprintf(`[{"cmd":"create","title":"x","tags":[%q],"extra":{"tags":"bad-tag"}}]`, id), "tags"},
		{"batch extra empty tags overrides valid field", []string{"batch"}, fmt.Sprintf(`[{"cmd":"create","title":"x","tags":[%q],"extra":{"tags":""}}]`, id), "tags"},
		{"batch extra invalid when", []string{"batch"}, `[{"cmd":"create","title":"x","extra":{"when":"tomorrow"}}]`, "when"},
		{"batch extra invalid deadline", []string{"batch"}, `[{"cmd":"create","title":"x","extra":{"deadline":"bad-date"}}]`, "deadline"},
		{"batch extra invalid scheduled", []string{"batch"}, `[{"cmd":"create","title":"x","extra":{"scheduled":"bad-date"}}]`, "scheduled"},
		{"batch extra invalid type", []string{"batch"}, `[{"cmd":"create","title":"x","extra":{"type":"area"}}]`, "type"},
		{"batch create rejects unknown extra", []string{"batch"}, `[{"cmd":"create","title":"x","extra":{"checklist":"ignored"}}]`, "checklist"},
		{"batch purge invalid target", []string{"batch"}, `[{"cmd":"purge","uuid":"bad-target"}]`, "uuid"},
		{"batch unknown scheduled field", []string{"batch"}, `[{"cmd":"create","title":"x","scheduled":"2026-10-12"}]`, "scheduled"},
		{"batch edit rejects extra", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"extra":{"scheduled":"2026-10-12"}}]`, id), "extra"},
		{"batch edit rejects empty extra object", []string{"batch"}, fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"extra":{}}]`, id), "extra"},
		{"batch rejects trailing JSON", []string{"batch"}, fmt.Sprintf(`[%s] {"extra":"value"}`, validCreate), "json"},
		{"batch validates all operations atomically", []string{"batch"}, fmt.Sprintf(`[%s,{"cmd":"edit","uuid":%q,"deadline":"bad-date"}]`, validCreate, id), "deadline"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloud := newCLIValidationCloud(t)
			exitCode, output := runValidationCLI(t, cloud, tc.stdin, tc.args...)
			if exitCode == 0 {
				t.Errorf("exit code = 0, want nonzero; output:\n%s", output)
			}
			if !strings.Contains(strings.ToLower(output), tc.field) {
				t.Errorf("output does not identify %q: %s", tc.field, output)
			}
			if got := len(cloud.commits()); got != 0 {
				t.Errorf("commit requests = %d, want 0", got)
			}
			if got := cloud.historyReadCount(); got != 1 {
				t.Errorf("history reads = %d, want only the initial head sync and no write preflight", got)
			}
		})
	}
}

func TestCLIWriteValidationAcceptsCanonicalReferencesExactly(t *testing.T) {
	t.Run("area tags", func(t *testing.T) {
		areaID, tag1, tag2 := thingscloud.NewUUID(), thingscloud.NewUUID(), thingscloud.NewUUID()
		cloud := newCLIValidationCloud(t)
		exitCode, output := runValidationCLI(t, cloud, "", "create-area", "Area", "--uuid", areaID, "--tags", tag1+","+tag2)
		if exitCode != 0 {
			t.Fatalf("exit code = %d: %s", exitCode, output)
		}
		payload := validationCommitPayload(t, cloud, areaID)
		want := []string{tag1, tag2}
		var got []string
		if err := json.Unmarshal(payload["tg"], &got); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("tg = %q, want exact refs %q", got, want)
		}
	})

	t.Run("tag parent", func(t *testing.T) {
		tagID, parentID := thingscloud.NewUUID(), thingscloud.NewUUID()
		cloud := newCLIValidationCloud(t)
		exitCode, output := runValidationCLI(t, cloud, "", "create-tag", "Tag", "--uuid", tagID, "--parent", parentID)
		if exitCode != 0 {
			t.Fatalf("exit code = %d: %s", exitCode, output)
		}
		payload := validationCommitPayload(t, cloud, tagID)
		var got []string
		if err := json.Unmarshal(payload["pn"], &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != parentID {
			t.Errorf("pn = %q, want exact ref [%q]", got, parentID)
		}
	})
}

func TestCLIWriteValidationAcceptsBatchDatesAndScheduledExtra(t *testing.T) {
	createID, editID := thingscloud.NewUUID(), thingscloud.NewUUID()
	projectID, tagID := thingscloud.NewUUID(), thingscloud.NewUUID()
	input := fmt.Sprintf(`[
		{"cmd":"create","title":"dated","uuid":%q,"when":"someday","deadline":"2026-10-14","project":%q,"tags":[%q],"extra":{"when":"anytime","deadline":"2026-10-15","scheduled":"2026-10-12"}},
		{"cmd":"edit","uuid":%q,"when":"someday","deadline":"2026-10-20"}
	]`, createID, projectID, tagID, editID)
	cloud := newCLIValidationCloud(t, editID)
	exitCode, output := runValidationCLI(t, cloud, input, "batch")
	if exitCode != 0 {
		t.Fatalf("exit code = %d: %s", exitCode, output)
	}

	create := validationCommitPayload(t, cloud, createID)
	edit := validationCommitPayload(t, cloud, editID)
	requireWireUnixDate(t, create, "dd", "2026-10-15")
	requireWireUnixDate(t, create, "sr", "2026-10-12")
	requireWireUnixDate(t, create, "tir", "2026-10-12")
	requireWireUnixDate(t, edit, "dd", "2026-10-20")
	if string(create["st"]) != "1" || string(edit["st"]) != "2" {
		t.Errorf("schedules create/edit = %s/%s, want 1/2", create["st"], edit["st"])
	}
	if string(create["pr"]) != fmt.Sprintf(`[%q]`, projectID) || string(create["tg"]) != fmt.Sprintf(`[%q]`, tagID) {
		t.Errorf("create refs changed: pr=%s tg=%s", create["pr"], create["tg"])
	}
}

func TestCLIWriteValidationAcceptsUnsetBatchOptionals(t *testing.T) {
	id := thingscloud.NewUUID()
	tests := []struct {
		name            string
		input           string
		ordinaryTaskIDs []string
	}{
		{
			name:  "empty top-level strings without extra",
			input: `[{"cmd":"create","title":"empty optionals","uuid":"","note":"","when":"","deadline":"","project":"","area":"","heading":"","tags":[],"type":""}]`,
		},
		{
			name:            "null extra",
			input:           fmt.Sprintf(`[{"cmd":"edit","uuid":%q,"title":"updated","extra":null}]`, id),
			ordinaryTaskIDs: []string{id},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloud := newCLIValidationCloud(t, tc.ordinaryTaskIDs...)
			exitCode, output := runValidationCLI(t, cloud, tc.input, "batch")
			if exitCode != 0 {
				t.Fatalf("exit code = %d: %s", exitCode, output)
			}
			if got := len(cloud.commits()); got != 1 {
				t.Errorf("commit requests = %d, want 1", got)
			}
		})
	}
}

func validationCommitPayload(t *testing.T, cloud *cliValidationCloud, id string) map[string]json.RawMessage {
	t.Helper()
	bodies := cloud.commits()
	if len(bodies) != 1 {
		t.Fatalf("commit requests = %d, want 1", len(bodies))
	}
	var envelope map[string]struct {
		Payload map[string]json.RawMessage `json:"p"`
	}
	if err := json.Unmarshal(bodies[0], &envelope); err != nil {
		t.Fatalf("decode commit: %v", err)
	}
	item, ok := envelope[id]
	if !ok {
		t.Fatalf("commit has no item %q: %s", id, bodies[0])
	}
	return item.Payload
}

func requireWireUnixDate(t *testing.T, payload map[string]json.RawMessage, field, date string) {
	t.Helper()
	want, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	if err := json.Unmarshal(payload[field], &got); err != nil {
		t.Fatalf("decode %s: %v", field, err)
	}
	if got != want.Unix() {
		t.Errorf("%s = %d, want %d for %s", field, got, want.Unix(), date)
	}
}
