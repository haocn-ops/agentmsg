package main

import (
	"context"
	"log/slog"
	"os"

	"agentmsg/internal/config"
	"agentmsg/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(".")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := repository.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := repository.RunMigrations(context.Background(), db); err != nil {
		logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	logger.Info("Migrations applied successfully")
}
