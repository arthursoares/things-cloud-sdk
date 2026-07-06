package thingscloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistory_Sync_ErrorsOnMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"current-item-index": not-json`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
	if err := h.Sync(); err == nil {
		t.Error("Sync with malformed JSON response: got nil error — a silently zero LatestServerIndex feeds a wrong ancestor-index into the next Write")
	}
}

func TestHistory_Write_ErrorsOnMalformedCommitResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json at all`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
	item := TaskActionItem{
		Item: Item{UUID: "VJ1edXTP9q3PmFDUuy8EQh", Kind: ItemKindTask, Action: ItemActionCreated},
		P:    TaskActionItemPayload{Title: String("x")},
	}
	if err := h.Write(item); err == nil {
		t.Error("Write with malformed commit response: got nil error — LatestServerIndex silently stays stale")
	}
}

func TestCreateHistory_ErrorsOnMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `garbage`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	if _, err := c.CreateHistory(); err == nil {
		t.Error("CreateHistory with malformed response: got nil error — returns a History with empty ID")
	}
}
