package thingscloud

import (
	"errors"
	"net/http"
	"testing"
)

func TestClient_History(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		// History() reads the "latest-server-index" field (historyResponse),
		// distinct from Sync()'s "current-item-index" (itemsResponse).
		server, _ := captureServer(http.StatusOK, `{"latest-server-index":42,"latest-schema-version":300,"latest-total-content-size":80112}`)
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		h, err := c.History("33333abb-bfe4-4b03-a5c9-106d42220c72")
		if err != nil {
			t.Fatalf("History failed: %v", err)
		}
		if h.ID != "33333abb-bfe4-4b03-a5c9-106d42220c72" {
			t.Errorf("ID = %q", h.ID)
		}
		if h.LatestServerIndex != 42 {
			t.Errorf("LatestServerIndex = %d, want 42", h.LatestServerIndex)
		}
		if h.LatestSchemaVersion != 300 {
			t.Errorf("LatestSchemaVersion = %d, want 300", h.LatestSchemaVersion)
		}
		if h.Client != c {
			t.Error("History.Client not wired to originating client")
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusUnauthorized, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		if _, err := c.History("id"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("History err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("OtherStatusReturnsHTTPError", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusInternalServerError, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		_, err := c.History("id")
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("History err = %v, want *HTTPError", err)
		}
		if httpErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("StatusCode = %d, want 500", httpErr.StatusCode)
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusOK, "not json")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		if _, err := c.History("id"); err == nil {
			t.Error("expected decode error on malformed JSON, got nil")
		}
	})
}

func TestClient_OwnHistory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "verify-success.json"})
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		h, err := c.OwnHistory()
		if err != nil {
			t.Fatalf("OwnHistory failed: %v", err)
		}
		if h.Client != c {
			t.Error("OwnHistory.Client not wired to originating client")
		}
	})

	t.Run("VerifyError", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{401, "error.json"})
		defer server.Close()

		c := New(server.URL, "martin@example.com", "")
		if _, err := c.OwnHistory(); err == nil {
			t.Error("expected OwnHistory to fail when Verify fails, got nil")
		}
	})
}

func TestClient_HistoryWithID(t *testing.T) {
	t.Parallel()
	c := New("http://example.com", "martin@example.com", "")
	h := c.HistoryWithID("known-id")
	if h.ID != "known-id" {
		t.Errorf("ID = %q, want known-id", h.ID)
	}
	if h.Client != c {
		t.Error("Client not wired")
	}
	if h.LatestServerIndex != 0 {
		t.Errorf("LatestServerIndex = %d, want 0 (no network call made)", h.LatestServerIndex)
	}
}

func TestHistory_Delete_ErrorStatus(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusInternalServerError, "")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := History{Client: c, ID: "id"}
	if err := h.Delete(); err == nil {
		t.Error("expected Delete to fail on non-202, got nil")
	}
}

func TestHistory_Sync_ErrorStatus(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusInternalServerError, "")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := History{Client: c, ID: "id"}
	err := h.Sync()
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Sync err = %v, want *HTTPError", err)
	}
}

func TestHistory_Sync_MalformedJSON(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusOK, "not json")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	h := History{Client: c, ID: "id"}
	if err := h.Sync(); err == nil {
		t.Error("expected Sync to fail on malformed JSON, got nil")
	}
}

func TestClient_CreateHistory_Unauthorized(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusUnauthorized, "")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	if _, err := c.CreateHistory(); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("CreateHistory err = %v, want ErrUnauthorized", err)
	}
}

func TestClient_Histories_Unauthorized(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusUnauthorized, "")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	if _, err := c.Histories(); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("Histories err = %v, want ErrUnauthorized", err)
	}
}

func TestHistory_Write_DuplicateUUID(t *testing.T) {
	t.Parallel()
	// A valid Base58 UUID reused twice in one commit must be rejected before
	// any network call, since the map keyed by UUID would silently drop one.
	id := NewUUID()
	c := New("http://example.com", "martin@example.com", "")
	h := History{Client: c, ID: "hist"}
	item := TombstoneActionItem{Item: Item{UUID: id}}
	if err := h.Write(item, item); err == nil {
		t.Error("expected Write to reject duplicate UUID in one commit, got nil")
	}
}

func TestHistory_Write_InvalidUUID(t *testing.T) {
	t.Parallel()
	c := New("http://example.com", "martin@example.com", "")
	h := History{Client: c, ID: "hist"}
	// Contains '0', which is not in the Base58 (Bitcoin) alphabet.
	item := TombstoneActionItem{Item: Item{UUID: "0000-not-base58"}}
	if err := h.Write(item); err == nil {
		t.Error("expected Write to reject non-Base58 UUID, got nil")
	}
}
