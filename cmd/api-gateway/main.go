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
	"agentmsg/internal/observability"
	"agentmsg/internal/repository"
	"agentmsg/internal/service"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(".")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		return 1
	}
	if err := cfg.Validate(true); err != nil {
		logger.Error("Invalid configuration", "error", err)
		return 1
	}

	db, err := repository.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return 1
	}
	defer db.Close()

	if cfg.AutoMigrate {
		if err := repository.RunMigrations(context.Background(), db); err != nil {
			logger.Error("Failed to run migrations", "error", err)
			return 1
		}
	}

	redisClient, err := repository.NewRedisClient(cfg.RedisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		return 1
	}
	defer redisClient.Close()

	traceShutdown, err := observability.InitTracing(context.Background(), observability.TraceConfig{
		ServiceName: "agentmsg-api-gateway",
		Environment: cfg.Env,
		Enabled:     cfg.OTELEnabled,
		Endpoint:    cfg.OTELExporterOTLPEndpoint,
		Insecure:    cfg.OTELInsecure,
	})
	if err != nil {
		logger.Error("Failed to initialize tracing", "error", err)
		return 1
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(ctx); err != nil {
			logger.Error("Failed to shutdown tracing", "error", err)
		}
	}()

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
		Middleware:     middleware.NewMiddleware(redisClient, db, authService, cfg.RateLimitRequests, cfg.RateLimitWindow),
	})

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              ":9090",
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 2)

	go func() {
		logger.Info("Starting API Gateway", "addr", cfg.APIGatewayPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("api gateway: %w", err)
		}
	}()

	go func() {
		logger.Info("Starting metrics server", "port", "9090")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var fatalErr error
	select {
	case sig := <-quit:
		logger.Info("Received shutdown signal", "signal", sig.String())
	case fatalErr = <-errCh:
		logger.Error("Runtime server error", "error", fatalErr)
	}

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error("Metrics server forced to shutdown", "error", err)
	}

	logger.Info("Server exited")
	if fatalErr != nil {
		return 1
	}
	return 0
}
