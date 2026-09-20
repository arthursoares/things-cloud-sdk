package thingscloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoryWriteTask7PreflightScopesTombstoneValidation(t *testing.T) {
	for _, kind := range []ItemKind{"Tombstone2", "Tombstone"} {
		t.Run(string(kind), func(t *testing.T) {
			target, unrelated := NewUUID(), NewUUID()
			deletedID := target
			if kind == "Tombstone" {
				deletedID = "legacy-target"
				target = EncodeLegacyIdentifier(deletedID)
				unrelated = "legacy-unrelated"
			}
			for _, tc := range []struct {
				name    string
				payload string
				wantOK  bool
			}{
				{"unrelated valid deletion", `{"dloid":"` + unrelated + `","dld":1}`, true},
				{"unrelated invalid timestamp", `{"dloid":"` + unrelated + `","dld":"invalid"}`, true},
				{"target invalid timestamp", `{"dloid":"` + deletedID + `","dld":"invalid"}`, false},
				{"target valid deletion", `{"dloid":"` + deletedID + `","dld":1}`, false},
				{"missing target", `{"dld":1}`, false},
				{"empty target", `{"dloid":"","dld":1}`, false},
				{"null target", `{"dloid":null,"dld":1}`, false},
				{"nonstring target", `{"dloid":42,"dld":1}`, false},
				{"duplicate target last", `{"dloid":"` + unrelated + `","dloid":"` + deletedID + `","dld":1}`, false},
				{"duplicate target first", `{"dloid":"` + deletedID + `","dloid":"` + unrelated + `","dld":1}`, false},
				{"escaped duplicate target", `{"dloid":"` + deletedID + `","\u0064loid":"` + unrelated + `","dld":1}`, false},
				{"case alias target last", `{"dloid":"` + unrelated + `","DLOID":"` + deletedID + `","dld":1}`, false},
				{"case alias target first", `{"DLOID":"` + deletedID + `","dloid":"` + unrelated + `","dld":1}`, false},
				{"unpaired surrogate", `{"dloid":"\ud800","dld":1}`, false},
				{"invalid UTF8", "{\"dloid\":\"\xff\",\"dld\":1}", false},
				{"nonobject payload", `[]`, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					posts := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method == http.MethodPost {
							posts++
							if got := r.URL.Query().Get("ancestor-index"); got != "2" {
								t.Errorf("ancestor-index = %q, want 2", got)
							}
							fmt.Fprint(w, `{"server-head-index":3}`)
							return
						}
						fmt.Fprint(w, historyPage(2,
							taskHistoryBatch(target, "Task7", 0, `{"rr":null,"rp":null,"rt":[]}`),
							taskHistoryBatch(NewUUID(), kind, 0, tc.payload)))
					}))
					defer server.Close()
					h := New(server.URL, "test@example.com", "password").HistoryWithID("history")
					h.LatestServerIndex, h.LoadedServerIndex = 8, 4
					err := h.Write(rawWriteProbe{id: target, body: task7UpdateBody(`"tt":"renamed"`)})
					if tc.wantOK {
						if err != nil || posts != 1 {
							t.Fatalf("Write: error=%v posts=%d, want one successful commit", err, posts)
						}
						return
					}
					if err == nil || posts != 0 {
						t.Fatalf("Write: error=%v posts=%d, want rejection before commit", err, posts)
					}
					if h.LatestServerIndex != 8 || h.LoadedServerIndex != 4 {
						t.Fatal("failed preflight mutated history indexes")
					}
					if tc.name != "target valid deletion" && !strings.Contains(err.Error(), "malformed tombstone in raw history at index 1:") {
						t.Fatalf("missing tombstone diagnostic: %v", err)
					}
				})
			}
		})
	}
}
