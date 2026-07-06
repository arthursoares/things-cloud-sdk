package thingscloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func authCapturingServer(statusCode int, body string) (*httptest.Server, *http.Header) {
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprintln(w, body)
	}))
	return server, &captured
}

func TestClient_Histories(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "histories-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		hs, err := c.Histories()
		if err != nil {
			t.Fatalf("Expected history request to succeed, but didn't: %q", err.Error())
		}
		if len(hs) != 1 {
			t.Errorf("Expected to receive %d histories, but got %d", 1, len(hs))
		}
	})

	t.Run("SetsAuthorizationHeader", func(t *testing.T) {
		t.Parallel()
		server, captured := authCapturingServer(http.StatusOK, `[]`)
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if _, err := c.Histories(); err != nil {
			t.Fatalf("Histories failed: %v", err)
		}
		if got := (*captured).Get("Authorization"); got != "Password secret" {
			t.Errorf("Authorization = %q, want %q", got, "Password secret")
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{401, "error.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "unknown@example.com", "")
		_, err := c.Histories()
		if err == nil {
			t.Error("Expected history request to fail, but didn't")
		}
	})
}

func TestClient_CreateHistory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "create-history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h, err := c.CreateHistory()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
		if h.ID != "33333abb-bfe4-4b03-a5c9-106d42220c72" {
			t.Fatalf("Expected key %s but got %s", "33333abb-bfe4-4b03-a5c9-106d42220c72", h.ID)
		}
	})

	t.Run("SetsAuthorizationHeader", func(t *testing.T) {
		t.Parallel()
		server, captured := authCapturingServer(http.StatusOK, `{}`)
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if _, err := c.CreateHistory(); err != nil {
			t.Fatalf("CreateHistory failed: %v", err)
		}
		if got := (*captured).Get("Authorization"); got != "Password secret" {
			t.Errorf("Authorization = %q, want %q", got, "Password secret")
		}
	})
}

func TestHistory_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{202, "create-history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		err := h.Delete()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
	})

	t.Run("SetsAuthorizationHeader", func(t *testing.T) {
		t.Parallel()
		server, captured := authCapturingServer(http.StatusAccepted, `{}`)
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		if err := h.Delete(); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if got := (*captured).Get("Authorization"); got != "Password secret" {
			t.Errorf("Authorization = %q, want %q", got, "Password secret")
		}
	})
}

func TestHistory_Sync(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		err := h.Sync()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
		if h.LatestServerIndex != 27 {
			t.Errorf("Expected LatestServerIndex of %d, but got %d", 27, h.LatestServerIndex)
		}
	})
}
