package thingscloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistory_Write_RejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}

	bad := []string{
		"6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c", // standard hyphenated UUID
		"",                                     // empty
		"VJ0edXTP9q3PmFDUuy8EQh",               // '0' not in Base58 alphabet
	}
	for _, id := range bad {
		item := TaskActionItem{
			Item: Item{UUID: id, Kind: ItemKindTask, Action: ItemActionCreated},
			P:    TaskActionItemPayload{Title: String("x")},
		}
		if err := h.Write(item); err == nil {
			t.Errorf("Write with UUID %q: got nil error, want validation error", id)
		}
	}
	if requests != 0 {
		t.Errorf("server received %d requests, want 0 — invalid items must be rejected before POST", requests)
	}
}

func TestHistory_Write_AcceptsCanonicalUUID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}

	item := TaskActionItem{
		Item: Item{UUID: NewUUID(), Kind: ItemKindTask, Action: ItemActionCreated},
		P:    TaskActionItemPayload{Title: String("x")},
	}
	if err := h.Write(item); err != nil {
		t.Errorf("Write with canonical UUID: %v, want nil", err)
	}
}

func TestHistory_Write_RejectsDuplicateUUIDs(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}

	id := NewUUID()
	a := TaskActionItem{
		Item: Item{UUID: id, Kind: ItemKindTask, Action: ItemActionModified},
		P:    TaskActionItemPayload{Title: String("renamed")},
	}
	b := TaskActionItem{
		Item: Item{UUID: id, Kind: ItemKindTask, Action: ItemActionModified},
		P:    TaskActionItemPayload{Status: Status(TaskStatusCompleted)},
	}
	if err := h.Write(a, b); err == nil {
		t.Error("Write with two items sharing a UUID: got nil error, want duplicate error — the commit map silently drops one op")
	}
	if requests != 0 {
		t.Errorf("server received %d requests, want 0", requests)
	}
}
