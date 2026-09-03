package main

import (
	"log/slog"
	"os"

	"github.com/StellarisJAY/agent-classroom/internal/bootstrap"
	"github.com/StellarisJAY/agent-classroom/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		slog.Error("bootstrap app failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := app.Close(); err != nil {
			slog.Error("close app failed", "error", err)
		}
	}()

	if err := app.Run(); err != nil {
		slog.Error("server run failed", "error", err)
		os.Exit(1)
	}
}
