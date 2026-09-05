package thingscloud

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestInvalidRequestIdentifiersReturnErrors(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"Verify", "Items", "Write"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("%s panicked instead of returning a request error: %v", operation, p)
				}
			}()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("invalid identifier reached the HTTP server")
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			c := New(server.URL, "bad\nemail", "password")
			h := c.HistoryWithID("bad\nhistory")
			h.LatestServerIndex = 42
			h.LoadedServerIndex = 21

			var err error
			switch operation {
			case "Verify":
				var response *VerifyResponse
				response, err = c.Verify()
				if response != nil {
					t.Error("Verify returned a response for an invalid request")
				}
			case "Items":
				var items []Item
				var more bool
				items, more, err = h.Items(ItemsOptions{StartIndex: 21})
				if items != nil || more {
					t.Errorf("Items returned (%v, %v) for an invalid request", items, more)
				}
			case "Write":
				err = h.Write(TaskActionItem{
					Item: Item{UUID: NewUUID(), Kind: ItemKindTask, Action: ItemActionCreated},
					P:    TaskActionItemPayload{Title: String("test")},
				})
			}
			var urlErr *url.Error
			if !errors.As(err, &urlErr) {
				t.Fatalf("%s error = %v, want request URL error", operation, err)
			}
			if h.LatestServerIndex != 42 || h.LoadedServerIndex != 21 {
				t.Errorf("invalid request changed history cursors: %+v", h)
			}
		})
	}
}

func TestHistoryWritePreservesRequestMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/version/1/history/history-id/commit" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		for key, want := range map[string]string{
			"Schema": "301", "Push-Priority": "5", "App-Id": "com.culturedcode.ThingsMac",
			"Accept": "application/json", "Content-Encoding": "UTF-8",
			"Content-Type": "application/json; charset=UTF-8", "User-Agent": ThingsUserAgent,
		} {
			if got := r.Header.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		if r.Header.Get("App-Instance-Id") == "" || r.Header.Get("Things-Client-Info") == "" {
			t.Error("missing app instance or client info header")
		}
		if got := r.URL.Query().Get("ancestor-index"); got != "42" {
			t.Errorf("ancestor-index = %q, want 42", got)
		}
		if got := r.URL.Query().Get("_cnt"); got != "1" {
			t.Errorf("_cnt = %q, want 1", got)
		}
		_, _ = w.Write([]byte(`{"server-head-index":43}`))
	}))
	defer server.Close()
	c := New(server.URL, "test@example.com", "password")
	h := c.HistoryWithID("history-id")
	h.LatestServerIndex = 42
	if err := h.Write(TaskActionItem{
		Item: Item{UUID: NewUUID(), Kind: ItemKindTask, Action: ItemActionCreated},
		P:    TaskActionItemPayload{Title: String("test")},
	}); err != nil {
		t.Fatal(err)
	}
	if h.LatestServerIndex != 43 {
		t.Errorf("LatestServerIndex = %d, want 43", h.LatestServerIndex)
	}
}
