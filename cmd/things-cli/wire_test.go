package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

// ---------------------------------------------------------------------------
// taskUpdate builder — every method builds part of a wire payload
// ---------------------------------------------------------------------------

func TestTaskUpdateBuilderFields(t *testing.T) {
	midnight := todayMidnightUTC()

	cases := []struct {
		name  string
		build func() map[string]any
		want  map[string]any
	}{
		{"Title", func() map[string]any { return newTaskUpdate().Title("new title").build() },
			map[string]any{"tt": "new title"}},
		{"Status completed", func() map[string]any { return newTaskUpdate().Status(3).build() },
			map[string]any{"ss": 3}},
		{"StopDate", func() map[string]any { return newTaskUpdate().StopDate(1751791234.5).build() },
			map[string]any{"sp": 1751791234.5}},
		{"Trash", func() map[string]any { return newTaskUpdate().Trash(true).build() },
			map[string]any{"tr": true}},
		{"Today", func() map[string]any { return newTaskUpdate().Today().build() },
			map[string]any{"st": 1, "sr": midnight, "tir": midnight}},
		{"Anytime", func() map[string]any { return newTaskUpdate().Anytime().build() },
			map[string]any{"st": 1, "sr": nil, "tir": nil}},
		{"Someday", func() map[string]any { return newTaskUpdate().Someday().build() },
			map[string]any{"st": 2, "sr": nil, "tir": nil}},
		{"Inbox", func() map[string]any { return newTaskUpdate().Inbox().build() },
			map[string]any{"st": 0, "sr": nil, "tir": nil}},
		{"ScheduleDate", func() map[string]any { return newTaskUpdate().ScheduleDate(1760000000).build() },
			map[string]any{"st": 1, "sr": int64(1760000000), "tir": int64(1760000000)}},
		{"Deadline", func() map[string]any { return newTaskUpdate().Deadline(1770000000).build() },
			map[string]any{"dd": int64(1770000000)}},
		{"Scheduled", func() map[string]any { return newTaskUpdate().Scheduled(100, 200).build() },
			map[string]any{"sr": int64(100), "tir": int64(200)}},
		{"Area", func() map[string]any { return newTaskUpdate().Area("area").build() },
			map[string]any{"ar": []string{"area"}, "pr": []string{}, "agr": []string{}}},
		{"Project", func() map[string]any { return newTaskUpdate().Project("project").build() },
			map[string]any{"pr": []string{"project"}, "ar": []string{}, "agr": []string{}}},
		{"Tags", func() map[string]any { return newTaskUpdate().Tags([]string{"a1", "b2"}).build() },
			map[string]any{"tg": []string{"a1", "b2"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.build()
			// Every update payload must carry a modification date (md is
			// null only on CREATES — updates require a timestamp).
			md, ok := got["md"].(float64)
			if !ok || md <= 0 {
				t.Errorf("md = %v, want positive float timestamp on updates", got["md"])
			}
			for k, want := range tc.want {
				gotV, exists := got[k]
				if !exists {
					t.Errorf("field %q missing from payload %v", k, got)
					continue
				}
				if fmt.Sprintf("%v", gotV) != fmt.Sprintf("%v", want) {
					t.Errorf("field %q = %v, want %v", k, gotV, want)
				}
			}
			// No extra fields beyond md + the expected set.
			if len(got) != len(tc.want)+1 {
				t.Errorf("payload has %d fields, want %d: %v", len(got), len(tc.want)+1, got)
			}
		})
	}
}

func TestTaskUpdateNoteAndClearNote(t *testing.T) {
	note := newTaskUpdate().Note("hello").build()
	wn, ok := note["nt"].(WireNote)
	if !ok {
		t.Fatalf("nt = %T, want WireNote", note["nt"])
	}
	if wn.Value != "hello" || wn.TypeTag != "tx" || wn.Type != 1 {
		t.Errorf("Note wire form = %+v", wn)
	}
	if wn.Checksum != noteChecksum("hello") {
		t.Errorf("Note checksum = %d, want %d", wn.Checksum, noteChecksum("hello"))
	}

	cleared := newTaskUpdate().ClearNote().build()
	cn := cleared["nt"].(WireNote)
	if cn.Value != "" || cn.Checksum != 0 {
		t.Errorf("ClearNote wire form = %+v, want empty value and zero checksum", cn)
	}
}

// ---------------------------------------------------------------------------
// Notes and dates
// ---------------------------------------------------------------------------

func TestNoteChecksumIsCRC32IEEE(t *testing.T) {
	// crc32.ChecksumIEEE("hello") is a fixed, well-known value.
	if got := noteChecksum("hello"); got != 0x3610a686 {
		t.Errorf("noteChecksum(hello) = %#x, want 0x3610a686 (CRC-32/IEEE)", got)
	}
	if got := noteChecksum(""); got != 0 {
		t.Errorf("noteChecksum(\"\") = %d, want 0", got)
	}
}

func TestWireNoteJSONFieldOrder(t *testing.T) {
	// Things expects the note object keys in _t, ch, v, t order.
	bs, err := json.Marshal(textNote("x"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(bs)
	order := []string{`"_t"`, `"ch"`, `"v"`, `"t"`}
	last := -1
	for _, k := range order {
		i := strings.Index(s, k)
		if i < 0 || i < last {
			t.Fatalf("note JSON %s does not have keys in order %v", s, order)
		}
		last = i
	}
}

func TestTodayMidnightUTC(t *testing.T) {
	got := todayMidnightUTC()
	if got%86400 != 0 {
		t.Errorf("todayMidnightUTC() = %d, not aligned to a UTC day boundary", got)
	}
	now := time.Now().UTC()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix()
	// Allow the local-vs-UTC day to differ by one day at most (the function
	// intentionally uses the local calendar date at UTC midnight).
	if got != want && got != want-86400 && got != want+86400 {
		t.Errorf("todayMidnightUTC() = %d, want within a day of %d", got, want)
	}
}

func TestParseDate(t *testing.T) {
	if ts := parseDate("2026-07-06"); ts == nil || ts.Format("2006-01-02") != "2026-07-06" {
		t.Errorf("parseDate(2026-07-06) = %v", ts)
	}
	for _, bad := range []string{"07/06/2026", "yesterday", "", "2026-13-45"} {
		if ts := parseDate(bad); ts != nil {
			t.Errorf("parseDate(%q) = %v, want nil", bad, ts)
		}
	}
}

func TestParseArgs(t *testing.T) {
	got := parseArgs([]string{"--title", "My Task", "--today", "--tags", "a,b", "--flag-at-end"})
	want := map[string]string{"title": "My Task", "today": "true", "tags": "a,b", "flag-at-end": "true"}
	if len(got) != len(want) {
		t.Fatalf("parseArgs = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseArgs[%q] = %q, want %q", k, got[k], v)
		}
	}
}

// ---------------------------------------------------------------------------
// Golden shape: the 34-field create payload
// ---------------------------------------------------------------------------

func TestTaskCreatePayloadGoldenShape(t *testing.T) {
	bs, err := json.Marshal(newTaskCreatePayload("shape check", map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bs, &m); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{
		"tp", "sr", "dds", "rt", "rmd", "ss", "tr", "dl", "icp", "st",
		"ar", "tt", "do", "lai", "tir", "tg", "agr", "ix", "cd", "lt",
		"icc", "md", "ti", "dd", "ato", "nt", "icsd", "pr", "rp", "acrd",
		"sp", "sb", "rr", "xx",
	}
	var gotKeys []string
	for k := range m {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sorted := append([]string(nil), wantKeys...)
	sort.Strings(sorted)
	if strings.Join(gotKeys, ",") != strings.Join(sorted, ",") {
		t.Fatalf("create payload keys =\n  %v\nwant exactly the 34 HAR-derived fields:\n  %v", gotKeys, sorted)
	}

	// Fields that MUST be null on a bare create — 0 here means the Unix
	// epoch and md-on-create corrupts sync.
	for _, k := range []string{"sr", "tir", "dd", "md", "sp", "rr", "rp", "lai", "ato", "icsd", "acrd", "dds", "rmd"} {
		if string(m[k]) != "null" {
			t.Errorf("create payload %q = %s, want null", k, m[k])
		}
	}
	// Empty arrays, not null.
	for _, k := range []string{"rt", "dl", "ar", "tg", "agr", "pr"} {
		if string(m[k]) != "[]" {
			t.Errorf("create payload %q = %s, want []", k, m[k])
		}
	}
	if string(m["st"]) != "0" || string(m["ss"]) != "0" || string(m["tp"]) != "0" {
		t.Errorf("bare create defaults: st=%s ss=%s tp=%s, want all 0", m["st"], m["ss"], m["tp"])
	}
}

// ---------------------------------------------------------------------------
// Handler wire tests — drive the real cmd* handlers against a fake server
// and assert the exact bytes that would reach Things Cloud.
// ---------------------------------------------------------------------------

type capturedCommit struct {
	ancestorIndex string
	body          map[string]struct {
		T int             `json:"t"`
		E string          `json:"e"`
		P json.RawMessage `json:"p"`
	}
}

// newWireRecorder returns a History wired to a fake server plus slices
// collecting history preflight GETs and every commit POSTed through it.
func newWireRecorder(t *testing.T, ordinaryTaskIDs ...string) (*thingscloud.History, *[]capturedCommit, *[]string) {
	t.Helper()
	var commits []capturedCommit
	var historyRequests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/commit"):
			var cc capturedCommit
			cc.ancestorIndex = r.URL.Query().Get("ancestor-index")
			if err := json.NewDecoder(r.Body).Decode(&cc.body); err != nil {
				t.Errorf("commit body did not parse: %v", err)
			}
			commits = append(commits, cc)
			fmt.Fprint(w, `{"server-head-index":42}`)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/items"):
			historyRequests = append(historyRequests, r.URL.RequestURI())
			if got := r.URL.Query().Get("start-index"); got != "0" {
				t.Errorf("preflight start-index = %q, want 0", got)
			}
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
			items := make([]map[string]any, 7)
			items[0] = created
			for i := 1; i < len(items); i++ {
				items[i] = map[string]any{}
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"items":              items,
				"current-item-index": 7,
				"schema":             301,
			}); err != nil {
				t.Errorf("encode history fixture: %v", err)
			}
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)

	c := thingscloud.New(server.URL, "test@example.com", "pw")
	h := &thingscloud.History{Client: c, ID: "wire-test-history", LatestServerIndex: 7}
	return h, &commits, &historyRequests
}

func assertTaskPreflight(t *testing.T, historyRequests []string, commit capturedCommit) {
	t.Helper()
	want := "/version/1/history/wire-test-history/items?start-index=0"
	if len(historyRequests) != 1 || historyRequests[0] != want {
		t.Fatalf("history preflight requests = %q, want [%q]", historyRequests, want)
	}
	if commit.ancestorIndex != "7" {
		t.Errorf("ancestor-index = %q, want preflight head 7", commit.ancestorIndex)
	}
}

func singleItem(t *testing.T, cc capturedCommit) (uuid string, action int, kind string, payload map[string]any) {
	t.Helper()
	if len(cc.body) != 1 {
		t.Fatalf("commit has %d items, want 1: %v", len(cc.body), cc.body)
	}
	for id, item := range cc.body {
		var p map[string]any
		if err := json.Unmarshal(item.P, &p); err != nil {
			t.Fatalf("payload not an object: %v", err)
		}
		return id, item.T, item.E, p
	}
	panic("unreachable")
}

func TestCmdCreateWire(t *testing.T) {
	h, commits, historyRequests := newWireRecorder(t)
	id := thingscloud.NewUUID()

	cmdCreate(h, []string{"Wire task", "--uuid", id, "--when", "today", "--note", "body"})

	if len(*commits) != 1 {
		t.Fatalf("%d commits, want 1", len(*commits))
	}
	if len(*historyRequests) != 0 {
		t.Fatalf("Task7 create made history preflight requests: %q", *historyRequests)
	}
	cc := (*commits)[0]
	if cc.ancestorIndex != "7" {
		t.Errorf("ancestor-index = %q, want 7 (the synced head)", cc.ancestorIndex)
	}
	gotID, action, kind, p := singleItem(t, cc)
	if gotID != id || action != 0 || kind != "Task7" {
		t.Errorf("envelope = (%s, %d, %s), want (%s, 0, Task7)", gotID, action, kind, id)
	}
	if p["md"] != nil {
		t.Errorf("create sent md = %v, must be null on creates", p["md"])
	}
	if p["st"] != float64(1) || p["sr"] == nil || p["tir"] == nil {
		t.Errorf("--when today: st=%v sr=%v tir=%v", p["st"], p["sr"], p["tir"])
	}
	nt := p["nt"].(map[string]any)
	if nt["v"] != "body" {
		t.Errorf("note value = %v", nt["v"])
	}
}

func TestCmdCompleteWire(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)

	cmdComplete(h, id)

	_, action, kind, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if action != 1 || kind != "Task7" {
		t.Errorf("envelope action/kind = %d/%s, want 1/Task7", action, kind)
	}
	if p["ss"] != float64(3) {
		t.Errorf("ss = %v, want 3 (completed)", p["ss"])
	}
	if sp, ok := p["sp"].(float64); !ok || sp <= 0 {
		t.Errorf("sp = %v, want completion timestamp", p["sp"])
	}
	if md, ok := p["md"].(float64); !ok || md <= 0 {
		t.Errorf("md = %v, want modification timestamp on update", p["md"])
	}
}

func TestCmdTrashWire(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)

	cmdTrash(h, id)

	_, action, kind, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if action != 1 || kind != "Task7" || p["tr"] != true {
		t.Errorf("trash wire = action %d kind %s tr %v", action, kind, p["tr"])
	}
}

func TestCmdPurgeWire(t *testing.T) {
	h, commits, _ := newWireRecorder(t)
	target := thingscloud.NewUUID()

	cmdPurge(h, target)

	tombID, action, kind, p := singleItem(t, (*commits)[0])
	if action != 0 || kind != "Tombstone2" {
		t.Errorf("purge envelope = action %d kind %s, want 0/Tombstone2", action, kind)
	}
	if err := thingscloud.ValidateUUID(tombID); err != nil {
		t.Errorf("tombstone UUID %q not canonical: %v", tombID, err)
	}
	if p["dloid"] != target {
		t.Errorf("dloid = %v, want %s", p["dloid"], target)
	}
	if dld, ok := p["dld"].(float64); !ok || dld <= 0 {
		t.Errorf("dld = %v, want deletion timestamp", p["dld"])
	}
}

func TestCmdMoveToTodayWire(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)

	cmdMoveToToday(h, id)

	_, _, _, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if p["st"] != float64(1) {
		t.Errorf("st = %v, want 1", p["st"])
	}
	if p["sr"] == nil || p["tir"] == nil || p["sr"] != p["tir"] {
		t.Errorf("sr/tir = %v/%v, want equal midnight timestamps", p["sr"], p["tir"])
	}
}

func TestCmdCreateAreaWire(t *testing.T) {
	h, commits, _ := newWireRecorder(t)
	id := thingscloud.NewUUID()

	cmdCreateArea(h, []string{"Wire Area", "--uuid", id})

	gotID, action, kind, p := singleItem(t, (*commits)[0])
	// Area2 is silently ignored by Things.app — Area3 is load-bearing.
	if gotID != id || action != 0 || kind != "Area3" {
		t.Errorf("area envelope = (%s, %d, %s), want (%s, 0, Area3)", gotID, action, kind, id)
	}
	if p["tt"] != "Wire Area" {
		t.Errorf("tt = %v", p["tt"])
	}
}

func TestCmdCreateTagWire(t *testing.T) {
	h, commits, _ := newWireRecorder(t)
	id := thingscloud.NewUUID()

	cmdCreateTag(h, []string{"Wire Tag", "--uuid", id})

	gotID, action, kind, p := singleItem(t, (*commits)[0])
	if gotID != id || action != 0 || kind != "Tag4" {
		t.Errorf("tag envelope = (%s, %d, %s), want (%s, 0, Tag4)", gotID, action, kind, id)
	}
	if p["sh"] != nil {
		t.Errorf("sh = %v, want null when no shorthand given", p["sh"])
	}
}

func TestCmdAddChecklistWire(t *testing.T) {
	h, commits, _ := newWireRecorder(t)
	task := thingscloud.NewUUID()

	cmdAddChecklist(h, task, []string{"one,two"})

	if len(*commits) != 2 {
		t.Fatalf("%d commits, want 2 (one per checklist item)", len(*commits))
	}
	for i, cc := range *commits {
		itemID, action, kind, p := singleItem(t, cc)
		if action != 0 || kind != "ChecklistItem3" {
			t.Errorf("item %d envelope = %d/%s, want 0/ChecklistItem3", i, action, kind)
		}
		if err := thingscloud.ValidateUUID(itemID); err != nil {
			t.Errorf("checklist item UUID %q not canonical: %v", itemID, err)
		}
		ts := p["ts"].([]any)
		if len(ts) != 1 || ts[0] != task {
			t.Errorf("item %d ts = %v, want [%s]", i, p["ts"], task)
		}
		if p["md"] != nil {
			t.Errorf("item %d md = %v, must be null on creates", i, p["md"])
		}
		if p["ix"] != float64(i) {
			t.Errorf("item %d ix = %v, want %d", i, p["ix"], i)
		}
	}
}

func TestCmdEditWire(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)
	proj := thingscloud.NewUUID()

	cmdEdit(h, id, []string{"--title", "renamed", "--project", proj})

	gotID, action, kind, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if gotID != id || action != 1 || kind != "Task7" {
		t.Errorf("edit envelope = (%s, %d, %s)", gotID, action, kind)
	}
	if p["tt"] != "renamed" {
		t.Errorf("tt = %v", p["tt"])
	}
	pr := p["pr"].([]any)
	if len(pr) != 1 || pr[0] != proj {
		t.Errorf("pr = %v, want [%s]", p["pr"], proj)
	}
	if ar, ok := p["ar"].([]any); !ok || len(ar) != 0 {
		t.Errorf("ar = %v, want [] when moving to a project", p["ar"])
	}
	if agr, ok := p["agr"].([]any); !ok || len(agr) != 0 {
		t.Errorf("agr = %v, want [] when moving to a project", p["agr"])
	}
	// Assigning a project without an explicit schedule must move the task
	// out of inbox with null dates (Bug 8 + PR #9 semantics).
	if p["st"] != float64(1) || p["sr"] != nil || p["tir"] != nil {
		t.Errorf("auto-anytime on edit: st=%v sr=%v tir=%v, want 1/null/null", p["st"], p["sr"], p["tir"])
	}
}

func TestCmdEditFutureScheduledWire(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)
	proj := thingscloud.NewUUID()
	future := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	wantDate := float64(parseDate(future).Unix())

	cmdEdit(h, id, []string{"--scheduled", future, "--project", proj})

	gotID, action, kind, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if gotID != id || action != 1 || kind != "Task7" {
		t.Fatalf("edit envelope = (%s, %d, %s), want (%s, 1, Task7)", gotID, action, kind, id)
	}
	if p["st"] != float64(2) || p["sr"] != wantDate || p["tir"] != wantDate {
		t.Fatalf("future schedule = st:%v sr:%v tir:%v, want 2/%v/%v", p["st"], p["sr"], p["tir"], wantDate, wantDate)
	}
	pr, ok := p["pr"].([]any)
	if !ok || len(pr) != 1 || pr[0] != proj {
		t.Fatalf("pr = %v, want [%s]", p["pr"], proj)
	}
	if len(p) != 7 {
		t.Fatalf("edit payload fields = %v, want only md, st, sr, tir, pr, ar, agr", p)
	}
}

func TestCmdEditExplicitWhenOverridesScheduledClassification(t *testing.T) {
	id := thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id)
	area := thingscloud.NewUUID()
	future := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	wantDate := float64(parseDate(future).Unix())

	cmdEdit(h, id, []string{"--scheduled", future, "--when", "anytime", "--area", area})

	_, _, _, p := singleItem(t, (*commits)[0])
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	if p["st"] != float64(1) || p["sr"] != wantDate || p["tir"] != wantDate {
		t.Fatalf("explicit anytime schedule = st:%v sr:%v tir:%v, want 1/%v/%v", p["st"], p["sr"], p["tir"], wantDate, wantDate)
	}
	areas, ok := p["ar"].([]any)
	if !ok || len(areas) != 1 || areas[0] != area {
		t.Fatalf("ar = %v, want [%s]", p["ar"], area)
	}
	if pr, ok := p["pr"].([]any); !ok || len(pr) != 0 {
		t.Errorf("pr = %v, want [] when moving to an area", p["pr"])
	}
	if agr, ok := p["agr"].([]any); !ok || len(agr) != 0 {
		t.Errorf("agr = %v, want [] when moving to an area", p["agr"])
	}
}

// ---------------------------------------------------------------------------
// initCLI / loadState / cmdBatch — full command plumbing against a fake server
// ---------------------------------------------------------------------------

// fakeCloud spins up a server implementing enough of the Things Cloud API
// for initCLI + read/write flows: verify, own-history, history head, items.
func fakeCloud(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/commit"):
			fmt.Fprint(w, `{"server-head-index":5}`)
		case strings.HasSuffix(r.URL.Path, "/items"):
			fmt.Fprint(w, `{"items":[{"VJ1edXTP9q3PmFDUuy8EQh":{"e":"Task6","t":0,"p":{"tt":"seeded","tp":0,"st":1,"ss":0}}}],"current-item-index":1,"schema":301}`)
		case strings.Contains(r.URL.Path, "/version/1/history/"):
			fmt.Fprint(w, `{"latest-server-index":1,"latest-schema-version":301}`)
		case strings.Contains(r.URL.Path, "/account/"):
			fmt.Fprint(w, `{"email":"t@example.com","status":"SYAccountStatusActive","history-key":"hist-1"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestInitCLIUsesEndpointOverride(t *testing.T) {
	server := fakeCloud(t)
	t.Setenv("THINGS_ENDPOINT", server.URL)
	t.Setenv("THINGS_USERNAME", "t@example.com")
	t.Setenv("THINGS_PASSWORD", "pw")

	ctx := initCLI(true)
	if ctx.history == nil || ctx.history.ID != "hist-1" {
		t.Fatalf("initCLI history = %+v, want ID hist-1 from fake server", ctx.history)
	}
	if got := ctx.serverIndex(); got != 1 {
		t.Errorf("serverIndex() = %d, want 1", got)
	}
}

func TestLoadStateBuildsAndCachesState(t *testing.T) {
	server := fakeCloud(t)
	t.Setenv("THINGS_ENDPOINT", server.URL)
	t.Setenv("THINGS_USERNAME", "t@example.com")
	t.Setenv("THINGS_PASSWORD", "pw")
	cachePath := t.TempDir() + "/cache.json"
	t.Setenv("THINGS_CLI_CACHE", cachePath)

	ctx := initCLI(false)
	state := ctx.loadState()
	if state.Tasks["VJ1edXTP9q3PmFDUuy8EQh"] == nil {
		t.Fatal("loadState did not aggregate the seeded task")
	}

	cache, err := loadCLIStateCache(cachePath)
	if err != nil || cache == nil {
		t.Fatalf("cache not written: %v %v", cache, err)
	}
	if cache.ServerIndex != 1 || cache.HistoryID != "hist-1" {
		t.Errorf("cache = index %d history %q, want 1/hist-1", cache.ServerIndex, cache.HistoryID)
	}

	// Second load must serve from the cache without rebuilding from zero.
	state2 := ctx.loadState()
	if state2.Tasks["VJ1edXTP9q3PmFDUuy8EQh"] == nil {
		t.Fatal("cached loadState lost the task")
	}
}

func TestCmdBatchWire(t *testing.T) {
	id1, id2 := thingscloud.NewUUID(), thingscloud.NewUUID()
	h, commits, historyRequests := newWireRecorder(t, id2)

	// cmdBatch reads ops from stdin.
	ops := fmt.Sprintf(`[
		{"cmd":"create","title":"batch A","uuid":%q},
		{"cmd":"complete","uuid":%q}
	]`, id1, id2)
	r, w, _ := os.Pipe()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
	go func() { _, _ = w.WriteString(ops); w.Close() }()

	cmdBatch(h)

	if len(*commits) != 1 {
		t.Fatalf("%d commits, want 1 — batch must send all ops in a single HTTP request", len(*commits))
	}
	assertTaskPreflight(t, *historyRequests, (*commits)[0])
	body := (*commits)[0].body
	if len(body) != 2 {
		t.Fatalf("commit has %d items, want 2", len(body))
	}
	create, complete := body[id1], body[id2]
	if create.E != "Task7" || create.T != 0 {
		t.Errorf("create envelope = %s/%d, want Task7/0", create.E, create.T)
	}
	if complete.E != "Task7" || complete.T != 1 {
		t.Errorf("complete envelope = %s/%d, want Task7/1", complete.E, complete.T)
	}
	var p map[string]any
	if err := json.Unmarshal(complete.P, &p); err != nil || p["ss"] != float64(3) {
		t.Errorf("complete payload ss = %v (err %v), want 3", p["ss"], err)
	}
}

// ---------------------------------------------------------------------------
// Read commands — render a known state and assert the JSON output
// ---------------------------------------------------------------------------

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestReadCommandsRenderState(t *testing.T) {
	state := testStateForListFilters()

	out := captureStdout(t, func() { cmdList(state, []string{"--today"}) })
	if !strings.Contains(out, "today-1") {
		t.Errorf("list --today output missing today-1:\n%s", out)
	}

	out = captureStdout(t, func() { cmdSearch(state, []string{"needle"}) })
	if !strings.Contains(out, "upcoming-1") {
		t.Errorf("search output missing upcoming-1:\n%s", out)
	}

	out = captureStdout(t, func() { cmdShow(state, "today-1") })
	if !strings.Contains(out, `"uuid"`) || !strings.Contains(out, "today-1") {
		t.Errorf("show output malformed:\n%s", out)
	}

	out = captureStdout(t, func() { cmdAreas(state) })
	if !strings.Contains(out, "Work") {
		t.Errorf("areas output missing Work:\n%s", out)
	}

	out = captureStdout(t, func() { cmdProjects(state) })
	if !strings.Contains(out, "Project Alpha") {
		t.Errorf("projects output missing Project Alpha:\n%s", out)
	}

	out = captureStdout(t, func() { cmdTags(state) })
	var tags []map[string]any
	if err := json.Unmarshal([]byte(out), &tags); err != nil {
		t.Errorf("tags output is not a JSON array: %v\n%s", err, out)
	}
}
