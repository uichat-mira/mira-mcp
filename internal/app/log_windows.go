//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 程序以 windowsgui 子系统运行，没有 stdout/stderr 可看，
// 诊断日志是排查「窗口未响应」这类问题的唯一证据来源。
var (
	logFile *os.File
	logMu   sync.Mutex
)

// resetLog 关闭上一轮日志句柄。
func resetLog() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

func ensureLog() {
	if logFile != nil {
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	full := filepath.Join(dir, "MiraMCP")
	if err := os.MkdirAll(full, 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(full, "mira-mcp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	logFile = f
}

// logf 写入诊断日志。只记录状态与错误，不记录密码或 token。
func logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	ensureLog()
	if logFile == nil {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05.000") + " " + fmt.Sprintf(format, args...) + "\n"
	_, _ = logFile.WriteString(line)
}

func logStartup() {
	logf("---- mira-mcp %s starting ----", Version)
}
