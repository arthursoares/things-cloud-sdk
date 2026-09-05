package thingscloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_RegisterAppInstance(t *testing.T) {
	t.Parallel()
	var capturedBody map[string]interface{}
	var capturedMethod string
	var capturedPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		bs, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bs, &capturedBody)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := New(ts.URL, "test@test.com", "password")
	err := c.RegisterAppInstance(AppInstanceRequest{
		AppInstanceID: "hash1-com.culturedcode.ThingsMac-hash2",
		HistoryKey:    "251943ab-63b5-45d1-8f9d-828a8d92fc15",
		APNSToken:     "token123",
		AppID:         "com.culturedcode.ThingsMac",
		Dev:           false,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedPath != "/version/1/app-instance/hash1-com.culturedcode.ThingsMac-hash2" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if capturedBody["history-key"] != "251943ab-63b5-45d1-8f9d-828a8d92fc15" {
		t.Error("expected history-key in body")
	}
}
