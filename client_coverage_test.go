package thingscloud

import (
	"net/http"
	"testing"
)

func TestHTTPError_Error(t *testing.T) {
	t.Parallel()
	err := &HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}
	got := err.Error()
	want := "http response code: 500 Internal Server Error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestClient_do_ConnectionError(t *testing.T) {
	t.Parallel()
	// Point at a closed port so the underlying RoundTrip fails; do() should
	// surface the transport error rather than a response.
	c := New("http://127.0.0.1:1", "martin@example.com", "")
	req, err := http.NewRequest("GET", "/version/1/account/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.do(req); err == nil {
		t.Error("expected do() to return a transport error, got nil")
	}
}

func TestClient_do_InvalidEndpoint(t *testing.T) {
	t.Parallel()
	// A control character in the endpoint makes url.Parse fail inside do().
	c := New("http://\x7f invalid", "martin@example.com", "")
	req, err := http.NewRequest("GET", "/path", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.do(req); err == nil {
		t.Error("expected do() to fail parsing a malformed endpoint, got nil")
	}
}

func TestClient_do_SetsContentTypeForBodyMethods(t *testing.T) {
	t.Parallel()
	var captured http.Header
	server, _ := captureServer(http.StatusOK, "")
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")

	// GET must NOT carry a Content-Type/Encoding.
	getReq, _ := http.NewRequest("GET", "/x", nil)
	resp, err := c.do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if ct := getReq.Header.Get("Content-Type"); ct != "" {
		t.Errorf("GET Content-Type = %q, want empty", ct)
	}

	// PUT must carry Content-Type/Encoding.
	putReq, _ := http.NewRequest("PUT", "/x", nil)
	resp, err = c.do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	captured = putReq.Header
	if ct := captured.Get("Content-Type"); ct == "" {
		t.Error("PUT Content-Type is empty, want it set")
	}
	if ce := captured.Get("Content-Encoding"); ce != "UTF-8" {
		t.Errorf("PUT Content-Encoding = %q, want UTF-8", ce)
	}
}

func TestClient_do_DebugLogging(t *testing.T) {
	t.Parallel()
	// Exercise the Debug branch that dumps request/response; we only assert it
	// does not panic and still performs the request.
	server, _ := captureServer(http.StatusOK, `{}`)
	defer server.Close()

	c := New(server.URL, "martin@example.com", "")
	c.Debug = true
	req, _ := http.NewRequest("GET", "/x", nil)
	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() with Debug failed: %v", err)
	}
	resp.Body.Close()
}
