package thingscloud

import (
	"net/http"
	"testing"
)

func TestClient_RegisterAppInstance_ErrorStatus(t *testing.T) {
	t.Parallel()
	server, _ := captureServer(http.StatusInternalServerError, "")
	defer server.Close()

	c := New(server.URL, "test@test.com", "password")
	err := c.RegisterAppInstance(AppInstanceRequest{
		AppInstanceID: "hash1-com.culturedcode.ThingsMac-hash2",
		HistoryKey:    "251943ab-63b5-45d1-8f9d-828a8d92fc15",
	})
	if err == nil {
		t.Error("expected RegisterAppInstance to fail on non-200, got nil")
	}
}

func TestClient_RegisterAppInstance_TransportError(t *testing.T) {
	t.Parallel()
	// Closed port -> the underlying request fails inside do().
	c := New("http://127.0.0.1:1", "test@test.com", "password")
	err := c.RegisterAppInstance(AppInstanceRequest{AppInstanceID: "x"})
	if err == nil {
		t.Error("expected RegisterAppInstance to surface a transport error, got nil")
	}
}
