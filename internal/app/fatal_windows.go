//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

var procMessageBox = user32.NewProc("MessageBoxW")

// ReportFatal 用系统消息框显示启动错误。
// 程序以 windowsgui 子系统构建，没有控制台可写 stderr，
// 所以错误必须通过对话框呈现，否则用户只会看到「程序没反应」。
func ReportFatal(err error) {
	if err == nil {
		return
	}
	text := utf16ptr("启动失败：\r\n\r\n" + err.Error())
	caption := utf16ptr("Mira 移动端连接")
	procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x00000010)
}
