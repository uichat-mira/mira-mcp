package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// CloudBaseURL 是 Mira Cloud Auth 的公开端点。
	// 依据：cloud-auth 仓库 docs/deployment.md 的拓扑说明与线上 /health 实测。
	CloudBaseURL = "https://cloud-auth.tomzlab.cn"

	// LoginPath 对应 cloud-auth OPS-006 的根登录契约。
	LoginPath = "/v1/auth/login"

	Version     = "0.2.0"
	DefaultPort = 7788
)

// LoginResult 是登录成功后需要持久化的最小信息。
type LoginResult struct {
	Token    string
	Username string
}

// LoginError 让界面可以区分输入问题、网络不可达和服务端拒绝，而不是统一报「失败」。
type LoginError struct {
	Status  int
	Message string
}

func (e *LoginError) Error() string { return e.Message }

// loginPayload 对应 POST /v1/auth/login 的成功响应。
// 约定见 cloud-auth docs/auth-v0.md：sessionToken + account{id,username} + expiresAt。
type loginPayload struct {
	SessionToken string `json:"sessionToken"`
	Account      *struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"account"`
	ExpiresAt string `json:"expiresAt"`
}

// Login 使用云端既有契约换取 Cloud Session。
func Login(ctx context.Context, baseURL, username, password string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, &LoginError{Message: "请填写账号和密码"}
	}

	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return nil, &LoginError{Message: "无法构造登录请求：" + err.Error()}
	}

	endpoint := strings.TrimRight(baseURL, "/") + LoginPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &LoginError{Message: "无法构造登录请求：" + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &LoginError{Message: "无法连接云端 " + endpoint + "：" + err.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, &LoginError{Status: resp.StatusCode, Message: "读取登录响应失败：" + err.Error()}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &LoginError{
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("登录失败（HTTP %d）：%s", resp.StatusCode, serverMessage(raw)),
		}
	}

	var payload loginPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &LoginError{Status: resp.StatusCode, Message: "登录响应不是合法 JSON：" + err.Error()}
	}
	if payload.SessionToken == "" {
		return nil, &LoginError{Status: resp.StatusCode, Message: "登录响应缺少 sessionToken 字段"}
	}

	name := username
	if payload.Account != nil && payload.Account.Username != "" {
		name = payload.Account.Username
	}

	return &LoginResult{Token: payload.SessionToken, Username: name}, nil
}

// serverMessage 把服务端错误码/信息原样交给用户，找不到就截断原始响应。
func serverMessage(raw []byte) string {
	var generic struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &generic); err == nil {
		switch {
		case generic.Code != "" && generic.Message != "":
			return generic.Code + "：" + generic.Message
		case generic.Message != "":
			return generic.Message
		case generic.Code != "":
			return generic.Code
		case generic.Error != "":
			return generic.Error
		}
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		// cloud-auth 对无效凭据返回空的 401 正文，这里按契约还原为用户可读的提示。
		return "账号或密码错误（或该账号未被允许使用）"
	}
	return truncate(trimmed, 200)
}

func truncate(s string, limit int) string {
	// 按字符而非字节截断，避免把中文切成非法 UTF-8。
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
