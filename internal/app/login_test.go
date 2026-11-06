package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginRejectsEmptyCredentials(t *testing.T) {
	if _, err := Login(context.Background(), "http://127.0.0.1:1", " ", "x"); err == nil {
		t.Fatal("空账号应当报错")
	}
	if _, err := Login(context.Background(), "http://127.0.0.1:1", "u", ""); err == nil {
		t.Fatal("空密码应当报错")
	}
}

func TestLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != LoginPath {
			t.Errorf("路径应为 %s，实际 %s", LoginPath, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求体不是合法 JSON: %v", err)
		}
		if body["username"] != "tomz" || body["password"] != "secret" {
			t.Errorf("请求体字段不符合契约: %v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sessionToken": "mcs_v1.abc",
			"account":      map[string]string{"id": "acc_1", "username": "tomz"},
			"expiresAt":    "2026-12-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	result, err := Login(context.Background(), srv.URL, " tomz ", "secret")
	if err != nil {
		t.Fatalf("登录不应失败: %v", err)
	}
	if result.Token != "mcs_v1.abc" || result.Username != "tomz" {
		t.Fatalf("返回内容不正确: %+v", result)
	}
}

func TestLoginSurfacesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "AUTH_INVALID_CREDENTIALS",
			"message": "账号或密码错误",
		})
	}))
	defer srv.Close()

	_, err := Login(context.Background(), srv.URL, "tomz", "bad")
	if err == nil {
		t.Fatal("401 应当返回错误")
	}
	if !strings.Contains(err.Error(), "账号或密码错误") {
		t.Fatalf("错误信息应包含服务端原文，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("错误信息应包含状态码，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("错误信息应包含错误码，实际: %v", err)
	}
}

func TestLoginMissingTokenFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": map[string]string{"username": "tomz"},
		})
	}))
	defer srv.Close()

	if _, err := Login(context.Background(), srv.URL, "tomz", "secret"); err == nil {
		t.Fatal("缺少 sessionToken 应当报错，而不是静默接受")
	}
}

func TestMCPRejectsUnauthorized(t *testing.T) {
	s := NewServer("right-secret", "tomz")
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	rec := httptest.NewRecorder()
	s.handleMCP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("缺少口令应返回 401，实际 %d", rec.Code)
	}
}

func TestMCPToolsListAuthorized(t *testing.T) {
	s := NewServer("secret-ok", "tomz")
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer secret-ok")
	rec := httptest.NewRecorder()
	s.handleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("合法口令应返回 200，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "mira_ping") {
		t.Fatalf("响应应包含 mira_ping，实际 %s", rec.Body.String())
	}
}
