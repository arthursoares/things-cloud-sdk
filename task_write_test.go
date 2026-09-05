package thingscloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type taskWriteProbe struct {
	id   string
	kind ItemKind
}

func (p taskWriteProbe) UUID() string { return p.id }

func (p taskWriteProbe) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"e": p.kind, "t": 0, "p": map[string]any{"tt": "private fixture title"}})
}

func TestHistoryWriteRejectsUnverifiedTaskKinds(t *testing.T) {
	for _, kind := range []ItemKind{"Task7", "Task8", "Task8\nprivate fixture title"} {
		t.Run(string(kind), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				fmt.Fprint(w, `{"server-head-index":1}`)
			}))
			defer server.Close()
			history := New(server.URL, "test@example.com", "test-password").HistoryWithID("test-history")
			err := history.Write(taskWriteProbe{id: NewUUID(), kind: kind})
			if err == nil {
				t.Error("unverified task kind was accepted for writing")
			} else if strings.Contains(err.Error(), "private fixture title") {
				t.Error("write diagnostic exposed fixture contents")
			}
			if requests.Load() != 0 {
				t.Errorf("sent %d requests, want zero", requests.Load())
			}
		})
	}
}
