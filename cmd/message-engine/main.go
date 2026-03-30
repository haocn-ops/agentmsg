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

	"agentmsg/internal/config"
	"agentmsg/internal/engine"
	"agentmsg/internal/observability"
	"agentmsg/internal/repository"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	if err := cfg.Validate(false); err != nil {
		logger.Error("Invalid configuration", "error", err)
		return 1
	}

	db, err := repository.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return 1
	}
	defer db.Close()

	redisClient, err := repository.NewRedisClient(cfg.RedisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		return 1
	}
	defer redisClient.Close()

	traceShutdown, err := observability.InitTracing(context.Background(), observability.TraceConfig{
		ServiceName: "agentmsg-message-engine",
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

	msgEngine := engine.NewMessageEngine(&engine.EngineConfig{
		WorkerCount:   16,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
		MaxRetries:    3,
	}, db, redisClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := msgEngine.Start(ctx); err != nil {
		logger.Error("Engine error", "error", err)
		return 1
	}

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	healthMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, checkCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer checkCancel()

		if err := db.Ping(checkCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, `{"status":"not_ready","database":"error"}`)
			return
		}
		if err := redisClient.Ping(checkCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, `{"status":"not_ready","redis":"error"}`)
			return
		}

		writeJSON(w, http.StatusOK, `{"status":"ready"}`)
	})

	healthServer := &http.Server{
		Addr:              ":" + cfg.MessageEnginePort,
		Handler:           healthMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              ":9091",
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 2)

	go func() {
		logger.Info("Starting message engine health server", "addr", cfg.MessageEnginePort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("health server: %w", err)
		}
	}()

	go func() {
		logger.Info("Starting message engine metrics server", "port", "9091")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	logger.Info("Message Engine started", "port", cfg.MessageEnginePort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var fatalErr error
	select {
	case sig := <-quit:
		logger.Info("Received shutdown signal", "signal", sig.String())
	case fatalErr = <-errCh:
		logger.Error("Runtime server error", "error", fatalErr)
	}

	logger.Info("Shutting down engine...")
	cancel()
	msgEngine.Stop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = healthServer.Shutdown(shutdownCtx)
	_ = metricsServer.Shutdown(shutdownCtx)

	logger.Info("Engine exited")
	if fatalErr != nil {
		return 1
	}
	return 0
}

func init() {
	fmt.Println(`
╔══════════════════════════════════════════════╗
║      AI Agent Messaging - Message Engine      ║
║                  v1.0.0                      ║
╚══════════════════════════════════════════════╝
	`)
}

func writeJSON(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(body))
}
