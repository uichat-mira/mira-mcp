package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Server 是本机的 MCP 端点，供同一账号下的移动端经云端通道调用。
type Server struct {
	secret   string
	username string
	mu       sync.RWMutex
	http     *http.Server
}

// NewServer 构造 MCP 端点。secret 是移动端必须携带的 Bearer 口令。
func NewServer(secret, username string) *Server {
	return &Server{secret: secret, username: username}
}

// SetUser 在同一个进程内切换账号时更新展示信息。
func (s *Server) SetUser(username string) {
	s.mu.Lock()
	s.username = username
	s.mu.Unlock()
}

// MCPEndpoint 返回移动端应访问的地址。
func MCPEndpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp", DefaultPort)
}

// Start 在本地回环地址上启动服务。
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/mcp", s.handleMCP)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", DefaultPort))
	if err != nil {
		return fmt.Errorf("本机端口 %d 被占用：%w", DefaultPort, err)
	}

	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("mcp server stopped: %v", err)
		}
	}()
	return nil
}

// Stop 关闭服务。
func (s *Server) Stop() {
	if s.http == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 只接受携带本机口令的调用；不接受匿名访问。
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != s.secret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "解析 JSON 失败"}})
		return
	}

	switch req.Method {
	case "initialize":
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mira-mcp", "version": Version},
		}})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.tools()}})
	case "tools/call":
		s.handleToolCall(w, req)
	default:
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "未知方法：" + req.Method}})
	}
}

func (s *Server) tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "mira_whoami",
			"description": "返回本机 Mira 账号与端点信息。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "mira_ping",
			"description": "确认本机 MCP 端点在线。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

func (s *Server) handleToolCall(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "参数不合法"}})
		return
	}

	var text string
	switch params.Name {
	case "mira_whoami":
		s.mu.RLock()
		text = fmt.Sprintf("账号：%s\n端点：%s\n版本：%s", s.username, MCPEndpoint(), Version)
		s.mu.RUnlock()
	case "mira_ping":
		text = "pong"
	default:
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "未知工具：" + params.Name}})
		return
	}

	writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	}})
}

func writeRPC(w http.ResponseWriter, resp rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
