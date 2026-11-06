package main

import (
	"fmt"
	"os"

	"github.com/uichat-mira/mira-mcp/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败："+err.Error())
		os.Exit(1)
	}
}
