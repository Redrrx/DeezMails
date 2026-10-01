package main

import (
	"log/slog"
	"os"

	"deezmails/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("DeezMails stopped", "error", err)
		os.Exit(1)
	}
}
