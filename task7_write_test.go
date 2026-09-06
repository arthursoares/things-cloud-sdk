package thingscloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type rawWriteProbe struct {
	id   string
	body string
}

func (p rawWriteProbe) UUID() string { return p.id }

func (p rawWriteProbe) MarshalJSON() ([]byte, error) { return []byte(p.body), nil }

func task7CreateBody(title string) string {
	return fmt.Sprintf(`{"e":"Task7","t":0,"p":{"tp":0,"sr":null,"dds":null,"rt":[],"rmd":null,"ss":0,"tr":false,"dl":[],"icp":false,"st":0,"ar":[],"tt":%s,"do":0,"lai":null,"tir":null,"tg":[],"agr":[],"ix":0,"cd":1770000000.25,"lt":false,"icc":0,"md":null,"ti":0,"dd":null,"ato":null,"nt":{"_t":"tx","ch":0,"v":"","t":1},"icsd":null,"pr":[],"rp":null,"acrd":null,"sp":null,"sb":0,"rr":null,"xx":{"sn":{},"_t":"oo"}}}`, strconv.Quote(title))
}

func task7UpdateBody(fields string) string {
	if fields != "" {
		fields = "," + fields
	}
	return `{"e":"Task7","t":1,"p":{"md":1770000001.5` + fields + `}}`
}

func historyPage(head int, batches ...string) string {
	return fmt.Sprintf(`{"items":[%s],"current-item-index":%d,"schema":301}`, strings.Join(batches, ","), head)
}

func taskHistoryBatch(id string, kind ItemKind, action int, payload string) string {
	return fmt.Sprintf(`{%s:{"e":%s,"t":%d,"p":%s}}`, strconv.Quote(id), strconv.Quote(string(kind)), action, payload)
}

func TestHistoryWriteAcceptsVerifiedTask7CreateWithoutPreflight(t *testing.T) {
	var methods []string
	var posted map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected %s request for create-only write", r.Method)
		}
		if got := r.URL.Query().Get("ancestor-index"); got != "9" {
			t.Errorf("ancestor-index = %q, want 9", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(w, `{"server-head-index":10}`)
	}))
	defer server.Close()

	id := NewUUID()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	h.LatestServerIndex = 9
	if err := h.Write(rawWriteProbe{id: id, body: task7CreateBody("ordinary")}); err != nil {
		t.Fatalf("Write(valid Task7 create): %v", err)
	}
	if len(methods) != 1 || methods[0] != http.MethodPost {
		t.Fatalf("methods = %v, want [POST]", methods)
	}
	if !strings.Contains(string(posted[id]), `"e":"Task7"`) {
		t.Fatalf("posted envelope = %s, want Task7", posted[id])
	}
}

func TestHistoryWriteTask7UpdatePreflightsRawHistoryAndPinsAncestor(t *testing.T) {
	target := NewUUID()
	var starts []string
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			starts = append(starts, r.URL.Query().Get("start-index"))
			switch r.URL.Query().Get("start-index") {
			case "0":
				fmt.Fprint(w, historyPage(3,
					taskHistoryBatch(target, ItemKindTask, 0, `{"tt":"legacy generation","rr":null,"rp":null,"rt":[]}`),
					taskHistoryBatch(target, ItemKindTask7, 1, `{"tt":"current generation"}`),
				))
			case "2":
				fmt.Fprint(w, historyPage(4, `{}`)) // growing head must not be chased
			default:
				t.Fatalf("unexpected start-index %q", r.URL.Query().Get("start-index"))
			}
		case http.MethodPost:
			posts++
			if got := r.URL.Query().Get("ancestor-index"); got != "3" {
				t.Errorf("ancestor-index = %q, want frozen head 3", got)
			}
			fmt.Fprint(w, `{"server-head-index":4}`)
		}
	}))
	defer server.Close()

	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	h.LatestServerIndex = 99
	if err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"renamed"`)}); err != nil {
		t.Fatalf("Write(valid ordinary update): %v", err)
	}
	if strings.Join(starts, ",") != "0,2" || posts != 1 {
		t.Fatalf("starts=%v posts=%d, want [0 2] and one POST", starts, posts)
	}
	if h.LatestServerIndex != 4 {
		t.Errorf("LatestServerIndex=%d, want commit head 4", h.LatestServerIndex)
	}
}

func TestHistoryWriteRejectsRecurringAndAmbiguousTask7TargetsBeforePost(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "repeater rule", payload: `{"rr":{"f":1},"rp":null,"rt":[]}`},
		{name: "repeater payload", payload: `{"rr":null,"rp":"opaque","rt":[]}`},
		{name: "linked instance", payload: `{"rr":null,"rp":null,"rt":["` + NewUUID() + `"]}`},
		{name: "instance creation marker", payload: `{"rr":null,"rp":null,"rt":[],"icsd":1770000000}`},
		{name: "after completion marker", payload: `{"rr":null,"rp":null,"rt":[],"acrd":1770000000}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewUUID()
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				fmt.Fprint(w, historyPage(1, taskHistoryBatch(target, ItemKindTask7, 0, tt.payload)))
			}))
			defer server.Close()
			h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
			if err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"unsafe"`)}); err == nil {
				t.Fatal("Write(recurring target) succeeded")
			}
			if posts != 0 {
				t.Fatalf("sent %d POSTs, want zero", posts)
			}
		})
	}
}

func TestHistoryWriteTask7PreflightTracksOmissionsNullClearsDeletesAndKinds(t *testing.T) {
	tests := []struct {
		name    string
		batches func(string) []string
		wantOK  bool
	}{
		{name: "omission preserves recurrence", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKindTask7, 0, `{"rr":{"f":1},"rp":null,"rt":[]}`),
				taskHistoryBatch(id, ItemKindTask7, 1, `{"tt":"still recurring"}`),
			}
		}},
		{name: "explicit null clears recurrence", wantOK: true, batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKindTask7, 0, `{"rr":{"f":1},"rp":"opaque","rt":["`+NewUUID()+`"],"icsd":1,"acrd":2}`),
				taskHistoryBatch(id, ItemKindTask7, 1, `{"rr":null,"rp":null,"rt":[],"icsd":null,"acrd":null}`),
			}
		}},
		{name: "legacy sparse create lacks recurrence proof", batches: func(id string) []string {
			return []string{taskHistoryBatch(id, ItemKind("Task6"), 0, `{"tt":"unknown recurrence state"}`)}
		}},
		{name: "direct task deletion", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKindTask7, 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(id, ItemKindTask7, 2, `{}`),
			}
		}},
		{name: "tombstone deletion", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKindTask7, 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(NewUUID(), ItemKindTombstone, 0, `{"dloid":"`+id+`","dld":1}`),
			}
		}},
		{name: "future target kind", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKind("Task8"), 0, `{"rr":null,"rp":null,"rt":[]}`),
			}
		}},
		{name: "future kind hidden by later create", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKind("Task8"), 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":null,"rp":null,"rt":[]}`),
			}
		}},
		{name: "non-task modification on target", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(id, ItemKind("Area3"), 1, `{}`),
			}
		}},
		{name: "duplicate target creation", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":null,"rp":null,"rt":[]}`),
			}
		}},
		{name: "missing target", batches: func(string) []string { return []string{`{}`} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewUUID()
			batches := tt.batches(target)
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					fmt.Fprint(w, `{"server-head-index":20}`)
					return
				}
				fmt.Fprint(w, historyPage(len(batches), batches...))
			}))
			defer server.Close()
			h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
			err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"update"`)})
			if (err == nil) != tt.wantOK {
				t.Fatalf("Write() error=%v, wantOK=%v", err, tt.wantOK)
			}
			if (posts == 1) != tt.wantOK {
				t.Fatalf("posts=%d, wantOK=%v", posts, tt.wantOK)
			}
		})
	}
}

func TestHistoryWriteValidatesEntireTask7BatchBeforeNetwork(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown create field", body: strings.Replace(task7CreateBody("x"), `"xx":`, `"unknown":1,"xx":`, 1)},
		{name: "create recurrence", body: strings.Replace(task7CreateBody("x"), `"rr":null`, `"rr":{"f":1}`, 1)},
		{name: "inbox create with dates", body: strings.NewReplacer(`"sr":null`, `"sr":1770000002`, `"tir":null`, `"tir":1770000002`).Replace(task7CreateBody("x"))},
		{name: "create dates differ", body: strings.NewReplacer(`"st":0`, `"st":1`, `"sr":null`, `"sr":1770000002`, `"tir":null`, `"tir":1770000003`).Replace(task7CreateBody("x"))},
		{name: "create dates null mismatch", body: strings.NewReplacer(`"st":0`, `"st":1`, `"sr":null`, `"sr":1770000002`).Replace(task7CreateBody("x"))},
		{name: "multiple area parents", body: strings.Replace(task7CreateBody("x"), `"ar":[]`, `"ar":["`+NewUUID()+`","`+NewUUID()+`"]`, 1)},
		{name: "malformed modification date", body: task7UpdateBody(`"tt":"x"`)[:len(task7UpdateBody(`"tt":"x"`))-2] + `,"md":"bad"}}`},
		{name: "delta note", body: task7UpdateBody(`"nt":{"_t":"tx","t":2,"ps":[]}`)},
		{name: "unknown update field", body: task7UpdateBody(`"private":"fixture"`)},
		{name: "deleted action", body: `{"e":"Task7","t":2,"p":{}}`},
		{name: "quoted action", body: strings.Replace(task7CreateBody("x"), `"t":0`, `"t":"0"`, 1)},
		{name: "quoted payload integer", body: strings.Replace(task7CreateBody("x"), `"tp":0`, `"tp":"0"`, 1)},
		{name: "null title", body: task7UpdateBody(`"tt":null`)},
		{name: "null trash", body: task7UpdateBody(`"tr":null`)},
		{name: "null create boolean", body: strings.Replace(task7CreateBody("x"), `"icp":false`, `"icp":null`, 1)},
		{name: "null note value", body: strings.Replace(task7CreateBody("x"), `"v":""`, `"v":null`, 1)},
		{name: "status without stop date", body: task7UpdateBody(`"ss":3`)},
		{name: "stop date without status", body: task7UpdateBody(`"sp":1770000002`)},
		{name: "reopen with stop date", body: task7UpdateBody(`"ss":0,"sp":1770000002`)},
		{name: "complete with null stop date", body: task7UpdateBody(`"ss":3,"sp":null`)},
		{name: "schedule without dates", body: task7UpdateBody(`"st":1`)},
		{name: "schedule dates differ", body: task7UpdateBody(`"st":1,"sr":1770000002,"tir":1770000003`)},
		{name: "inbox with dates", body: task7UpdateBody(`"st":0,"sr":1770000002,"tir":1770000002`)},
		{name: "unpaired surrogate", body: strings.Replace(task7CreateBody("x"), `"tt":"x"`, `"tt":"\uD800"`, 1)},
		{name: "duplicate recurrence field", body: strings.Replace(task7CreateBody("x"), `"rr":null`, `"rr":{"f":1},"rr":null`, 1)},
		{name: "duplicate nested note field", body: strings.Replace(task7CreateBody("x"), `"_t":"tx"`, `"_t":"private","_t":"tx"`, 1)},
		{name: "duplicate envelope kind", body: strings.Replace(task7CreateBody("x"), `{"e":"Task7"`, `{"e":"Tag4","e":"Task7"`, 1)},
		{name: "missing payload", body: `{"e":"Task7","t":1}`},
		{name: "payload not object", body: `{"e":"Task7","t":1,"p":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
			defer server.Close()
			h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
			err := h.Write(
				rawWriteProbe{id: NewUUID(), body: task7CreateBody("valid sibling")},
				rawWriteProbe{id: NewUUID(), body: tt.body},
			)
			if err == nil {
				t.Fatal("Write(invalid Task7 batch) succeeded")
			}
			if requests != 0 {
				t.Fatalf("sent %d requests, want zero", requests)
			}
		})
	}
}

func TestHistoryWriteAcceptsVerifiedTask7FutureCreate(t *testing.T) {
	body := strings.NewReplacer(
		`"st":0`, `"st":2`,
		`"sr":null`, `"sr":1770000002`,
		`"tir":null`, `"tir":1770000002`,
	).Replace(task7CreateBody("future"))
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("future create sent %s, want POST", r.Method)
		}
		posts++
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: NewUUID(), body: body}); err != nil {
		t.Fatalf("Write(valid future Task7 create): %v", err)
	}
	if posts != 1 {
		t.Fatalf("posts=%d, want one", posts)
	}
}

func TestHistoryWriteRejectsInvalidUTF8Task7BeforeNetwork(t *testing.T) {
	body := strings.Replace(task7CreateBody("x"), `"tt":"x"`, "\"tt\":\"\xff\"", 1)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: NewUUID(), body: body}); err == nil {
		t.Fatal("Write(invalid UTF-8 Task7) succeeded")
	}
	if requests != 0 {
		t.Fatalf("sent %d requests, want zero", requests)
	}
}

func TestHistoryWriteAcceptsPairedJSONSurrogate(t *testing.T) {
	body := strings.Replace(task7CreateBody("x"), `"tt":"x"`, `"tt":"\uD83D\uDE80"`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: NewUUID(), body: body}); err != nil {
		t.Fatalf("Write(valid paired surrogate): %v", err)
	}
}

func TestHistoryWriteTask7RejectsMalformedCompanionTombstoneBeforeNetwork(t *testing.T) {
	target := NewUUID()
	tombstone := rawWriteProbe{
		id:   NewUUID(),
		body: `{"e":"Tombstone2","t":0,"p":{"dloid":"` + target + `","dloid":"` + NewUUID() + `","dld":1770000001}}`,
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: NewUUID(), body: task7CreateBody("x")}, tombstone); err == nil {
		t.Fatal("Write(Task7 plus malformed tombstone) succeeded")
	}
	if requests != 0 {
		t.Fatalf("sent %d requests, want zero", requests)
	}
}

func TestHistoryWriteTask7BatchRejectsOneRecurringTargetBeforeAllPosts(t *testing.T) {
	ordinary, recurring := NewUUID(), NewUUID()
	gets, posts := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			return
		}
		gets++
		fmt.Fprint(w, historyPage(2,
			taskHistoryBatch(ordinary, ItemKindTask7, 0, `{"rr":null,"rp":null,"rt":[]}`),
			taskHistoryBatch(recurring, ItemKindTask7, 0, `{"rr":{"f":1},"rp":null,"rt":[]}`),
		))
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	err := h.Write(
		rawWriteProbe{id: ordinary, body: task7UpdateBody(`"tt":"safe"`)},
		rawWriteProbe{id: recurring, body: task7UpdateBody(`"tt":"unsafe"`)},
	)
	if err == nil {
		t.Fatal("Write(mixed recurrence batch) succeeded")
	}
	if gets != 1 || posts != 0 {
		t.Fatalf("gets=%d posts=%d, want one shared GET and zero POSTs", gets, posts)
	}
}

func TestHistoryWriteTask7RejectsSameBatchTombstoneAmbiguityBeforeNetwork(t *testing.T) {
	target := NewUUID()
	tombstone := rawWriteProbe{
		id:   NewUUID(),
		body: `{"e":"Tombstone2","t":0,"p":{"dloid":"` + target + `","dld":1770000001}}`,
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"x"`)}, tombstone); err == nil {
		t.Fatal("Write(Task7 modification plus target tombstone) succeeded")
	}
	if requests != 0 {
		t.Fatalf("sent %d requests, want zero", requests)
	}
}

func TestHistoryWriteTask7RejectsSameBatchCreateAndTombstoneBeforeNetwork(t *testing.T) {
	target := NewUUID()
	tombstone := rawWriteProbe{
		id:   NewUUID(),
		body: `{"e":"Tombstone2","t":0,"p":{"dloid":"` + target + `","dld":1770000001}}`,
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(rawWriteProbe{id: target, body: task7CreateBody("x")}, tombstone); err == nil {
		t.Fatal("Write(Task7 create plus target tombstone) succeeded")
	}
	if requests != 0 {
		t.Fatalf("sent %d requests, want zero", requests)
	}
}

func TestHistoryWriteTask7RejectsAmbiguousSameHistoryIndexRegardlessOfMapOrder(t *testing.T) {
	target := NewUUID()
	tombstoneID := NewUUID()
	batch := fmt.Sprintf(`{%s:{"e":"Task7","t":0,"p":{"rr":null,"rp":null,"rt":[]}},%s:{"e":"Tombstone2","t":0,"p":{"dloid":%s,"dld":1}}}`,
		strconv.Quote(target), strconv.Quote(tombstoneID), strconv.Quote(target))
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			return
		}
		fmt.Fprint(w, historyPage(1, batch))
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	for i := 0; i < 25; i++ {
		if err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"x"`)}); err == nil {
			t.Fatalf("iteration %d accepted ambiguous same-index history", i)
		}
	}
	if posts != 0 {
		t.Fatalf("sent %d POSTs, want zero", posts)
	}
}

func TestHistoryWriteTask7RejectsAmbiguousRawHistoryPayloads(t *testing.T) {
	tests := []struct {
		name    string
		batches func(string) []string
	}{
		{name: "duplicate recurrence key", batches: func(id string) []string {
			return []string{taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":{"f":1},"rr":null,"rp":null,"rt":[]}`)}
		}},
		{name: "duplicate tombstone target", batches: func(id string) []string {
			return []string{
				taskHistoryBatch(id, ItemKind("Task7"), 0, `{"rr":null,"rp":null,"rt":[]}`),
				taskHistoryBatch(NewUUID(), ItemKind("Tombstone2"), 0, `{"dloid":"`+NewUUID()+`","dloid":"`+id+`","dld":1}`),
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewUUID()
			batches := tt.batches(target)
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					return
				}
				fmt.Fprint(w, historyPage(len(batches), batches...))
			}))
			defer server.Close()
			h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
			if err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"x"`)}); err == nil {
				t.Fatal("Write(ambiguous raw history) succeeded")
			}
			if posts != 0 {
				t.Fatalf("sent %d POSTs, want zero", posts)
			}
		})
	}
}

func TestTask7PreflightRejectsInvalidUTF8TargetPayload(t *testing.T) {
	target := NewUUID()
	states := map[string]*task7TargetState{target: {lastAffectedIndex: -1}}
	item := Item{
		UUID:           target,
		Kind:           ItemKind("Task7"),
		Action:         ItemActionCreated,
		P:              json.RawMessage("{\"rr\":null,\"rp\":null,\"rt\":[],\"tt\":\"\xff\"}"),
		ServerIndex:    0,
		HasServerIndex: true,
	}
	if err := applyTask7PreflightItem(states, item); err != nil {
		t.Fatalf("applyTask7PreflightItem returned transport-level error: %v", err)
	}
	if !states[target].ambiguous {
		t.Fatal("invalid UTF-8 target payload was not marked ambiguous")
	}
}

func TestHistoryWriteTask7PreflightFailureDoesNotMutateHistory(t *testing.T) {
	target := NewUUID()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start-index") == "0" {
			fmt.Fprint(w, historyPage(3, taskHistoryBatch(target, ItemKindTask7, 0, `{"rr":null,"rp":null,"rt":[]}`)))
			return
		}
		fmt.Fprint(w, historyPage(3))
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	h.LatestServerIndex, h.LoadedServerIndex = 8, 4
	err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"x"`)})
	if err == nil {
		t.Fatal("Write(zero-progress history) succeeded")
	}
	if h.LatestServerIndex != 8 || h.LoadedServerIndex != 4 {
		t.Fatalf("history mutated on preflight failure: latest=%d loaded=%d", h.LatestServerIndex, h.LoadedServerIndex)
	}
}

func TestHistoryWriteLegacyTaskStillSkipsPreflight(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("legacy write sent %s, want POST", r.Method)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(taskWriteProbe{id: NewUUID(), kind: ItemKindTask}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d, want one POST", requests)
	}
}

func TestHistoryWriteMutableKindVariablesCannotEnableFutureTaskWrites(t *testing.T) {
	originalTask, originalTask7 := ItemKindTask, ItemKindTask7
	ItemKindTask, ItemKindTask7 = "Task8", "Task8"
	defer func() {
		ItemKindTask, ItemKindTask7 = originalTask, originalTask7
	}()

	futureTarget := NewUUID()
	requests, posts := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodGet {
			fmt.Fprint(w, historyPage(1, taskHistoryBatch(futureTarget, ItemKind("Task8"), 0, `{"rr":null,"rp":null,"rt":[]}`)))
			return
		}
		posts++
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()
	h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
	if err := h.Write(taskWriteProbe{id: NewUUID(), kind: ItemKindTask}); err == nil {
		t.Fatal("mutated ItemKindTask enabled serialized Task8")
	}
	if requests != 0 {
		t.Fatalf("future task write sent %d requests, want zero", requests)
	}
	if err := h.Write(rawWriteProbe{id: NewUUID(), body: task7CreateBody("literal Task7")}); err != nil {
		t.Fatalf("literal Task7 was not validated by its serialized kind: %v", err)
	}
	if requests != 1 {
		t.Fatalf("literal Task7 sent %d requests, want one POST", requests)
	}
	if err := h.Write(rawWriteProbe{id: futureTarget, body: task7UpdateBody(`"tt":"x"`)}); err == nil {
		t.Fatal("mutated ItemKindTask7 made raw Task8 target look supported during preflight")
	}
	if requests != 2 || posts != 1 {
		t.Fatalf("preflight mutation case requests=%d posts=%d, want two total requests and one POST", requests, posts)
	}
}
