package thingscloud

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureServer records the method, path, Authorization header and decoded JSON
// body of the single request it receives, and replies with statusCode/respBody.
func captureServer(statusCode int, respBody string) (*httptest.Server, *capturedRequest) {
	cap := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.Method = r.Method
		cap.Path = r.URL.Path
		cap.Authorization = r.Header.Get("Authorization")
		bs, _ := io.ReadAll(r.Body)
		cap.RawBody = string(bs)
		_ = json.Unmarshal(bs, &cap.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if respBody != "" {
			_, _ = w.Write([]byte(respBody))
		}
	}))
	return server, cap
}

type capturedRequest struct {
	Method        string
	Path          string
	Authorization string
	RawBody       string
	Body          map[string]interface{}
}

func TestAccountService_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server, cap := captureServer(http.StatusAccepted, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if err := c.Accounts.Delete(); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if cap.Method != "DELETE" {
			t.Errorf("Method = %q, want DELETE", cap.Method)
		}
		if cap.Path != "/version/1/account/martin@example.com" {
			t.Errorf("Path = %q", cap.Path)
		}
		if cap.Authorization != "Password secret" {
			t.Errorf("Authorization = %q, want %q", cap.Authorization, "Password secret")
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusUnauthorized, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "wrong")
		if err := c.Accounts.Delete(); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("Delete err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("OtherStatus", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusInternalServerError, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		err := c.Accounts.Delete()
		if err == nil || errors.Is(err, ErrUnauthorized) {
			t.Errorf("Delete err = %v, want generic http error", err)
		}
	})
}

func TestAccountService_AcceptSLA(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server, cap := captureServer(http.StatusOK, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if err := c.Accounts.AcceptSLA(); err != nil {
			t.Fatalf("AcceptSLA failed: %v", err)
		}
		if cap.Method != "PUT" {
			t.Errorf("Method = %q, want PUT", cap.Method)
		}
		if _, ok := cap.Body["SLA-version-accepted"]; !ok {
			t.Errorf("body missing SLA-version-accepted: %s", cap.RawBody)
		}
		if cap.Authorization != "Password secret" {
			t.Errorf("Authorization = %q", cap.Authorization)
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusUnauthorized, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "wrong")
		if err := c.Accounts.AcceptSLA(); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("AcceptSLA err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("OtherStatus", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusBadGateway, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if err := c.Accounts.AcceptSLA(); err == nil || errors.Is(err, ErrUnauthorized) {
			t.Errorf("AcceptSLA err = %v, want generic http error", err)
		}
	})
}

func TestAccountService_Confirm(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server, cap := captureServer(http.StatusOK, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if err := c.Accounts.Confirm("code-123"); err != nil {
			t.Fatalf("Confirm failed: %v", err)
		}
		if cap.Body["confirmation-code"] != "code-123" {
			t.Errorf("confirmation-code = %v, want code-123", cap.Body["confirmation-code"])
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusUnauthorized, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "wrong")
		if err := c.Accounts.Confirm("code"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("Confirm err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("OtherStatus", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusForbidden, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "secret")
		if err := c.Accounts.Confirm("code"); err == nil || errors.Is(err, ErrUnauthorized) {
			t.Errorf("Confirm err = %v, want generic http error", err)
		}
	})
}

func TestAccountService_SignUp(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server, cap := captureServer(http.StatusCreated, "")
		defer server.Close()

		c := New(server.URL, "", "")
		newClient, err := c.Accounts.SignUp("new@example.com", "pw123")
		if err != nil {
			t.Fatalf("SignUp failed: %v", err)
		}
		if newClient == nil {
			t.Fatal("SignUp returned nil client")
		}
		if newClient.EMail != "new@example.com" {
			t.Errorf("client EMail = %q, want new@example.com", newClient.EMail)
		}
		if cap.Path != "/version/1/account/new@example.com" {
			t.Errorf("Path = %q", cap.Path)
		}
		if cap.Body["password"] != "pw123" {
			t.Errorf("password = %v, want pw123", cap.Body["password"])
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusConflict, "")
		defer server.Close()

		c := New(server.URL, "", "")
		if _, err := c.Accounts.SignUp("taken@example.com", "pw"); err == nil {
			t.Error("expected SignUp to fail on non-201, but it succeeded")
		}
	})
}

func TestAccountService_ChangePassword(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server, cap := captureServer(http.StatusOK, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "old")
		newClient, err := c.Accounts.ChangePassword("newpw")
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}
		if newClient == nil {
			t.Fatal("ChangePassword returned nil client")
		}
		if cap.Body["password"] != "newpw" {
			t.Errorf("password = %v, want newpw", cap.Body["password"])
		}
		// Changing a password must be authenticated with the OLD password,
		// like every other account mutation (Delete/AcceptSLA/Confirm).
		if cap.Authorization != "Password old" {
			t.Errorf("Authorization = %q, want %q", cap.Authorization, "Password old")
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusUnauthorized, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "old")
		if _, err := c.Accounts.ChangePassword("newpw"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("ChangePassword err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("OtherStatus", func(t *testing.T) {
		t.Parallel()
		server, _ := captureServer(http.StatusInternalServerError, "")
		defer server.Close()

		c := New(server.URL, "martin@example.com", "old")
		if _, err := c.Accounts.ChangePassword("newpw"); err == nil || errors.Is(err, ErrUnauthorized) {
			t.Errorf("ChangePassword err = %v, want generic http error", err)
		}
	})
}
