package main

import (
	"os"

	"github.com/uichat-mira/mira-mcp/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		app.ReportFatal(err)
		os.Exit(1)
	}
}
