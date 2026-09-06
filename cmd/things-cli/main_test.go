package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
)

var (
	testTaskID    = thingscloud.NewUUID()
	testProjectID = thingscloud.NewUUID()
	testAreaID    = thingscloud.NewUUID()
	testHeadingID = thingscloud.NewUUID()
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
	payload := newTaskUpdate().Project(testProjectID).Anytime().build()

	assertAnytimeSchedule(t, payload)
	if got := payload["pr"]; got == nil {
		t.Fatal("project field was not set")
	}
}

func TestTaskScheduleForDate(t *testing.T) {
	today := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC).Unix()
	tests := []struct {
		name string
		date int64
		want int
	}{
		{name: "past", date: today - 86400, want: 1},
		{name: "today", date: today, want: 1},
		{name: "future", date: today + 86400, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskScheduleForDate(tt.date, today); got != tt.want {
				t.Fatalf("taskScheduleForDate() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewTaskCreatePayloadScheduledDateClassification(t *testing.T) {
	today := time.Now()
	tests := []struct {
		name string
		date time.Time
		want int
	}{
		{name: "past", date: today.AddDate(0, 0, -1), want: 1},
		{name: "today", date: today, want: 1},
		{name: "future", date: today.AddDate(0, 0, 1), want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := tt.date.Format("2006-01-02")
			wantDate := parseDate(date).Unix()
			payload := newTaskCreatePayload("scheduled", map[string]string{"scheduled": date})
			if payload.St != tt.want {
				t.Fatalf("st = %d, want %d", payload.St, tt.want)
			}
			if payload.Sr == nil || *payload.Sr != wantDate || payload.Tir == nil || *payload.Tir != wantDate {
				t.Fatalf("sr/tir = %v/%v, want UTC midnight %d", payload.Sr, payload.Tir, wantDate)
			}
		})
	}
}

func TestNewTaskCreatePayloadExplicitWhenOverridesScheduledClassification(t *testing.T) {
	today := time.Now()
	tests := []struct {
		name string
		when string
		date time.Time
		want int
	}{
		{name: "future anytime", when: "anytime", date: today.AddDate(0, 0, 1), want: 1},
		{name: "today someday", when: "someday", date: today, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := tt.date.Format("2006-01-02")
			wantDate := parseDate(date).Unix()
			payload := newTaskCreatePayload("scheduled", map[string]string{
				"scheduled": date,
				"when":      tt.when,
			})
			if payload.St != tt.want {
				t.Fatalf("st = %d, want explicit --when status %d", payload.St, tt.want)
			}
			if payload.Sr == nil || *payload.Sr != wantDate || payload.Tir == nil || *payload.Tir != wantDate {
				t.Fatalf("sr/tir = %v/%v, want scheduled date %d", payload.Sr, payload.Tir, wantDate)
			}
		})
	}
}

func TestNewTaskCreatePayloadRelationshipsPreserveScheduledStatus(t *testing.T) {
	future := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	tests := []struct {
		name string
		opts map[string]string
	}{
		{name: "project", opts: map[string]string{"scheduled": future, "project": testProjectID}},
		{name: "area", opts: map[string]string{"scheduled": future, "area": testAreaID}},
		{name: "heading", opts: map[string]string{"scheduled": future, "heading": testHeadingID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := newTaskCreatePayload("scheduled", tt.opts)
			if payload.St != 2 {
				t.Fatalf("st = %d, want 2", payload.St)
			}
			if payload.Sr == nil || payload.Tir == nil {
				t.Fatalf("sr/tir = %v/%v, want scheduled date", payload.Sr, payload.Tir)
			}
		})
	}
}

func TestTaskUpdateScheduleDateClassification(t *testing.T) {
	today := todayMidnightUTC()
	for _, tt := range []struct {
		name string
		date int64
		want int
	}{
		{name: "past", date: today - 86400, want: 1},
		{name: "today", date: today, want: 1},
		{name: "future", date: today + 86400, want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := newTaskUpdate().ScheduleDate(tt.date).build()
			if payload["st"] != tt.want || payload["sr"] != tt.date || payload["tir"] != tt.date {
				t.Fatalf("schedule = st:%v sr:%v tir:%v, want %d/%d/%d", payload["st"], payload["sr"], payload["tir"], tt.want, tt.date, tt.date)
			}
		})
	}
}

func TestBatchCreateScheduledDateMatchesCreate(t *testing.T) {
	today := time.Now()
	tests := []struct {
		name string
		date time.Time
		when string
	}{
		{name: "past", date: today.AddDate(0, 0, -1)},
		{name: "today", date: today},
		{name: "future", date: today.AddDate(0, 0, 1)},
		{name: "explicit someday", date: today, when: "someday"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := tt.date.Format("2006-01-02")
			opts := map[string]string{"scheduled": date, "project": testProjectID}
			if tt.when != "" {
				opts["when"] = tt.when
			}
			want := newTaskCreatePayload("scheduled", opts)

			env, _, err := buildBatchCreate(BatchOp{
				Title:   "scheduled",
				When:    tt.when,
				Project: testProjectID,
				Extra:   map[string]string{"scheduled": date},
			})
			if err != nil {
				t.Fatalf("buildBatchCreate failed: %v", err)
			}
			payload, ok := env.(writeEnvelope).payload.(TaskCreatePayload)
			if !ok {
				t.Fatalf("batch payload = %T, want TaskCreatePayload", env.(writeEnvelope).payload)
			}
			if payload.St != want.St || payload.Sr == nil || want.Sr == nil || *payload.Sr != *want.Sr || payload.Tir == nil || want.Tir == nil || *payload.Tir != *want.Tir {
				t.Fatalf("batch schedule = st:%d sr:%v tir:%v, want create schedule st:%d sr:%v tir:%v", payload.St, payload.Sr, payload.Tir, want.St, want.Sr, want.Tir)
			}
			if len(payload.Pr) != 1 || payload.Pr[0] != testProjectID {
				t.Fatalf("batch project = %v, want [%s]", payload.Pr, testProjectID)
			}
		})
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

func TestScheduledOptionValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    map[string]string
		wantErr bool
	}{
		{name: "absent", opts: map[string]string{}},
		{name: "valid", opts: map[string]string{"scheduled": "2026-09-06"}},
		{name: "missing value", opts: map[string]string{"scheduled": "true"}, wantErr: true},
		{name: "empty", opts: map[string]string{"scheduled": ""}, wantErr: true},
		{name: "malformed", opts: map[string]string{"scheduled": "tomorrow"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskOpts(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTaskOpts(%v) error = %v, wantErr %v", tt.opts, err, tt.wantErr)
			}
		})
	}
}

func TestDirectCommandsRejectInvalidScheduledBeforeWrite(t *testing.T) {
	if command := os.Getenv("THINGS_CLI_INVALID_SCHEDULE_COMMAND"); command != "" {
		h, commits := newWireRecorder(t)
		switch command {
		case "create":
			cmdCreate(h, []string{"invalid", "--scheduled", "tomorrow", "--project", testProjectID})
		case "edit":
			cmdEdit(h, testTaskID, []string{"--scheduled", "--area", testAreaID})
		default:
			t.Fatalf("unknown helper command %q", command)
		}
		if len(*commits) != 0 {
			t.Fatalf("invalid schedule wrote %d commits, want zero", len(*commits))
		}
		return
	}

	for _, command := range []string{"create", "edit"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDirectCommandsRejectInvalidScheduledBeforeWrite$")
			cmd.Env = append(os.Environ(), "THINGS_CLI_INVALID_SCHEDULE_COMMAND="+command)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("%s with invalid --scheduled succeeded; output: %s", command, output)
			}
			if !strings.Contains(string(output), "--scheduled") {
				t.Fatalf("%s error = %s, want scheduled-date validation error", command, output)
			}
		})
	}
}

func TestBuildBatchCreateRejectsInvalidScheduled(t *testing.T) {
	for _, tt := range []struct {
		name      string
		scheduled string
	}{
		{name: "missing value", scheduled: ""},
		{name: "malformed", scheduled: "tomorrow"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := buildBatchCreate(BatchOp{
				Title:   "invalid",
				Project: testProjectID,
				Extra:   map[string]string{"scheduled": tt.scheduled},
			})
			if err == nil || !strings.Contains(err.Error(), "--scheduled") {
				t.Fatalf("buildBatchCreate error = %v, want scheduled-date validation error", err)
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
		UUID:    testTaskID,
		Project: testProjectID,
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
		UUID: testTaskID,
		Area: testAreaID,
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
			op:   BatchOp{UUID: testTaskID, Project: testProjectID},
		},
		{
			name: "area",
			op:   BatchOp{UUID: testTaskID, Area: testAreaID},
		},
		{
			name: "heading",
			op:   BatchOp{UUID: testTaskID, Heading: testHeadingID},
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
		UUID:    testTaskID,
		Project: testProjectID,
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
	state.Tasks[testTaskID] = &thingscloud.Task{
		UUID:  testTaskID,
		Title: "Cached Task",
	}
	state.Areas[testAreaID] = &thingscloud.Area{
		UUID:  testAreaID,
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
	if loaded.Version != cliStateCacheVersion {
		t.Fatalf("Version = %d, want %d", loaded.Version, cliStateCacheVersion)
	}
	if loaded.ServerIndex != 42 {
		t.Fatalf("ServerIndex = %d, want 42", loaded.ServerIndex)
	}
	if loaded.State.Tasks[testTaskID].Title != "Cached Task" {
		t.Fatalf("task title = %q, want Cached Task", loaded.State.Tasks[testTaskID].Title)
	}
	if loaded.State.Areas[testAreaID].Title != "Cached Area" {
		t.Fatalf("area title = %q, want Cached Area", loaded.State.Areas[testAreaID].Title)
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

func TestCLIStateCacheRejectsOldVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"historyId":"history-1","serverIndex":7,"state":{}}`), 0o600); err != nil {
		t.Fatalf("writing old cache: %v", err)
	}

	cache, err := loadCLIStateCache(path)
	if err != nil {
		t.Fatalf("loadCLIStateCache failed: %v", err)
	}
	if cache != nil {
		t.Fatalf("old-version cache was accepted: %#v", cache)
	}
}

func TestCLIStateCacheMalformedFileTreatedAsNoCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"historyId":"history-1",`), 0o600); err != nil {
		t.Fatalf("writing malformed cache: %v", err)
	}

	cache, err := loadCLIStateCache(path)
	if err != nil {
		t.Fatalf("loadCLIStateCache failed: %v", err)
	}
	if cache != nil {
		t.Fatalf("malformed cache was accepted: %#v", cache)
	}
}

func testStateForListFilters() *memory.State {
	state := memory.NewState()
	today := time.Now().UTC()
	tomorrow := today.Add(24 * time.Hour)

	state.Areas[testAreaID] = &thingscloud.Area{UUID: testAreaID, Title: "Work"}
	state.Tasks[testProjectID] = &thingscloud.Task{
		UUID:  testProjectID,
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
		ParentTaskIDs: []string{testProjectID},
	}
	state.Tasks["area-task-1"] = &thingscloud.Task{
		UUID:     "area-task-1",
		Title:    "Area Task",
		Schedule: thingscloud.TaskScheduleAnytime,
		AreaIDs:  []string{testAreaID},
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

func TestGenerateUUIDAlwaysCanonical(t *testing.T) {
	for i := 0; i < 2000; i++ {
		s := generateUUID()
		if err := thingscloud.ValidateUUID(s); err != nil {
			t.Fatalf("generateUUID() = %q is not canonical Base58: %v", s, err)
		}
	}
}

func TestValidateIdentifierOpts(t *testing.T) {
	good := thingscloud.NewUUID()

	if err := validateIdentifierOpts(map[string]string{
		"uuid": good, "project": good, "area": good, "heading": good, "tags": good + "," + good,
	}); err != nil {
		t.Errorf("all-valid opts: %v, want nil", err)
	}
	if err := validateIdentifierOpts(map[string]string{"when": "today", "note": "free text"}); err != nil {
		t.Errorf("no identifier opts: %v, want nil", err)
	}

	bad := map[string]map[string]string{
		"hyphenated uuid": {"uuid": "6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c"},
		"bad project":     {"project": "not-base58-0OIl"},
		"bad area":        {"area": "abc def"},
		"bad heading":     {"heading": "VJ0edXTP9q3PmFDUuy8EQh"},
		"one bad tag":     {"tags": good + ",bad-tag-uuid"},
	}
	for name, opts := range bad {
		if err := validateIdentifierOpts(opts); err == nil {
			t.Errorf("%s: got nil error, want validation error", name)
		}
	}
}

func TestBuildBatchCreateRejectsInvalidRef(t *testing.T) {
	_, _, err := buildBatchCreate(BatchOp{Title: "x", Project: "6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c"})
	if err == nil {
		t.Error("buildBatchCreate with hyphenated project UUID: got nil error, want validation error")
	}
	_, _, err = buildBatchCreate(BatchOp{Title: "x", UUID: "not!base58"})
	if err == nil {
		t.Error("buildBatchCreate with invalid explicit UUID: got nil error, want validation error")
	}
}

func TestBuildBatchEditRejectsInvalidRef(t *testing.T) {
	_, _, err := buildBatchEdit(BatchOp{UUID: thingscloud.NewUUID(), Heading: "6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c"})
	if err == nil {
		t.Error("buildBatchEdit with hyphenated heading UUID: got nil error, want validation error")
	}
	_, _, err = buildBatchEdit(BatchOp{UUID: "bad uuid", Title: "y"})
	if err == nil {
		t.Error("buildBatchEdit with invalid target UUID: got nil error, want validation error")
	}
}

func TestNewTaskCreatePayloadStructuralTypesNeverInbox(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
	}{
		{"project default", map[string]string{"type": "project"}},
		{"project explicit inbox", map[string]string{"type": "project", "when": "inbox"}},
		{"heading default", map[string]string{"type": "heading"}},
		{"heading explicit inbox", map[string]string{"type": "heading", "when": "inbox"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := newTaskCreatePayload("x", tc.opts)
			if payload.St != 1 {
				t.Errorf("st = %d, want 1 — structural items (tp=%d) in inbox corrupt the sync history", payload.St, payload.Tp)
			}
		})
	}
}
