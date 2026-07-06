package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
)

func requirePayloadMap(t *testing.T, env any) map[string]any {
	t.Helper()
	envelope, ok := env.(writeEnvelope)
	if !ok {
		t.Fatalf("expected writeEnvelope, got %T", env)
	}
	payload, ok := envelope.payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", envelope.payload)
	}
	return payload
}

func assertAnytimeSchedule(t *testing.T, payload map[string]any) {
	t.Helper()
	if payload["st"] != 1 {
		t.Fatalf("st = %v, want 1", payload["st"])
	}
	if payload["sr"] != nil {
		t.Fatalf("sr = %v, want nil", payload["sr"])
	}
	if payload["tir"] != nil {
		t.Fatalf("tir = %v, want nil", payload["tir"])
	}
}

func TestTaskUpdateAnytimeClearsScheduleDates(t *testing.T) {
	payload := newTaskUpdate().Project("project-1").Anytime().build()

	assertAnytimeSchedule(t, payload)
	if got := payload["pr"]; got == nil {
		t.Fatal("project field was not set")
	}
}

func TestHasExplicitSchedule(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]string
		want bool
	}{
		{
			name: "none",
			opts: map[string]string{},
			want: false,
		},
		{
			name: "when",
			opts: map[string]string{"when": "today"},
			want: true,
		},
		{
			name: "scheduled",
			opts: map[string]string{"scheduled": "2026-05-20"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasExplicitSchedule(tt.opts); got != tt.want {
				t.Fatalf("hasExplicitSchedule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandNeedsHistoryHead(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"list", false},
		{"show", false},
		{"areas", false},
		{"projects", false},
		{"tags", false},
		{"create", true},
		{"edit", true},
		{"complete", true},
		{"trash", true},
		{"batch", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := commandNeedsHistoryHead(tt.cmd); got != tt.want {
				t.Fatalf("commandNeedsHistoryHead(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestBatchMoveToProjectUsesNullScheduleDates(t *testing.T) {
	env, _, err := buildBatchMoveToProject(BatchOp{
		UUID:    "task-1",
		Project: "project-1",
	})
	if err != nil {
		t.Fatalf("buildBatchMoveToProject failed: %v", err)
	}

	payload := requirePayloadMap(t, env)
	assertAnytimeSchedule(t, payload)

	bs, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	var wire struct {
		P map[string]any `json:"p"`
	}
	if err := json.Unmarshal(bs, &wire); err != nil {
		t.Fatalf("unmarshal wire payload failed: %v", err)
	}
	if wire.P["sr"] != nil {
		t.Fatalf("wire sr = %v, want null", wire.P["sr"])
	}
	if wire.P["tir"] != nil {
		t.Fatalf("wire tir = %v, want null", wire.P["tir"])
	}
}

func TestBatchMoveToAreaUsesNullScheduleDates(t *testing.T) {
	env, _, err := buildBatchMoveToArea(BatchOp{
		UUID: "task-1",
		Area: "area-1",
	})
	if err != nil {
		t.Fatalf("buildBatchMoveToArea failed: %v", err)
	}

	assertAnytimeSchedule(t, requirePayloadMap(t, env))
}

func TestBatchEditAutoAnytimeUsesNullScheduleDates(t *testing.T) {
	tests := []struct {
		name string
		op   BatchOp
	}{
		{
			name: "project",
			op:   BatchOp{UUID: "task-1", Project: "project-1"},
		},
		{
			name: "area",
			op:   BatchOp{UUID: "task-1", Area: "area-1"},
		},
		{
			name: "heading",
			op:   BatchOp{UUID: "task-1", Heading: "heading-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _, err := buildBatchEdit(tt.op)
			if err != nil {
				t.Fatalf("buildBatchEdit failed: %v", err)
			}
			assertAnytimeSchedule(t, requirePayloadMap(t, env))
		})
	}
}

func TestBatchEditExplicitWhenWinsOverAutoAnytime(t *testing.T) {
	env, _, err := buildBatchEdit(BatchOp{
		UUID:    "task-1",
		Project: "project-1",
		When:    "someday",
	})
	if err != nil {
		t.Fatalf("buildBatchEdit failed: %v", err)
	}

	payload := requirePayloadMap(t, env)
	if payload["st"] != 2 {
		t.Fatalf("st = %v, want 2", payload["st"])
	}
	if payload["sr"] != nil {
		t.Fatalf("sr = %v, want nil", payload["sr"])
	}
	if payload["tir"] != nil {
		t.Fatalf("tir = %v, want nil", payload["tir"])
	}
}

func TestCLIStateCachePathUsesEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("THINGS_CLI_CACHE", path)

	if got := cliStateCachePath(); got != path {
		t.Fatalf("cliStateCachePath() = %q, want %q", got, path)
	}
}

func TestCLIStateCacheMissingFile(t *testing.T) {
	cache, err := loadCLIStateCache(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("loadCLIStateCache failed: %v", err)
	}
	if cache != nil {
		t.Fatalf("cache = %#v, want nil", cache)
	}
}

func TestCLIStateCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	state := memory.NewState()
	state.Tasks["task-1"] = &thingscloud.Task{
		UUID:  "task-1",
		Title: "Cached Task",
	}
	state.Areas["area-1"] = &thingscloud.Area{
		UUID:  "area-1",
		Title: "Cached Area",
	}

	cache := &cliStateCache{
		HistoryID:   "history-1",
		ServerIndex: 42,
		State:       state,
	}
	if err := saveCLIStateCache(path, cache); err != nil {
		t.Fatalf("saveCLIStateCache failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cache failed: %v", err)
	}
	if info.IsDir() {
		t.Fatal("cache path is a directory")
	}

	loaded, err := loadCLIStateCache(path)
	if err != nil {
		t.Fatalf("loadCLIStateCache failed: %v", err)
	}
	if loaded.HistoryID != "history-1" {
		t.Fatalf("HistoryID = %q, want history-1", loaded.HistoryID)
	}
	if loaded.ServerIndex != 42 {
		t.Fatalf("ServerIndex = %d, want 42", loaded.ServerIndex)
	}
	if loaded.State.Tasks["task-1"].Title != "Cached Task" {
		t.Fatalf("task title = %q, want Cached Task", loaded.State.Tasks["task-1"].Title)
	}
	if loaded.State.Areas["area-1"].Title != "Cached Area" {
		t.Fatalf("area title = %q, want Cached Area", loaded.State.Areas["area-1"].Title)
	}
}

func TestCLIStateCacheNormalizesEmptyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	cache := &cliStateCache{
		HistoryID:   "history-1",
		ServerIndex: 7,
		State:       &memory.State{},
	}
	if err := saveCLIStateCache(path, cache); err != nil {
		t.Fatalf("saveCLIStateCache failed: %v", err)
	}

	loaded, err := loadCLIStateCache(path)
	if err != nil {
		t.Fatalf("loadCLIStateCache failed: %v", err)
	}
	if loaded.State.Tasks == nil {
		t.Fatal("Tasks map was not initialized")
	}
	if loaded.State.Areas == nil {
		t.Fatal("Areas map was not initialized")
	}
	if loaded.State.Tags == nil {
		t.Fatal("Tags map was not initialized")
	}
	if loaded.State.CheckListItems == nil {
		t.Fatal("CheckListItems map was not initialized")
	}
}

func testStateForListFilters() *memory.State {
	state := memory.NewState()
	today := time.Now().UTC()
	tomorrow := today.Add(24 * time.Hour)

	state.Areas["area-1"] = &thingscloud.Area{UUID: "area-1", Title: "Work"}
	state.Tasks["project-1"] = &thingscloud.Task{
		UUID:  "project-1",
		Title: "Project Alpha",
		Type:  thingscloud.TaskTypeProject,
	}
	state.Tasks["inbox-1"] = &thingscloud.Task{
		UUID:     "inbox-1",
		Title:    "Inbox Task",
		Schedule: thingscloud.TaskScheduleInbox,
	}
	state.Tasks["today-1"] = &thingscloud.Task{
		UUID:          "today-1",
		Title:         "Today Task",
		Schedule:      thingscloud.TaskScheduleAnytime,
		ScheduledDate: &today,
	}
	state.Tasks["anytime-1"] = &thingscloud.Task{
		UUID:     "anytime-1",
		Title:    "Anytime Task",
		Schedule: thingscloud.TaskScheduleAnytime,
	}
	state.Tasks["someday-1"] = &thingscloud.Task{
		UUID:     "someday-1",
		Title:    "Someday Task",
		Schedule: thingscloud.TaskScheduleSomeday,
	}
	state.Tasks["upcoming-1"] = &thingscloud.Task{
		UUID:          "upcoming-1",
		Title:         "Upcoming Task",
		Note:          "needle in note",
		Schedule:      thingscloud.TaskScheduleSomeday,
		ScheduledDate: &tomorrow,
	}
	state.Tasks["project-task-1"] = &thingscloud.Task{
		UUID:          "project-task-1",
		Title:         "Project Task",
		Schedule:      thingscloud.TaskScheduleAnytime,
		ParentTaskIDs: []string{"project-1"},
	}
	state.Tasks["area-task-1"] = &thingscloud.Task{
		UUID:     "area-task-1",
		Title:    "Area Task",
		Schedule: thingscloud.TaskScheduleAnytime,
		AreaIDs:  []string{"area-1"},
	}
	state.Tasks["completed-1"] = &thingscloud.Task{
		UUID:     "completed-1",
		Title:    "Completed Task",
		Schedule: thingscloud.TaskScheduleAnytime,
		Status:   thingscloud.TaskStatusCompleted,
	}
	state.Tasks["trashed-1"] = &thingscloud.Task{
		UUID:     "trashed-1",
		Title:    "Trashed Task",
		Schedule: thingscloud.TaskScheduleAnytime,
		InTrash:  true,
	}
	return state
}

func outputUUIDs(tasks []TaskOutput) []string {
	uuids := make([]string, len(tasks))
	for i, task := range tasks {
		uuids[i] = task.UUID
	}
	return uuids
}

func requireUUIDs(t *testing.T, tasks []TaskOutput, want ...string) {
	t.Helper()
	got := outputUUIDs(tasks)
	if len(got) != len(want) {
		t.Fatalf("uuids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uuids = %v, want %v", got, want)
		}
	}
}

func TestListTasksLocationFilters(t *testing.T) {
	state := testStateForListFilters()

	requireUUIDs(t, listTasks(state, map[string]string{"today": "true"}), "today-1")
	requireUUIDs(t, listTasks(state, map[string]string{"inbox": "true"}), "inbox-1")
	requireUUIDs(t, listTasks(state, map[string]string{"someday": "true"}), "someday-1")
	requireUUIDs(t, listTasks(state, map[string]string{"upcoming": "true"}), "upcoming-1")

	anytime := outputUUIDs(listTasks(state, map[string]string{"anytime": "true"}))
	wantAnytime := []string{"anytime-1", "area-task-1", "project-task-1"}
	if len(anytime) != len(wantAnytime) {
		t.Fatalf("anytime = %v, want %v", anytime, wantAnytime)
	}
	for i := range wantAnytime {
		if anytime[i] != wantAnytime[i] {
			t.Fatalf("anytime = %v, want %v", anytime, wantAnytime)
		}
	}
}

func TestListTasksSearchAndContainerFilters(t *testing.T) {
	state := testStateForListFilters()

	requireUUIDs(t, listTasks(state, map[string]string{"search": "needle"}), "upcoming-1")
	requireUUIDs(t, listTasks(state, map[string]string{"search": "project"}), "project-task-1")
	requireUUIDs(t, listTasks(state, map[string]string{"area": "Work"}), "area-task-1")
	requireUUIDs(t, listTasks(state, map[string]string{"project": "Project Alpha"}), "project-task-1")
}
