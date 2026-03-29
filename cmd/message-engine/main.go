package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agentmsg/internal/config"
	"agentmsg/internal/engine"
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

	redisClient, err := repository.NewRedisClient(cfg.RedisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	msgEngine := engine.NewMessageEngine(&engine.EngineConfig{
		WorkerCount:    16,
		BatchSize:      100,
		FlushInterval:  100,
		MaxRetries:     3,
	}, db, redisClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := msgEngine.Start(ctx); err != nil {
			logger.Error("Engine error", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("Message Engine started", "port", cfg.MessageEnginePort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down engine...")
	cancel()

	logger.Info("Engine exited")
}

func init() {
	fmt.Println(`
╔══════════════════════════════════════════════╗
║      AI Agent Messaging - Message Engine      ║
║                  v1.0.0                      ║
╚══════════════════════════════════════════════╝
	`)
}
