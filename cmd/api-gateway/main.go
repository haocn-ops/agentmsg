package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"agentmsg/internal/api"
	"agentmsg/internal/config"
	"agentmsg/internal/middleware"
	"agentmsg/internal/repository"
	"agentmsg/internal/service"
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

	agentRepo := repository.NewAgentRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	ackRepo := repository.NewAcknowledgementRepository(db)

	agentService := service.NewAgentService(agentRepo, redisClient)
	messageService := service.NewMessageService(messageRepo, ackRepo, redisClient)
	authService := service.NewAuthService(cfg.JWTSecret)

	server := api.NewServer(&api.ServerConfig{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.APIGatewayPort),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}, &api.Dependencies{
		AgentService:   agentService,
		MessageService: messageService,
		AuthService:    authService,
		Database:       db,
		Redis:          redisClient,
		Middleware:     middleware.NewMiddleware(redisClient, authService, cfg.RateLimitRequests, cfg.RateLimitWindow),
	})

	go func() {
		logger.Info("Starting API Gateway", "addr", cfg.APIGatewayPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logger.Info("Starting metrics server", "port", "9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			logger.Error("Metrics server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited")
}
