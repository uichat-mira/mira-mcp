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
	// CloudBaseURL 是 Mira Cloud 的公开端点。
	// 取值依据：移动端 relay 规范中的默认 relay 端点，以及 uichat-mira-relay README
	// 声明的 hosted endpoint。若云端改址，这里必须同步更新。
	CloudBaseURL = "https://relay.tomz.io"

	// LoginPath 与移动端 desktopMiraHost 使用的登录契约一致：POST {username,password}。
	LoginPath = "/login"

	Version     = "0.1.0"
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

type loginPayload struct {
	Token    string     `json:"token"`
	Username string     `json:"username"`
	Data     *loginData `json:"data"`
}

type loginData struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// Login 使用云端既有契约换取访问令牌。
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

	client := &http.Client{Timeout: 15 * time.Second}
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

	token, name := payload.Token, payload.Username
	if token == "" && payload.Data != nil {
		token, name = payload.Data.Token, payload.Data.Username
	}
	if token == "" {
		return nil, &LoginError{Status: resp.StatusCode, Message: "登录响应缺少 token 字段"}
	}
	if name == "" {
		name = username
	}

	return &LoginResult{Token: token, Username: name}, nil
}

// serverMessage 尽量把服务端的错误信息原样交给用户，找不到就截断原始响应。
func serverMessage(raw []byte) string {
	var generic struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &generic); err == nil {
		if generic.Message != "" {
			return generic.Message
		}
		if generic.Error != "" {
			return generic.Error
		}
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "服务端未返回内容，请确认云端地址是否正确"
	}
	return truncate(trimmed, 200)
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
