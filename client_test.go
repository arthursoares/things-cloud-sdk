package thingscloud

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type fakeResponse struct {
	statusCode int
	file       string
}

func fakeServer(t fakeResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(fmt.Sprintf("tapes/%s", t.file))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Printf("Unable to open fixture %q path %q", t.file, r.URL.Path)
			return
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Printf("Unable to load fixture %q path %q", t.file, r.URL.Path)
			return
		}
		w.WriteHeader(t.statusCode)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, string(content))
	}))
}

func TestClient_UserAgent(t *testing.T) {
	var capturedHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := New(ts.URL, "test@example.com", "password")

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.do(req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify User-Agent is the updated value
	got := capturedHeaders.Get("User-Agent")
	want := "ThingsMac/32209501"
	if got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}

	// Verify things-client-info header is set and non-empty
	clientInfo := capturedHeaders.Get("Things-Client-Info")
	if clientInfo == "" {
		t.Error("things-client-info header is missing or empty")
	}
}

func TestNew_SetsHTTPTimeout(t *testing.T) {
	t.Parallel()

	c := New("https://example.com", "user@example.com", "pw")
	if c.client.Timeout <= 0 {
		t.Error("New() http.Client has no Timeout — a stalled connection hangs Sync forever")
	}
}
