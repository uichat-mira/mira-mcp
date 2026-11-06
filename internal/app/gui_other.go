//go:build !windows

package app

import "errors"

// Run 在非 Windows 平台直接失败：登录窗依赖 Win32 原生控件。
func Run() error {
	return errors.New("mira-mcp 仅支持 Windows")
}
