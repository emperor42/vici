package vici

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnonymousSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SecretKey = []byte("test-secret") // Enable auth logic but no token provided

	handler := Middleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, auth, _ := GetUserContext(r)
		if auth {
			t.Error("Expected anonymous user")
		}
		if id == "" {
			t.Error("Expected non-empty user ID")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check if cookie was set
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Error("Expected session cookie to be set")
	}
	if cookies[0].Name != cfg.CookieName {
		t.Errorf("Expected cookie name %s, got %s", cfg.CookieName, cookies[0].Name)
	}
}

func TestAuthenticatedUser(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SecretKey = []byte("test-secret")

	// Simulate a valid token (prefix "valid_" + random string)
	validToken := "valid_" + generateShortID()

	handler := Middleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, auth, meta := GetUserContext(r)
		if !auth {
			t.Error("Expected authenticated user")
		}
		if meta["auth_method"] != "header" {
			t.Errorf("Expected auth_method 'header', got '%s'", meta["auth_method"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", validToken)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestContextPropagation(t *testing.T) {
	cfg := DefaultConfig()

	handler := Middleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context is accessible
		id, auth, meta := GetUserContext(r)
		if id == "" {
			t.Error("User ID not propagated")
		}
		w.Write([]byte(id))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
