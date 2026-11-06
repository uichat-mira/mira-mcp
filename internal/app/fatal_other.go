//go:build !windows

package app

import (
	"fmt"
	"os"
)

// ReportFatal 在非 Windows 平台把错误写到标准错误。
func ReportFatal(err error) {
	fmt.Fprintln(os.Stderr, "启动失败："+err.Error())
}
