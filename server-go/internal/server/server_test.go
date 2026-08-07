package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuziyue/douyin-downloader/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", body["status"])
	}
}

func TestLoginSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	body := `{"username":"xuziyue","password":"mmjsxu666555"}`
	req := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	body := `{"username":"wrong","password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestResolveRequiresAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	body := `{"url":"https://www.douyin.com/video/123"}`
	req := httptest.NewRequest("POST", "/api/v1/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}

func TestTokenIssueAndVerify(t *testing.T) {
	secret := "test_secret_123"
	token, err := issueToken("testuser", secret)
	if err != nil {
		t.Fatalf("issueToken failed: %v", err)
	}

	payload, ok := verifyToken(token, secret)
	if !ok {
		t.Fatal("verifyToken failed for valid token")
	}
	if payload["sub"] != "testuser" {
		t.Errorf("expected sub=testuser, got %v", payload["sub"])
	}

	// Invalid token
	_, ok = verifyToken("invalid.token.here", secret)
	if ok {
		t.Error("expected verifyToken to fail for invalid token")
	}

	// Wrong secret
	_, ok = verifyToken(token, "wrong_secret")
	if ok {
		t.Error("expected verifyToken to fail with wrong secret")
	}
}

func TestCORS(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	req := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected CORS header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestServerShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cl := &config.ConfigLoader{Config: cfg}
	deps := NewServerDeps(cl)
	srv := New(deps, "127.0.0.1", 0)

	ctx := context.Background()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}
